package subscriptions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMiddlewareRequiresSubscription(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := quotaRequest(user.ID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "subscription_required") {
		t.Fatalf("expected subscription_required 403, got %d %s", rec.Code, rec.Body.String())
	}

	quota := int64(100)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, quotaRequest(user.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected allowed request, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestMiddlewareBlocksExceededQuota(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(10)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}
	if err := store.IncrementUsage(context.Background(), user.ID, quota, time.Now().UTC()); err != nil {
		t.Fatalf("increment usage: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, quotaRequest(user.ID))

	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "quota_exceeded") {
		t.Fatalf("expected quota_exceeded 429, got %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestQuotaRecorderIncrementsRedisUsage(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(100)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	recorder := NewQuotaRecorder(noopRecorder{}, store, nil)
	if err := recorder.RecordInterception(context.Background(), &aibrecorder.InterceptionRecord{
		ID:          "interception-1",
		InitiatorID: user.ID,
	}); err != nil {
		t.Fatalf("record interception: %v", err)
	}
	if err := recorder.RecordTokenUsage(context.Background(), &aibrecorder.TokenUsageRecord{
		InterceptionID:        "interception-1",
		Input:                 3,
		Output:                4,
		CacheReadInputTokens:  1,
		CacheWriteInputTokens: 2,
	}); err != nil {
		t.Fatalf("record token usage: %v", err)
	}

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("current quota: %v", err)
	}
	if status.Used5HTokens != 10 || status.Used7DTokens != 10 {
		t.Fatalf("unexpected usage: %#v", status)
	}
}

func TestWatchCatchesUpMissedUnlimitedToLimitedAssignment(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()

	unlimited, err := service.CreatePlan(ctx, PlanInput{Name: "Unlimited"})
	if err != nil {
		t.Fatalf("create unlimited plan: %v", err)
	}
	quota := int64(100)
	limited, err := service.CreatePlan(ctx, PlanInput{
		Name:          "Limited",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
	})
	if err != nil {
		t.Fatalf("create limited plan: %v", err)
	}

	unlimitedID := unlimited.ID
	if _, err := service.AssignUserPlan(ctx, user.ID, &unlimitedID); err != nil {
		t.Fatalf("assign unlimited plan: %v", err)
	}
	initial, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load initial quota: %v", err)
	}
	if initial.Plan == nil || initial.Plan.ID != unlimited.ID || initial.Quota5HTokens != nil {
		t.Fatalf("expected cached unlimited plan, got %#v", initial)
	}

	// Publish the assignment change before Watch subscribes. Watch must catch
	// up from Redis' version counter rather than depending solely on Pub/Sub.
	service.SetNotifier(store.store)
	limitedID := limited.ID
	if _, err := service.AssignUserPlan(ctx, user.ID, &limitedID); err != nil {
		t.Fatalf("assign limited plan: %v", err)
	}

	watchCtx, cancelWatch := context.WithCancel(ctx)
	t.Cleanup(cancelWatch)
	go store.Watch(watchCtx)

	deadline := time.Now().Add(2 * time.Second)
	for {
		current, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC())
		if err != nil {
			t.Fatalf("load refreshed quota: %v", err)
		}
		if current.Plan != nil && current.Plan.ID == limited.ID &&
			current.Quota5HTokens != nil && *current.Quota5HTokens == quota &&
			current.Quota7DTokens != nil && *current.Quota7DTokens == quota {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("limited plan was not observed after watcher catch-up: %#v", current)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRefreshIfStaleUpdatesUnlimitedToLimitedAssignmentImmediately(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()

	unlimited, err := service.CreatePlan(ctx, PlanInput{Name: "Unlimited"})
	if err != nil {
		t.Fatalf("create unlimited plan: %v", err)
	}
	quota := int64(100)
	limited, err := service.CreatePlan(ctx, PlanInput{
		Name:          "Limited",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
	})
	if err != nil {
		t.Fatalf("create limited plan: %v", err)
	}

	unlimitedID := unlimited.ID
	if _, err := service.AssignUserPlan(ctx, user.ID, &unlimitedID); err != nil {
		t.Fatalf("assign unlimited plan: %v", err)
	}
	initial, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load initial quota: %v", err)
	}
	if initial.Plan == nil || initial.Plan.ID != unlimited.ID || initial.Quota5HTokens != nil {
		t.Fatalf("expected cached unlimited plan, got %#v", initial)
	}

	service.SetNotifier(store.store)
	limitedID := limited.ID
	if _, err := service.AssignUserPlan(ctx, user.ID, &limitedID); err != nil {
		t.Fatalf("assign limited plan: %v", err)
	}
	if err := store.RefreshIfStale(ctx); err != nil {
		t.Fatalf("refresh stale subscription state: %v", err)
	}

	current, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load refreshed quota: %v", err)
	}
	if current.Plan == nil || current.Plan.ID != limited.ID ||
		current.Quota5HTokens == nil || *current.Quota5HTokens != quota ||
		current.Quota7DTokens == nil || *current.Quota7DTokens != quota {
		t.Fatalf("expected limited plan after one refresh, got %#v", current)
	}
}

func TestRefreshIfStaleAcceptsLowerRedisVersion(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()

	unlimited, err := service.CreatePlan(ctx, PlanInput{Name: "Unlimited"})
	if err != nil {
		t.Fatalf("create unlimited plan: %v", err)
	}
	quota := int64(100)
	limited, err := service.CreatePlan(ctx, PlanInput{
		Name:          "Limited",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
	})
	if err != nil {
		t.Fatalf("create limited plan: %v", err)
	}

	unlimitedID := unlimited.ID
	if _, err := service.AssignUserPlan(ctx, user.ID, &unlimitedID); err != nil {
		t.Fatalf("assign unlimited plan: %v", err)
	}
	if err := store.store.Client().Set(
		ctx,
		redisstore.VersionKey(configevents.DomainSubscriptions),
		2,
		0,
	).Err(); err != nil {
		t.Fatalf("seed redis subscription version: %v", err)
	}
	store.SyncVersion(ctx)
	initial, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("cache version 2 assignment: %v", err)
	}
	if initial.Plan == nil || initial.Plan.ID != unlimited.ID {
		t.Fatalf("expected cached unlimited plan, got %#v", initial)
	}

	if err := store.store.Client().Set(
		ctx,
		redisstore.VersionKey(configevents.DomainSubscriptions),
		8,
		0,
	).Err(); err != nil {
		t.Fatalf("advance redis subscription version: %v", err)
	}
	store.SyncVersion(ctx)
	limitedID := limited.ID
	if _, err := service.AssignUserPlan(ctx, user.ID, &limitedID); err != nil {
		t.Fatalf("assign limited plan: %v", err)
	}

	// Simulate Redis losing its version counter and reusing version 2, whose
	// assignment cache still contains Unlimited.
	if err := store.store.Client().Set(
		ctx,
		redisstore.VersionKey(configevents.DomainSubscriptions),
		2,
		0,
	).Err(); err != nil {
		t.Fatalf("reset redis subscription version: %v", err)
	}
	if err := store.RefreshIfStale(ctx); err != nil {
		t.Fatalf("refresh lower subscription version: %v", err)
	}
	current, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota after version reset: %v", err)
	}
	if got := store.version.Load(); got != 2 || current.Plan == nil || current.Plan.ID != limited.ID {
		t.Fatalf("expected limited plan at reset version 2, version=%d quota=%#v", got, current)
	}
}

func TestListPlansPagedIncludesAssignmentCounts(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	ctx := context.Background()

	pro, err := service.CreatePlan(ctx, PlanInput{Name: "Pro"})
	if err != nil {
		t.Fatalf("create pro plan: %v", err)
	}
	free, err := service.CreatePlan(ctx, PlanInput{Name: "Free"})
	if err != nil {
		t.Fatalf("create free plan: %v", err)
	}
	free, err = service.UpdatePlan(ctx, free.ID, PlanInput{Name: "Free", IsDefault: true})
	if err != nil {
		t.Fatalf("set free as default plan: %v", err)
	}

	proID := pro.ID
	if _, err := service.AssignUserPlan(ctx, user.ID, &proID); err != nil {
		t.Fatalf("assign user plan: %v", err)
	}

	serviceAccount := users.User{
		ID:                "22222222-2222-2222-2222-222222222222",
		ExternalSub:       "service-worker",
		Email:             "worker@example.com",
		PreferredUsername: "worker",
		Name:              "Worker",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := service.db.Create(&serviceAccount).Error; err != nil {
		t.Fatalf("create service account: %v", err)
	}
	if _, err := service.AssignServiceAccountPlan(ctx, serviceAccount.ID, &proID); err != nil {
		t.Fatalf("assign service account plan: %v", err)
	}

	freeUser := users.User{
		ID:                "33333333-3333-3333-3333-333333333333",
		ExternalSub:       "free-user-sub",
		Email:             "free@example.com",
		PreferredUsername: "free",
		Name:              "Free User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := service.db.Create(&freeUser).Error; err != nil {
		t.Fatalf("create free user: %v", err)
	}
	freeID := free.ID
	if _, err := service.AssignUserPlan(ctx, freeUser.ID, &freeID); err != nil {
		t.Fatalf("assign free user plan: %v", err)
	}

	inheritedServiceAccount := users.User{
		ID:                "44444444-4444-4444-4444-444444444444",
		ExternalSub:       "inherited-worker",
		Email:             "inherited@example.com",
		PreferredUsername: "inherited",
		Name:              "Inherited Worker",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := service.db.Create(&inheritedServiceAccount).Error; err != nil {
		t.Fatalf("create inherited service account: %v", err)
	}

	result, err := service.ListPlansPaged(ctx, PlanListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "name",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}

	plansByName := map[string]PlanResponse{}
	for _, item := range result.Items {
		plansByName[item.Name] = item
	}
	if plansByName["Pro"].AssignedUsersCount != 1 ||
		plansByName["Pro"].AssignedServiceAccountsCount != 1 ||
		plansByName["Pro"].AssignedDirectAccountsCount != 2 ||
		plansByName["Pro"].AssignedIndirectAccountsCount != 0 ||
		plansByName["Pro"].AssignedAccountsCount != 2 {
		t.Fatalf("unexpected pro assignment counts: %#v", plansByName["Pro"])
	}
	if plansByName["Free"].AssignedUsersCount != 1 ||
		plansByName["Free"].AssignedServiceAccountsCount != 0 ||
		plansByName["Free"].AssignedDirectAccountsCount != 1 ||
		plansByName["Free"].AssignedIndirectAccountsCount != 1 ||
		plansByName["Free"].AssignedAccountsCount != 2 {
		t.Fatalf("unexpected free assignment counts: %#v", plansByName["Free"])
	}

	reloaded, err := service.GetPlan(ctx, pro.ID)
	if err != nil {
		t.Fatalf("get pro plan: %v", err)
	}
	if reloaded.AssignedAccountsCount != 2 {
		t.Fatalf("expected get plan to include assignment count, got %#v", reloaded)
	}

	reloadedDefault, err := service.GetPlan(ctx, free.ID)
	if err != nil {
		t.Fatalf("get default plan: %v", err)
	}
	if reloadedDefault.AssignedDirectAccountsCount != 1 ||
		reloadedDefault.AssignedIndirectAccountsCount != 1 ||
		reloadedDefault.AssignedAccountsCount != 2 {
		t.Fatalf("expected get default plan to include inherited assignment count, got %#v", reloadedDefault)
	}
}

func TestAssignmentsReturnCompleteAccountProjections(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	ctx := context.Background()

	plan, err := service.CreatePlan(ctx, PlanInput{Name: "Complete"})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)
	if err := service.db.Model(&users.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"note":       "keep user metadata",
		"expires_at": expiresAt,
	}).Error; err != nil {
		t.Fatalf("update user projection fields: %v", err)
	}
	seedSubscriptionTokenConsumption(t, service.db, user.ID, "user-complete", 12, 7)

	planID := plan.ID
	assignedUser, err := service.AssignUserPlan(ctx, user.ID, &planID)
	if err != nil {
		t.Fatalf("assign user plan: %v", err)
	}
	if assignedUser.Note != "keep user metadata" ||
		assignedUser.ExpiresAt == nil || !assignedUser.ExpiresAt.Equal(expiresAt) ||
		assignedUser.InputTokens != 12 || assignedUser.OutputTokens != 7 {
		t.Fatalf("incomplete assigned user projection: %#v", assignedUser)
	}

	account := users.User{
		ID:                "55555555-5555-5555-5555-555555555555",
		ExternalSub:       "service-complete",
		PreferredUsername: "complete-worker",
		Name:              "Complete Worker",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		Note:              "keep service metadata",
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := service.db.Create(&account).Error; err != nil {
		t.Fatalf("create service account: %v", err)
	}
	seedSubscriptionTokenConsumption(t, service.db, account.ID, "service-complete", 21, 4)

	assignedAccount, err := service.AssignServiceAccountPlan(ctx, account.ID, &planID)
	if err != nil {
		t.Fatalf("assign service account plan: %v", err)
	}
	if assignedAccount.Note != "keep service metadata" ||
		assignedAccount.InputTokens != 21 || assignedAccount.OutputTokens != 4 {
		t.Fatalf("incomplete assigned service-account projection: %#v", assignedAccount)
	}
}

func newSubscriptionTestStore(t *testing.T) (*Service, *RedisStore, users.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}
	createSubscriptionUsageTables(t, db)
	user := users.User{
		ID:                "11111111-1111-1111-1111-111111111111",
		ExternalSub:       "sub",
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	redisServer := miniredis.RunT(t)
	redisStore, err := redisstore.NewRequired(context.Background(), "redis://"+redisServer.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	t.Cleanup(func() { _ = redisStore.Close() })
	store := NewRedisStore(redisStore, service, time.Minute, nil)
	store.SyncVersion(context.Background())
	return service, store, user
}

func createSubscriptionUsageTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, statement := range []string{
		`CREATE TABLE interceptions (id text PRIMARY KEY, initiator_id text NOT NULL)`,
		`CREATE TABLE token_usages (
			id text PRIMARY KEY,
			interception_id text NOT NULL,
			input_tokens integer NOT NULL DEFAULT 0,
			output_tokens integer NOT NULL DEFAULT 0
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create subscription usage table: %v", err)
		}
	}
}

func seedSubscriptionTokenConsumption(t *testing.T, db *gorm.DB, userID, interceptionID string, input, output int64) {
	t.Helper()

	if err := db.Exec("INSERT INTO interceptions (id, initiator_id) VALUES (?, ?)", interceptionID, userID).Error; err != nil {
		t.Fatalf("seed subscription interception: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO token_usages (id, interception_id, input_tokens, output_tokens) VALUES (?, ?, ?, ?)",
		"usage-"+interceptionID,
		interceptionID,
		input,
		output,
	).Error; err != nil {
		t.Fatalf("seed subscription token usage: %v", err)
	}
}

func quotaRequest(userID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt"}`))
	profile := auth.UserProfile{
		ID:       userID,
		Type:     auth.UserTypeUser,
		Role:     auth.RoleUser,
		IsActive: true,
	}
	return req.WithContext(auth.ContextWithUser(req.Context(), profile))
}

type noopRecorder struct{}

func (noopRecorder) RecordInterception(context.Context, *aibrecorder.InterceptionRecord) error {
	return nil
}

func (noopRecorder) RecordInterceptionEnded(context.Context, *aibrecorder.InterceptionRecordEnded) error {
	return nil
}

func (noopRecorder) RecordTokenUsage(context.Context, *aibrecorder.TokenUsageRecord) error {
	return nil
}

func (noopRecorder) RecordPromptUsage(context.Context, *aibrecorder.PromptUsageRecord) error {
	return nil
}

func (noopRecorder) RecordToolUsage(context.Context, *aibrecorder.ToolUsageRecord) error {
	return nil
}

func (noopRecorder) RecordModelThought(context.Context, *aibrecorder.ModelThoughtRecord) error {
	return nil
}
