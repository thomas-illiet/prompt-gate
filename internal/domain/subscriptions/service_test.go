package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
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

func TestCurrentQuotaSnapshotIsAtomicWithConcurrentUsage(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(1_000_000)
	if _, err := service.CreatePlan(ctx, PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}
	if _, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC()); err != nil {
		t.Fatalf("prime quota assignment: %v", err)
	}

	const (
		writers             = 4
		incrementsPerWriter = 150
		readers             = 4
		readsPerReader      = 200
	)
	start := make(chan struct{})
	errorsCh := make(chan error, 1)
	reportError := func(err error) {
		select {
		case errorsCh <- err:
		default:
		}
	}
	var wait sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for index := 0; index < incrementsPerWriter; index++ {
				if err := store.IncrementUsage(ctx, user.ID, 1, time.Now().UTC()); err != nil {
					reportError(fmt.Errorf("increment usage: %w", err))
					return
				}
			}
		}()
	}
	for reader := 0; reader < readers; reader++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			for index := 0; index < readsPerReader; index++ {
				status, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC())
				if err != nil {
					reportError(fmt.Errorf("read quota: %w", err))
					return
				}
				if status.Used5HTokens != status.Used7DTokens {
					reportError(fmt.Errorf(
						"non-atomic quota snapshot: used5h=%d used7d=%d",
						status.Used5HTokens,
						status.Used7DTokens,
					))
					return
				}
			}
		}()
	}
	close(start)
	wait.Wait()
	select {
	case err := <-errorsCh:
		t.Fatal(err)
	default:
	}

	status, observedAt, err := store.currentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load final quota snapshot: %v", err)
	}
	wantUsed := int64(writers * incrementsPerWriter)
	if status.Used5HTokens != wantUsed || status.Used7DTokens != wantUsed {
		t.Fatalf("unexpected final atomic snapshot: %#v", status)
	}
	nextStatus, nextObservedAt, err := store.currentQuota(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load subsequent quota snapshot: %v", err)
	}
	if nextStatus.Used5HTokens != status.Used5HTokens || nextStatus.Used7DTokens != status.Used7DTokens {
		t.Fatalf("quota changed without a concurrent writer: first=%#v next=%#v", status, nextStatus)
	}
	if !nextObservedAt.After(observedAt) {
		t.Fatalf("quota observation clock did not advance: first=%s next=%s", observedAt, nextObservedAt)
	}
	clockMicros, err := store.store.Client().Get(ctx, redisstore.SubscriptionQuotaObservationClockKey()).Int64()
	if err != nil {
		t.Fatalf("load quota observation clock: %v", err)
	}
	if clockMicros != nextObservedAt.UnixMicro() {
		t.Fatalf("quota observation clock = %d, want %d", clockMicros, nextObservedAt.UnixMicro())
	}
}

func TestCurrentQuotaObservationClockOrdersFrozenAndRegressedRedisTime(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	redisTime := time.Date(2026, time.January, 12, 10, 5, 0, 0, time.UTC)
	store, redisServer := newSubscriptionRedisStoreAt(t, service, redisTime)

	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}
	if err := store.IncrementUsage(ctx, user.ID, 50, redisTime); err != nil {
		t.Fatalf("seed quota usage: %v", err)
	}

	beforeReset, observedBeforeReset, err := store.currentQuota(ctx, user.ID, redisTime)
	if err != nil {
		t.Fatalf("load pre-reset quota: %v", err)
	}
	if beforeReset.Used5HTokens != 50 || beforeReset.Used7DTokens != 50 {
		t.Fatalf("unexpected pre-reset quota: %#v", beforeReset)
	}
	if err := service.UpsertQuotaState(ctx, user.ID, beforeReset, observedBeforeReset); err != nil {
		t.Fatalf("persist pre-reset quota: %v", err)
	}

	if _, err := store.ApplyQuotaReset(ctx, user.ID, 1); err != nil {
		t.Fatalf("apply quota reset: %v", err)
	}
	afterReset, observedAfterReset, err := store.currentQuota(ctx, user.ID, redisTime)
	if err != nil {
		t.Fatalf("load post-reset quota: %v", err)
	}
	if afterReset.Used5HTokens != 0 || afterReset.Used7DTokens != 0 {
		t.Fatalf("unexpected post-reset quota: %#v", afterReset)
	}
	if !observedAfterReset.After(observedBeforeReset) {
		t.Fatalf(
			"frozen Redis TIME did not advance the logical observation clock: before=%v after=%v",
			observedBeforeReset,
			observedAfterReset,
		)
	}
	if err := service.UpsertQuotaState(ctx, user.ID, afterReset, observedAfterReset); err != nil {
		t.Fatalf("persist post-reset quota: %v", err)
	}

	redisServer.SetTime(redisTime.Add(-2 * time.Hour))
	regressedQuota, observedAfterRegression, err := store.currentQuota(ctx, user.ID, redisTime.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("load quota after Redis clock regression: %v", err)
	}
	if !observedAfterRegression.After(observedAfterReset) {
		t.Fatalf(
			"regressed Redis TIME moved the logical observation clock backwards: previous=%v regressed=%v",
			observedAfterReset,
			observedAfterRegression,
		)
	}
	if err := service.UpsertQuotaState(ctx, user.ID, regressedQuota, observedAfterRegression); err != nil {
		t.Fatalf("persist quota after Redis clock regression: %v", err)
	}
	if err := service.UpsertQuotaState(ctx, user.ID, beforeReset, observedBeforeReset); err != nil {
		t.Fatalf("retry stale pre-reset projection: %v", err)
	}

	var persisted SubscriptionQuotaState
	if err := service.db.WithContext(ctx).Where("user_id = ?", user.ID).Take(&persisted).Error; err != nil {
		t.Fatalf("load persisted quota projection: %v", err)
	}
	if persisted.Used5HTokens != 0 || persisted.Used7DTokens != 0 {
		t.Fatalf("stale projection replaced the reset state: %#v", persisted)
	}
	if !persisted.SyncedAt.Equal(observedAfterRegression) {
		t.Fatalf("unexpected durable observation time: got %v want %v", persisted.SyncedAt, observedAfterRegression)
	}
	clock, err := store.store.Client().Get(ctx, redisstore.SubscriptionQuotaObservationClockKey()).Int64()
	if err != nil {
		t.Fatalf("load logical observation clock: %v", err)
	}
	if clock != observedAfterRegression.UnixMicro() {
		t.Fatalf("unexpected logical observation clock: got %d want %d", clock, observedAfterRegression.UnixMicro())
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
		ExternalSub:       testPointer("service-worker"),
		Email:             "worker@example.com",
		PreferredUsername: "worker",
		Name:              "Worker",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
	}
	if err := service.db.Create(&serviceAccount).Error; err != nil {
		t.Fatalf("create service account: %v", err)
	}
	if _, err := service.AssignServiceAccountPlan(ctx, serviceAccount.ID, &proID); err != nil {
		t.Fatalf("assign service account plan: %v", err)
	}

	freeUser := users.User{
		ID:                "33333333-3333-3333-3333-333333333333",
		ExternalSub:       testPointer("free-user-sub"),
		Email:             "free@example.com",
		PreferredUsername: "free",
		Name:              "Free User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
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
		ExternalSub:       testPointer("inherited-worker"),
		Email:             "inherited@example.com",
		PreferredUsername: "inherited",
		Name:              "Inherited Worker",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
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
		ExternalSub:       testPointer("service-complete"),
		PreferredUsername: "complete-worker",
		Name:              "Complete Worker",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		Note:              "keep service metadata",
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
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

func TestSyncQuotaStatesPrunesDeletedUsersIncludingOldAndBatchedEntries(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{Name: "Default", Quota5HTokens: &quota, IsDefault: true}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}
	if _, err := store.CurrentQuota(ctx, user.ID, time.Now().UTC()); err != nil {
		t.Fatalf("cache user assignment: %v", err)
	}
	if err := store.IncrementUsage(ctx, user.ID, 5, time.Now().UTC()); err != nil {
		t.Fatalf("seed user usage: %v", err)
	}
	reset, err := store.ApplyQuotaReset(ctx, user.ID, 1)
	if err != nil || !reset.Advanced {
		t.Fatalf("seed user reset state: %#v, %v", reset, err)
	}
	oldScore := time.Now().UTC().Add(-30 * 24 * time.Hour).Unix()
	if err := store.store.Client().ZAdd(ctx, redisstore.SubscriptionActiveUsersZSetKey(), redis.Z{
		Score: float64(oldScore), Member: user.ID,
	}).Err(); err != nil {
		t.Fatalf("age tracked user: %v", err)
	}
	if err := store.store.Client().SAdd(ctx, redisstore.SubscriptionActiveUsersKey(), user.ID).Err(); err != nil {
		t.Fatalf("seed legacy tracked user: %v", err)
	}
	retainedUser := users.User{
		ID:                "22222222-2222-2222-2222-222222222222",
		ExternalSub:       testPointer("retained-old-user"),
		PreferredUsername: "retained-old-user",
		Name:              "Retained Old User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          false,
	}
	if err := service.db.Create(&retainedUser).Error; err != nil {
		t.Fatalf("create retained inactive user: %v", err)
	}
	if err := store.store.Client().ZAdd(ctx, redisstore.SubscriptionActiveUsersZSetKey(), redis.Z{
		Score: float64(oldScore), Member: retainedUser.ID,
	}).Err(); err != nil {
		t.Fatalf("seed retained inactive user: %v", err)
	}
	if err := store.store.Client().HSet(ctx, redisstore.SubscriptionQuotaResetStateKey(retainedUser.ID), "sequence", 1, "cutoff", 1).Err(); err != nil {
		t.Fatalf("seed retained inactive reset: %v", err)
	}

	if err := users.NewService(service.db).DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("delete user while Redis state remains: %v", err)
	}
	for index := 0; index < 125; index++ {
		orphanID := fmt.Sprintf("00000000-0000-0000-0000-%012d", index+1)
		if err := store.store.Client().ZAdd(ctx, redisstore.SubscriptionActiveUsersZSetKey(), redis.Z{
			Score: float64(oldScore), Member: orphanID,
		}).Err(); err != nil {
			t.Fatalf("seed orphan %s: %v", orphanID, err)
		}
		if err := store.store.Client().HSet(ctx, redisstore.SubscriptionQuotaResetStateKey(orphanID), "sequence", 1, "cutoff", 1).Err(); err != nil {
			t.Fatalf("seed orphan reset %s: %v", orphanID, err)
		}
	}
	malformedID := "malformed-orphan-id"
	if err := store.store.Client().ZAdd(ctx, redisstore.SubscriptionActiveUsersZSetKey(), redis.Z{
		Score: float64(oldScore), Member: malformedID,
	}).Err(); err != nil {
		t.Fatalf("seed malformed orphan: %v", err)
	}
	if err := store.store.Client().HSet(ctx, redisstore.SubscriptionQuotaResetStateKey(malformedID), "sequence", 1, "cutoff", 1).Err(); err != nil {
		t.Fatalf("seed malformed orphan reset: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.SyncQuotaStates(cancelledCtx, service); err == nil {
		t.Fatal("expected interrupted cleanup to be retried later")
	}
	if exists, err := store.store.Client().Exists(ctx, redisstore.SubscriptionQuotaResetStateKey(user.ID)).Result(); err != nil || exists != 1 {
		t.Fatalf("interrupted cleanup unexpectedly removed durable reset state: exists=%d err=%v", exists, err)
	}

	if _, err := store.SyncQuotaStates(ctx, service); err != nil {
		t.Fatalf("prune first deleted-user quota batch: %v", err)
	}
	remainingAfterFirstBatch, err := store.store.Client().ZCard(ctx, redisstore.SubscriptionActiveUsersZSetKey()).Result()
	if err != nil {
		t.Fatalf("count tracked users after first cleanup batch: %v", err)
	}
	if remainingAfterFirstBatch <= 1 {
		t.Fatalf("expected cursorized cleanup to leave later batches, remaining=%d", remainingAfterFirstBatch)
	}

	const maxCleanupRuns = 20
	for run := 1; run < maxCleanupRuns; run++ {
		if _, err := store.SyncQuotaStates(ctx, service); err != nil {
			t.Fatalf("prune deleted-user quota batch %d: %v", run+1, err)
		}
		remaining, err := store.store.Client().ZCard(ctx, redisstore.SubscriptionActiveUsersZSetKey()).Result()
		if err != nil {
			t.Fatalf("count tracked users after cleanup batch %d: %v", run+1, err)
		}
		legacyRemaining, err := store.store.Client().SCard(ctx, redisstore.SubscriptionActiveUsersKey()).Result()
		if err != nil {
			t.Fatalf("count legacy tracked users after cleanup batch %d: %v", run+1, err)
		}
		if remaining == 1 && legacyRemaining == 0 {
			break
		}
	}
	if remaining, err := store.store.Client().ZCard(ctx, redisstore.SubscriptionActiveUsersZSetKey()).Result(); err != nil || remaining != 1 {
		t.Fatalf("cursorized cleanup did not converge: remaining=%d err=%v", remaining, err)
	}
	keys := []string{
		redisstore.SubscriptionUsageZSetKey(user.ID, Window5H),
		redisstore.SubscriptionUsageHashKey(user.ID, Window5H),
		redisstore.SubscriptionUsageZSetKey(user.ID, Window7D),
		redisstore.SubscriptionUsageHashKey(user.ID, Window7D),
		redisstore.SubscriptionQuotaResetStateKey(user.ID),
		redisstore.SubscriptionAssignmentKey(store.version.Load(), user.ID),
	}
	if remaining, err := store.store.Client().Exists(ctx, keys...).Result(); err != nil || remaining != 0 {
		t.Fatalf("deleted user left Redis quota keys: remaining=%d err=%v", remaining, err)
	}
	if score, err := store.store.Client().ZScore(ctx, redisstore.SubscriptionActiveUsersZSetKey(), user.ID).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("deleted user remains in active zset: score=%v err=%v", score, err)
	}
	if member, err := store.store.Client().SIsMember(ctx, redisstore.SubscriptionActiveUsersKey(), user.ID).Result(); err != nil || member {
		t.Fatalf("deleted user remains in legacy active set: member=%v err=%v", member, err)
	}
	for _, orphanID := range []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000063",
		"00000000-0000-0000-0000-000000000125",
		malformedID,
	} {
		if exists, err := store.store.Client().Exists(ctx, redisstore.SubscriptionQuotaResetStateKey(orphanID)).Result(); err != nil || exists != 0 {
			t.Fatalf("batched orphan %s was not pruned: exists=%d err=%v", orphanID, exists, err)
		}
	}
	if exists, err := store.store.Client().Exists(ctx, redisstore.SubscriptionQuotaResetStateKey(retainedUser.ID)).Result(); err != nil || exists != 1 {
		t.Fatalf("existing inactive user was pruned: exists=%d err=%v", exists, err)
	}
}

func TestSyncQuotaStatesFiltersRecentOrphanOutsideCleanupBatch(t *testing.T) {
	service, store, _ := newSubscriptionTestStore(t)
	ctx := context.Background()
	orphanID := "33333333-3333-3333-3333-333333333333"
	if err := store.IncrementUsage(ctx, orphanID, 7, time.Now().UTC()); err != nil {
		t.Fatalf("seed recent orphan usage: %v", err)
	}

	// Force the bounded backstop to inspect the legacy set on this run. The
	// orphan must still be filtered from ActiveUserIDs before the FK-backed
	// quota projection is written.
	store.deletedUserScanSet = true
	if _, err := store.SyncQuotaStates(ctx, service); err != nil {
		t.Fatalf("sync recent orphan quota state: %v", err)
	}

	keys := []string{
		redisstore.SubscriptionUsageZSetKey(orphanID, Window5H),
		redisstore.SubscriptionUsageHashKey(orphanID, Window5H),
		redisstore.SubscriptionUsageZSetKey(orphanID, Window7D),
		redisstore.SubscriptionUsageHashKey(orphanID, Window7D),
	}
	if remaining, err := store.store.Client().Exists(ctx, keys...).Result(); err != nil || remaining != 0 {
		t.Fatalf("recent orphan left Redis quota keys: remaining=%d err=%v", remaining, err)
	}
}

func TestActiveUserIDsUsesRedisClockAndUnionsLegacySet(t *testing.T) {
	ctx := context.Background()
	redisServer := miniredis.RunT(t)
	redisServer.SetTime(time.Now().UTC().Add(30 * 24 * time.Hour))
	redisBackend, err := redisstore.NewRequired(ctx, "redis://"+redisServer.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	t.Cleanup(func() { _ = redisBackend.Close() })
	store := NewRedisStore(redisBackend, nil, time.Minute, nil)
	redisNow, err := redisBackend.Client().Time(ctx).Result()
	if err != nil {
		t.Fatalf("load Redis time: %v", err)
	}

	const (
		recentID      = "11111111-1111-1111-1111-111111111111"
		legacyOnlyID  = "22222222-2222-2222-2222-222222222222"
		localClockID  = "33333333-3333-3333-3333-333333333333"
		staleLegacyID = "44444444-4444-4444-4444-444444444444"
	)
	if err := redisBackend.Client().ZAdd(ctx, redisstore.SubscriptionActiveUsersZSetKey(),
		redis.Z{Score: float64(redisNow.Add(-time.Hour).Unix()), Member: recentID},
		redis.Z{Score: float64(time.Now().UTC().Add(-time.Hour).Unix()), Member: localClockID},
		redis.Z{Score: float64(redisNow.Add(-8 * 24 * time.Hour).Unix()), Member: staleLegacyID},
	).Err(); err != nil {
		t.Fatalf("seed active users: %v", err)
	}
	if err := redisBackend.Client().SAdd(
		ctx,
		redisstore.SubscriptionActiveUsersKey(),
		recentID,
		legacyOnlyID,
		staleLegacyID,
	).Err(); err != nil {
		t.Fatalf("seed legacy active users: %v", err)
	}

	ids, err := store.ActiveUserIDs(ctx)
	if err != nil {
		t.Fatalf("load active users: %v", err)
	}
	got := make(map[string]int, len(ids))
	for _, id := range ids {
		got[id]++
	}
	if got[recentID] != 1 || got[legacyOnlyID] != 1 || got[staleLegacyID] != 1 {
		t.Fatalf("expected current and legacy active users exactly once, got %#v", ids)
	}
	if got[localClockID] != 0 {
		t.Fatalf("local clock leaked into Redis-relative activity window: %#v", ids)
	}
	if remaining, err := redisBackend.Client().SCard(ctx, redisstore.SubscriptionActiveUsersKey()).Result(); err != nil || remaining != 0 {
		t.Fatalf("legacy active-user set was not drained: remaining=%d err=%v", remaining, err)
	}
	if _, err := redisBackend.Client().ZScore(ctx, redisstore.SubscriptionActiveUsersZSetKey(), legacyOnlyID).Result(); err != nil {
		t.Fatalf("legacy-only user was not migrated to active zset: %v", err)
	}
	staleScore, err := redisBackend.Client().ZScore(ctx, redisstore.SubscriptionActiveUsersZSetKey(), staleLegacyID).Result()
	if err != nil || int64(staleScore) < redisNow.Unix() {
		t.Fatalf("stale legacy zset score was not refreshed: score=%v err=%v", staleScore, err)
	}
	secondIDs, err := store.ActiveUserIDs(ctx)
	if err != nil {
		t.Fatalf("load active users after migration: %v", err)
	}
	if !slices.Contains(secondIDs, staleLegacyID) {
		t.Fatalf("refreshed legacy user disappeared after migration: %#v", secondIDs)
	}
}

func TestSyncQuotaStatesRetainsProjectionWhileSevenDayBucketIsLive(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	recordedAt := time.Date(2026, time.January, 5, 10, 5, 0, 0, time.UTC)
	store, redisServer := newSubscriptionRedisStoreAt(t, service, recordedAt)

	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}
	if err := store.IncrementUsage(ctx, user.ID, 25, recordedAt); err != nil {
		t.Fatalf("seed seven-day usage: %v", err)
	}

	// The user is already older than seven exact days, but the 10:00 hourly
	// bucket remains authoritative until 11:00 on the seventh day.
	boundary := recordedAt.Add(7*24*time.Hour + time.Minute)
	redisServer.SetTime(boundary)
	live, err := store.CurrentQuota(ctx, user.ID, boundary)
	if err != nil {
		t.Fatalf("load quota inside retained bucket: %v", err)
	}
	if live.Used7DTokens != 25 || live.Reset7DAt == nil || !live.Reset7DAt.After(boundary) {
		t.Fatalf("seven-day bucket is not live at the projection boundary: %#v", live)
	}

	if _, err := store.SyncQuotaStates(ctx, service); err != nil {
		t.Fatalf("sync quota projection inside retained bucket: %v", err)
	}
	var projected SubscriptionQuotaState
	if err := service.db.WithContext(ctx).Where("user_id = ?", user.ID).Take(&projected).Error; err != nil {
		t.Fatalf("load retained quota projection: %v", err)
	}
	if projected.Used7DTokens != 25 {
		t.Fatalf("live seven-day usage was pruned from SQL projection: %#v", projected)
	}
}

func TestQuotaStateProjectionPruneWaitsForLegacyMigration(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	if err := service.UpsertQuotaState(ctx, user.ID, QuotaStatus{HasSubscription: false}, time.Now().UTC()); err != nil {
		t.Fatalf("seed quota projection: %v", err)
	}
	if err := store.store.Client().SAdd(ctx, redisstore.SubscriptionActiveUsersKey(), "legacy-not-migrated").Err(); err != nil {
		t.Fatalf("seed pending legacy migration: %v", err)
	}

	if err := store.pruneQuotaStateProjections(ctx, service, nil); err != nil {
		t.Fatalf("defer quota projection prune: %v", err)
	}
	var retained int64
	if err := service.db.Model(&SubscriptionQuotaState{}).Where("user_id = ?", user.ID).Count(&retained).Error; err != nil {
		t.Fatalf("count retained quota projection: %v", err)
	}
	if retained != 1 {
		t.Fatalf("pending legacy migration pruned quota projection: count=%d", retained)
	}

	if err := store.store.Client().Del(ctx, redisstore.SubscriptionActiveUsersKey()).Err(); err != nil {
		t.Fatalf("complete legacy migration: %v", err)
	}
	if err := store.pruneQuotaStateProjections(ctx, service, nil); err != nil {
		t.Fatalf("prune quota projection after migration: %v", err)
	}
	if err := service.db.Model(&SubscriptionQuotaState{}).Where("user_id = ?", user.ID).Count(&retained).Error; err != nil {
		t.Fatalf("count pruned quota projection: %v", err)
	}
	if retained != 0 {
		t.Fatalf("completed legacy migration retained stale quota projection: count=%d", retained)
	}
}

func TestActiveUserIDsMigratesLegacySetInBoundedBatches(t *testing.T) {
	_, store, _ := newSubscriptionTestStore(t)
	ctx := context.Background()
	legacyIDs := make([]any, 0, 150)
	for index := 1; index <= 150; index++ {
		legacyIDs = append(legacyIDs, fmt.Sprintf("00000000-0000-0000-0000-%012d", index))
	}
	if err := store.store.Client().SAdd(ctx, redisstore.SubscriptionActiveUsersKey(), legacyIDs...).Err(); err != nil {
		t.Fatalf("seed legacy active users: %v", err)
	}

	first, err := store.ActiveUserIDs(ctx)
	if err != nil {
		t.Fatalf("migrate first legacy batch: %v", err)
	}
	if len(first) != 100 {
		t.Fatalf("expected bounded first migration batch of 100, got %d", len(first))
	}
	if remaining, err := store.store.Client().SCard(ctx, redisstore.SubscriptionActiveUsersKey()).Result(); err != nil || remaining != 50 {
		t.Fatalf("unexpected legacy remainder after first batch: remaining=%d err=%v", remaining, err)
	}
	if migrated, err := store.store.Client().ZCard(ctx, redisstore.SubscriptionActiveUsersZSetKey()).Result(); err != nil || migrated != 100 {
		t.Fatalf("unexpected migrated zset size after first batch: migrated=%d err=%v", migrated, err)
	}

	second, err := store.ActiveUserIDs(ctx)
	if err != nil {
		t.Fatalf("migrate second legacy batch: %v", err)
	}
	if len(second) != 150 {
		t.Fatalf("expected all migrated users on second run, got %d", len(second))
	}
	if remaining, err := store.store.Client().SCard(ctx, redisstore.SubscriptionActiveUsersKey()).Result(); err != nil || remaining != 0 {
		t.Fatalf("legacy set was not drained: remaining=%d err=%v", remaining, err)
	}
	if migrated, err := store.store.Client().ZCard(ctx, redisstore.SubscriptionActiveUsersZSetKey()).Result(); err != nil || migrated != 150 {
		t.Fatalf("unexpected final migrated zset size: migrated=%d err=%v", migrated, err)
	}
}

func TestExistingUserIDSetBatchesLargeCandidateList(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	candidates := make([]string, 0, 1002)
	candidates = append(candidates, user.ID)
	for index := 1; index <= 1001; index++ {
		candidates = append(candidates, fmt.Sprintf("00000000-0000-0000-0000-%012d", index))
	}

	existing, err := service.existingUserIDSet(context.Background(), candidates)
	if err != nil {
		t.Fatalf("load existing users in batches: %v", err)
	}
	if len(existing) != 1 {
		t.Fatalf("expected only the seeded user, got %#v", existing)
	}
	if _, ok := existing[user.ID]; !ok {
		t.Fatalf("seeded user %q missing from existing set", user.ID)
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
		ExternalSub:       testPointer("sub"),
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
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

func newSubscriptionRedisStoreAt(
	t *testing.T,
	service *Service,
	redisTime time.Time,
) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	ctx := context.Background()
	redisServer := miniredis.RunT(t)
	redisServer.SetTime(redisTime.UTC())
	redisBackend, err := redisstore.NewRequired(ctx, "redis://"+redisServer.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new time-controlled redis store: %v", err)
	}
	t.Cleanup(func() { _ = redisBackend.Close() })
	store := NewRedisStore(redisBackend, service, time.Minute, nil)
	store.SyncVersion(ctx)
	return store, redisServer
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
		`CREATE TABLE proxy_daily_usage_kpis (
			day date NOT NULL,
			initiator_id text NOT NULL,
			requests integer NOT NULL DEFAULT 0,
			PRIMARY KEY (day, initiator_id)
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
