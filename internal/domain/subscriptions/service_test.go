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
