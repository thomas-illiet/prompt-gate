package subscriptions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/redisstore"
)

func TestGrantQuotaResetCreditIsIdempotentCappedAndHashed(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{Name: "Default", Quota5HTokens: &quota, IsDefault: true}); err != nil {
		t.Fatalf("create resettable plan: %v", err)
	}
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	actor := QuotaResetActor{Type: QuotaResetActorAdminAPIKey}

	first, err := service.GrantQuotaResetCredit(ctx, user.ID, actor, "grant-request-1", now)
	if err != nil {
		t.Fatalf("grant first reset credit: %v", err)
	}
	if first.Available != 1 || first.Maximum != MaxQuotaResetCredits || !first.HasResettableQuota {
		t.Fatalf("unexpected first summary: %#v", first)
	}
	replayed, err := service.GrantQuotaResetCredit(ctx, user.ID, actor, "grant-request-1", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("replay reset credit grant: %v", err)
	}
	if replayed.Available != 1 {
		t.Fatalf("idempotent replay changed balance: %#v", replayed)
	}

	var event QuotaResetEvent
	if err := service.db.Where("user_id = ? AND type = ?", user.ID, QuotaResetEventAdminGrant).Take(&event).Error; err != nil {
		t.Fatalf("load reset grant event: %v", err)
	}
	digest := sha256.Sum256([]byte("grant-request-1"))
	wantHash := hex.EncodeToString(digest[:])
	if event.IdempotencyKey == nil || *event.IdempotencyKey != wantHash || *event.IdempotencyKey == "grant-request-1" {
		t.Fatalf("idempotency key was not hashed: %#v", event.IdempotencyKey)
	}
	if event.ActorType != QuotaResetActorAdminAPIKey || event.ActorUserID != nil {
		t.Fatalf("unexpected API-key actor audit: %#v", event)
	}

	second, err := service.GrantQuotaResetCredit(ctx, user.ID, actor, "grant-request-2", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("grant second reset credit: %v", err)
	}
	if second.Available != MaxQuotaResetCredits {
		t.Fatalf("expected full balance, got %#v", second)
	}
	if _, err := service.GrantQuotaResetCredit(ctx, user.ID, actor, "grant-request-3", now.Add(3*time.Minute)); !errors.Is(err, ErrQuotaResetBalanceFull) {
		t.Fatalf("expected balance-full error, got %v", err)
	}
}

func TestUseQuotaResetAppliesAtAnyFiniteUsageAndPreservesAnalytics(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{
		Name: "Default", Quota5HTokens: &quota, Quota7DTokens: &quota, IsDefault: true,
	}); err != nil {
		t.Fatalf("create resettable plan: %v", err)
	}
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm subscription snapshot: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Minute)
	if _, err := service.GrantQuotaResetCredit(ctx, user.ID, QuotaResetActor{Type: QuotaResetActorAdminAPIKey}, "grant-use", now.Add(-time.Minute)); err != nil {
		t.Fatalf("grant reset credit: %v", err)
	}
	if err := store.IncrementUsage(ctx, user.ID, 40, now.Add(-time.Minute)); err != nil {
		t.Fatalf("seed quota usage: %v", err)
	}
	seedSubscriptionTokenConsumption(t, service.db, user.ID, "reset-analytics", 31, 9)

	result, err := service.UseQuotaReset(ctx, store, user.ID, "use-request-1", now)
	if err != nil {
		t.Fatalf("use reset credit below quota ceiling: %v", err)
	}
	if result.Status != QuotaResetUseApplied || result.Summary.Available != 0 || result.Summary.PendingReset {
		t.Fatalf("unexpected applied reset result: %#v", result)
	}
	if result.Quota.Used5HTokens != 0 || result.Quota.Used7DTokens != 0 {
		t.Fatalf("quota counters were not reset: %#v", result.Quota)
	}
	var interceptionCount, usageCount int64
	if err := service.db.Table("interceptions").Count(&interceptionCount).Error; err != nil {
		t.Fatalf("count interceptions: %v", err)
	}
	if err := service.db.Table("token_usages").Count(&usageCount).Error; err != nil {
		t.Fatalf("count token usage: %v", err)
	}
	if interceptionCount != 1 || usageCount != 1 {
		t.Fatalf("quota reset modified analytics: interceptions=%d usages=%d", interceptionCount, usageCount)
	}

	if err := store.IncrementUsage(ctx, user.ID, 7, now.Add(time.Minute)); err != nil {
		t.Fatalf("record post-reset usage: %v", err)
	}
	if err := service.db.Model(&SubscriptionPlan{}).
		Where("is_default = ?", true).
		Updates(map[string]any{"quota_5h_tokens": nil, "quota_7d_tokens": nil}).Error; err != nil {
		t.Fatalf("remove finite quotas before replay: %v", err)
	}
	replayed, err := service.UseQuotaReset(ctx, store, user.ID, "use-request-1", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("replay applied reset: %v", err)
	}
	if replayed.Status != QuotaResetUseApplied || replayed.Summary.HasResettableQuota || replayed.Quota.Used5HTokens != 7 || replayed.Quota.Used7DTokens != 7 {
		t.Fatalf("idempotent replay cleared newer usage: %#v", replayed)
	}

	var resetEvents int64
	if err := service.db.Model(&QuotaResetEvent{}).
		Where("user_id = ? AND type = ?", user.ID, QuotaResetEventUsed).
		Count(&resetEvents).Error; err != nil {
		t.Fatalf("count reset use events: %v", err)
	}
	if resetEvents != 1 {
		t.Fatalf("expected one idempotent reset event, got %d", resetEvents)
	}
}

func TestUseQuotaResetRefreshesStalePlanBeforeLiveProjection(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	service.SetNotifier(store.store)

	plan, err := service.CreatePlan(ctx, PlanInput{Name: "Initially unlimited", IsDefault: true})
	if err != nil {
		t.Fatalf("create initial plan: %v", err)
	}
	store.SyncVersion(ctx)
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm initial unlimited snapshot: %v", err)
	}

	quota := int64(100)
	if _, err := service.UpdatePlan(ctx, plan.ID, PlanInput{
		Name: "Now limited", Quota5HTokens: &quota, IsDefault: true,
	}); err != nil {
		t.Fatalf("make plan resettable: %v", err)
	}
	if _, err := service.GrantQuotaResetCredit(
		ctx,
		user.ID,
		QuotaResetActor{Type: QuotaResetActorAdminAPIKey},
		"grant-after-plan-update",
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("grant reset after plan update: %v", err)
	}

	result, err := service.UseQuotaReset(ctx, store, user.ID, "use-after-plan-update", time.Now().UTC())
	if err != nil {
		t.Fatalf("use reset after plan update: %v", err)
	}
	if result.Status != QuotaResetUseApplied || result.Quota.Quota5HTokens == nil || *result.Quota.Quota5HTokens != quota {
		t.Fatalf("reset projected a stale subscription plan: %#v", result)
	}
	if store.version.Load() != 2 {
		t.Fatalf("quota reset did not refresh the Redis version: got %d, want 2", store.version.Load())
	}

	updatedQuota := int64(200)
	if _, err := service.UpdatePlan(ctx, plan.ID, PlanInput{
		Name: "Limited again", Quota5HTokens: &updatedQuota, IsDefault: true,
	}); err != nil {
		t.Fatalf("update plan before idempotent replay: %v", err)
	}
	replayed, err := service.UseQuotaReset(ctx, store, user.ID, "use-after-plan-update", time.Now().UTC())
	if err != nil {
		t.Fatalf("replay reset after second plan update: %v", err)
	}
	if replayed.Status != QuotaResetUseApplied || replayed.Quota.Quota5HTokens == nil || *replayed.Quota.Quota5HTokens != updatedQuota {
		t.Fatalf("reset replay projected a stale subscription plan: %#v", replayed)
	}
	if store.version.Load() != 3 {
		t.Fatalf("quota reset replay did not refresh the Redis version: got %d, want 3", store.version.Load())
	}
}

func TestUseQuotaResetRemainsPendingAndReconciles(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{Name: "Default", Quota7DTokens: &quota, IsDefault: true}); err != nil {
		t.Fatalf("create resettable plan: %v", err)
	}
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm subscription snapshot: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second).Add(123456789 * time.Nanosecond)
	if _, err := service.GrantQuotaResetCredit(ctx, user.ID, QuotaResetActor{Type: QuotaResetActorAdminAPIKey}, "grant-pending", now.Add(-time.Minute)); err != nil {
		t.Fatalf("grant reset credit: %v", err)
	}

	pending, err := service.UseQuotaReset(ctx, nil, user.ID, "use-pending", now)
	if err != nil {
		t.Fatalf("reserve pending reset: %v", err)
	}
	if pending.Status != QuotaResetUsePending || !pending.Summary.PendingReset || pending.Summary.Available != 0 {
		t.Fatalf("unexpected pending result: %#v", pending)
	}
	var pendingEvent QuotaResetEvent
	if err := service.db.Where("user_id = ? AND type = ?", user.ID, QuotaResetEventUsed).Take(&pendingEvent).Error; err != nil {
		t.Fatalf("load pending reset: %v", err)
	}
	if pendingEvent.ResetCutoffAt != nil {
		t.Fatalf("pending reset exposed a cutoff before Redis applied it: %#v", pendingEvent.ResetCutoffAt)
	}
	history, err := service.ListQuotaResetHistory(ctx, user.ID, QuotaResetHistoryParams{})
	if err != nil {
		t.Fatalf("list history with pending reset: %v", err)
	}
	if history.Total != 1 || len(history.Items) != 1 || history.Items[0].Type != QuotaResetEventAdminGrant {
		t.Fatalf("pending reset leaked into history: %#v", history)
	}
	if _, err := service.UseQuotaReset(ctx, nil, user.ID, "different-request", now); !errors.Is(err, ErrQuotaResetPending) {
		t.Fatalf("expected second reset to be rejected while pending, got %v", err)
	}
	if err := store.IncrementUsage(ctx, user.ID, 13, now.Add(time.Hour)); err != nil {
		t.Fatalf("record usage after reset reservation: %v", err)
	}
	activeBefore, err := store.store.Client().ZScore(ctx, redisstore.SubscriptionActiveUsersZSetKey(), user.ID).Result()
	if err != nil {
		t.Fatalf("load active-user score before reconciliation: %v", err)
	}

	applied, err := service.ReconcilePendingQuotaResets(ctx, store, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("reconcile pending reset: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected one reconciled reset, got %d", applied)
	}
	summary, err := service.QuotaResetSummary(ctx, user.ID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("load reconciled summary: %v", err)
	}
	if summary.PendingReset {
		t.Fatalf("pending flag survived reconciliation: %#v", summary)
	}
	quotaStatus, err := store.CurrentQuota(ctx, user.ID, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("load usage after delayed reconciliation: %v", err)
	}
	if quotaStatus.Used5HTokens != 0 || quotaStatus.Used7DTokens != 0 {
		t.Fatalf("reconciled reset did not clear both live windows: %#v", quotaStatus)
	}
	activeAfter, err := store.store.Client().ZScore(ctx, redisstore.SubscriptionActiveUsersZSetKey(), user.ID).Result()
	if err != nil {
		t.Fatalf("load active-user score after reconciliation: %v", err)
	}
	if activeAfter != activeBefore {
		t.Fatalf("delayed reset lowered active-user score: before=%v after=%v", activeBefore, activeAfter)
	}
	history, err = service.ListQuotaResetHistory(ctx, user.ID, QuotaResetHistoryParams{})
	if err != nil {
		t.Fatalf("list reconciled history: %v", err)
	}
	if history.Total != 2 || history.Items[0].Type != QuotaResetEventUsed {
		t.Fatalf("applied reset missing from history: %#v", history)
	}
	if err := service.db.Where("id = ?", pendingEvent.ID).Take(&pendingEvent).Error; err != nil {
		t.Fatalf("reload applied reset event: %v", err)
	}
	if pendingEvent.ResetCutoffAt == nil || pendingEvent.ResetCutoffAt.IsZero() {
		t.Fatalf("applied reset did not persist the Redis cutoff: %#v", pendingEvent)
	}
}

func TestApplyQuotaResetSequencePreventsStaleReplay(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{
		Name: "Default", Quota5HTokens: &quota, Quota7DTokens: &quota, IsDefault: true,
	}); err != nil {
		t.Fatalf("create resettable plan: %v", err)
	}
	if err := store.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm subscription snapshot: %v", err)
	}
	beforeReset := time.Now().UTC().Add(-time.Minute)
	if err := store.IncrementUsage(ctx, user.ID, 20, beforeReset); err != nil {
		t.Fatalf("seed pre-reset usage: %v", err)
	}
	if err := store.store.Client().SAdd(ctx, redisstore.SubscriptionActiveUsersKey(), user.ID).Err(); err != nil {
		t.Fatalf("seed legacy active-user marker: %v", err)
	}
	first, err := store.ApplyQuotaReset(ctx, user.ID, 1)
	if err != nil || !first.Advanced || first.Sequence != 1 {
		t.Fatalf("apply first reset: %#v, %v", first, err)
	}
	legacyMember, err := store.store.Client().SIsMember(ctx, redisstore.SubscriptionActiveUsersKey(), user.ID).Result()
	if err != nil {
		t.Fatalf("inspect legacy active-user marker: %v", err)
	}
	if legacyMember {
		t.Fatal("quota reset did not remove the legacy active-user marker")
	}
	if err := store.IncrementUsage(ctx, user.ID, 99, first.Cutoff.Add(-time.Microsecond)); err != nil {
		t.Fatalf("process late pre-reset usage: %v", err)
	}
	if err := store.IncrementUsage(ctx, user.ID, 6, first.Cutoff.Add(time.Microsecond)); err != nil {
		t.Fatalf("record post-reset usage: %v", err)
	}
	replay, err := store.ApplyQuotaReset(ctx, user.ID, 1)
	if err != nil || replay.Advanced || replay.Sequence != 1 || !replay.Cutoff.Equal(first.Cutoff) {
		t.Fatalf("unexpected reset replay: %#v, %v", replay, err)
	}
	status, err := store.CurrentQuota(ctx, user.ID, first.Cutoff.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("load quota after stale replay: %v", err)
	}
	if status.Used5HTokens != 6 || status.Used7DTokens != 6 {
		t.Fatalf("stale reset cleared newer usage: %#v", status)
	}

	newerSequence, err := store.ApplyQuotaReset(ctx, user.ID, 2)
	if err != nil || !newerSequence.Advanced || newerSequence.Sequence != 2 || newerSequence.Cutoff.Before(first.Cutoff) {
		t.Fatalf("apply newer reset sequence: %#v, %v", newerSequence, err)
	}
	usageKeys := []string{
		redisstore.SubscriptionUsageZSetKey(user.ID, Window5H),
		redisstore.SubscriptionUsageHashKey(user.ID, Window5H),
		redisstore.SubscriptionUsageZSetKey(user.ID, Window7D),
		redisstore.SubscriptionUsageHashKey(user.ID, Window7D),
	}
	remainingKeys, err := store.store.Client().Exists(ctx, usageKeys...).Result()
	if err != nil {
		t.Fatalf("inspect reset usage keys: %v", err)
	}
	if remainingKeys != 0 {
		t.Fatalf("new reset sequence left %d live usage keys", remainingKeys)
	}
	if err := store.IncrementUsage(ctx, user.ID, 50, newerSequence.Cutoff.Add(-time.Microsecond)); err != nil {
		t.Fatalf("process late usage after newer reset: %v", err)
	}
	if err := store.IncrementUsage(ctx, user.ID, 4, newerSequence.Cutoff.Add(time.Microsecond)); err != nil {
		t.Fatalf("record usage after newer reset: %v", err)
	}
	status, err = store.CurrentQuota(ctx, user.ID, newerSequence.Cutoff.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("load quota after newer reset: %v", err)
	}
	if status.Used5HTokens != 4 || status.Used7DTokens != 4 {
		t.Fatalf("newer reset admitted old usage or rejected new usage: %#v", status)
	}

	futureCutoff := newerSequence.Cutoff.Add(time.Hour).Truncate(time.Microsecond)
	if err := store.store.Client().HSet(
		ctx,
		redisstore.SubscriptionQuotaResetStateKey(user.ID),
		"cutoff",
		futureCutoff.UnixMicro(),
	).Err(); err != nil {
		t.Fatalf("seed future reset cutoff: %v", err)
	}
	third, err := store.ApplyQuotaReset(ctx, user.ID, 3)
	if err != nil || !third.Advanced || third.Sequence != 3 || !third.Cutoff.Equal(futureCutoff) {
		t.Fatalf("new reset did not preserve monotone cutoff: %#v, %v", third, err)
	}
	status, err = store.CurrentQuota(ctx, user.ID, futureCutoff.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("load quota after monotone-cutoff reset: %v", err)
	}
	if status.Used5HTokens != 0 || status.Used7DTokens != 0 {
		t.Fatalf("new sequence did not clear usage under a future cutoff: %#v", status)
	}
}

func TestQuotaResetHistoryIsStrictlyScoped(t *testing.T) {
	service, _, firstUser := newSubscriptionTestStore(t)
	ctx := context.Background()
	secondUser := users.User{
		ID: "22222222-2222-2222-2222-222222222222", ExternalSub: quotaResetStringPtr("second"), Email: "second@example.com",
		PreferredUsername: "second", Name: "Second", Type: auth.UserTypeUser, Role: auth.RoleUser, IsActive: true,
	}
	if err := service.db.Create(&secondUser).Error; err != nil {
		t.Fatalf("create second user: %v", err)
	}
	now := time.Date(2026, 10, 23, 12, 30, 0, 0, time.UTC)
	actor := QuotaResetActor{Type: QuotaResetActorAdminAPIKey}
	if _, err := service.GrantQuotaResetCredit(ctx, firstUser.ID, actor, "first-grant", now); err != nil {
		t.Fatalf("grant first user: %v", err)
	}
	if _, err := service.GrantQuotaResetCredit(ctx, secondUser.ID, actor, "second-grant", now); err != nil {
		t.Fatalf("grant second user: %v", err)
	}
	history, err := service.ListQuotaResetHistory(ctx, firstUser.ID, QuotaResetHistoryParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list first user history: %v", err)
	}
	if history.Total != 1 || len(history.Items) != 1 || history.Items[0].Type != QuotaResetEventAdminGrant {
		t.Fatalf("history included another user's event: %#v", history)
	}
}

func TestGrantQuotaResetCreditSupportsInactiveHumanWithoutPlanAndRejectsServiceAccount(t *testing.T) {
	service, _, human := newSubscriptionTestStore(t)
	ctx := context.Background()
	if err := service.db.Model(&users.User{}).Where("id = ?", human.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate human user: %v", err)
	}
	actor := QuotaResetActor{Type: QuotaResetActorAdminAPIKey}
	summary, err := service.GrantQuotaResetCredit(ctx, human.ID, actor, "inactive-no-plan", time.Now().UTC())
	if err != nil {
		t.Fatalf("grant inactive human without plan: %v", err)
	}
	if summary.Available != 1 || summary.HasResettableQuota {
		t.Fatalf("unexpected inactive no-plan summary: %#v", summary)
	}
	serviceAccount := users.User{
		ID: "33333333-3333-3333-3333-333333333333", ExternalSub: quotaResetStringPtr("service"), PreferredUsername: "service",
		Name: "Service", Type: auth.UserTypeService, Role: auth.RoleUser, IsActive: true,
	}
	if err := service.db.Create(&serviceAccount).Error; err != nil {
		t.Fatalf("create service account: %v", err)
	}
	if _, err := service.GrantQuotaResetCredit(ctx, serviceAccount.ID, actor, "service-grant", time.Now().UTC()); !errors.Is(err, ErrQuotaResetUserNotFound) {
		t.Fatalf("expected service account rejection, got %v", err)
	}
}

func TestConcurrentQuotaResetGrantsRespectMaximum(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	sqlDB, err := service.db.DB()
	if err != nil {
		t.Fatalf("load SQL database: %v", err)
	}
	// SQLite does not implement PostgreSQL row locks. A single connection keeps
	// this portability test deterministic; production serialization is provided
	// by SELECT FOR UPDATE in lockQuotaResetWallet.
	sqlDB.SetMaxOpenConns(1)

	ctx := context.Background()
	actor := QuotaResetActor{Type: QuotaResetActorAdminAPIKey}
	start := make(chan struct{})
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, err := service.GrantQuotaResetCredit(ctx, user.ID, actor, "concurrent-"+string(rune('a'+index)), time.Now().UTC())
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	var granted, full int
	for err := range errs {
		switch {
		case err == nil:
			granted++
		case errors.Is(err, ErrQuotaResetBalanceFull):
			full++
		default:
			t.Fatalf("unexpected concurrent grant error: %v", err)
		}
	}
	if granted != MaxQuotaResetCredits || full != 8-MaxQuotaResetCredits {
		t.Fatalf("unexpected concurrent outcomes: granted=%d full=%d", granted, full)
	}
	var wallet QuotaResetWallet
	if err := service.db.Where("user_id = ?", user.ID).Take(&wallet).Error; err != nil {
		t.Fatalf("load wallet after concurrent grants: %v", err)
	}
	if wallet.Available != MaxQuotaResetCredits {
		t.Fatalf("concurrent grants exceeded maximum: %#v", wallet)
	}
}

func TestMonthlyQuotaResetDistributionHonorsEligibilityExpiryAndCap(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{Name: "Default", Quota5HTokens: &quota, IsDefault: true}); err != nil {
		t.Fatalf("create resettable plan: %v", err)
	}
	now := time.Date(2026, 10, 23, 13, 0, 0, 0, time.UTC)
	expired := users.User{
		ID: "22222222-2222-2222-2222-222222222222", ExternalSub: quotaResetStringPtr("expired"), PreferredUsername: "expired",
		Type: auth.UserTypeUser, Role: auth.RoleUser, IsActive: true, ExpiresAt: quotaResetTimePtr(now.Add(-time.Minute)),
	}
	if err := service.db.Create(&expired).Error; err != nil {
		t.Fatalf("create expired user: %v", err)
	}

	first, err := service.DistributeMonthlyQuotaResetCredits(ctx, now)
	if err != nil {
		t.Fatalf("distribute monthly reset: %v", err)
	}
	if first.Eligible != 1 || first.Granted != 1 || first.AlreadyProcessed != 0 || first.SkippedAtMaximum != 0 {
		t.Fatalf("unexpected first distribution: %#v", first)
	}
	replay, err := service.DistributeMonthlyQuotaResetCredits(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("repeat monthly distribution: %v", err)
	}
	if replay.Eligible != 1 || replay.AlreadyProcessed != 1 || replay.Granted != 0 {
		t.Fatalf("distribution was not idempotent: %#v", replay)
	}

	if _, err := service.GrantQuotaResetCredit(ctx, user.ID, QuotaResetActor{Type: QuotaResetActorAdminAPIKey}, "fill-wallet", now.Add(2*time.Hour)); err != nil {
		t.Fatalf("fill reset wallet: %v", err)
	}
	nextMonth := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	capped, err := service.DistributeMonthlyQuotaResetCredits(ctx, nextMonth)
	if err != nil {
		t.Fatalf("distribute at cap: %v", err)
	}
	if capped.Eligible != 1 || capped.SkippedAtMaximum != 1 || capped.Granted != 0 {
		t.Fatalf("unexpected capped distribution: %#v", capped)
	}
	if err := service.db.Model(&QuotaResetWallet{}).Where("user_id = ?", user.ID).Update("available", 1).Error; err != nil {
		t.Fatalf("consume test credit: %v", err)
	}
	late, err := service.DistributeMonthlyQuotaResetCredits(ctx, nextMonth.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("repeat capped month distribution: %v", err)
	}
	if late.AlreadyProcessed != 1 || late.Granted != 0 {
		t.Fatalf("capped month was backfilled after consumption: %#v", late)
	}
}

func TestMonthlyQuotaResetDistributionIncludesNewUserAndUTCMonthBoundary(t *testing.T) {
	service, _, firstUser := newSubscriptionTestStore(t)
	ctx := context.Background()
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{Name: "Default", Quota5HTokens: &quota, IsDefault: true}); err != nil {
		t.Fatalf("create resettable plan: %v", err)
	}
	endOfOctober := time.Date(2026, 10, 31, 23, 59, 59, 0, time.UTC)
	first, err := service.DistributeMonthlyQuotaResetCredits(ctx, endOfOctober)
	if err != nil || first.Granted != 1 {
		t.Fatalf("initial October distribution: %#v, %v", first, err)
	}

	newUser := users.User{
		ID: "44444444-4444-4444-4444-444444444444", ExternalSub: quotaResetStringPtr("new-user"), Email: "new@example.com",
		PreferredUsername: "new-user", Name: "New User", Type: auth.UserTypeUser, Role: auth.RoleUser, IsActive: true,
	}
	if err := service.db.Create(&newUser).Error; err != nil {
		t.Fatalf("create newly eligible user: %v", err)
	}
	sameMonth, err := service.DistributeMonthlyQuotaResetCredits(ctx, endOfOctober.Add(500*time.Millisecond))
	if err != nil {
		t.Fatalf("distribute to new user in current month: %v", err)
	}
	if sameMonth.Eligible != 2 || sameMonth.Granted != 1 || sameMonth.AlreadyProcessed != 1 {
		t.Fatalf("newly eligible user missed current month: %#v", sameMonth)
	}

	startOfNovember := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	nextMonth, err := service.DistributeMonthlyQuotaResetCredits(ctx, startOfNovember)
	if err != nil {
		t.Fatalf("distribute at UTC month boundary: %v", err)
	}
	if nextMonth.Eligible != 2 || nextMonth.Granted != 2 {
		t.Fatalf("UTC month boundary was not treated as a new month: %#v", nextMonth)
	}
	for _, userID := range []string{firstUser.ID, newUser.ID} {
		var wallet QuotaResetWallet
		if err := service.db.Where("user_id = ?", userID).Take(&wallet).Error; err != nil {
			t.Fatalf("load wallet %s: %v", userID, err)
		}
		if wallet.Available != MaxQuotaResetCredits {
			t.Fatalf("unexpected balance at UTC boundary for %s: %#v", userID, wallet)
		}
	}
}

func TestUpsertQuotaStateRejectsOlderProjection(t *testing.T) {
	service, _, user := newSubscriptionTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 23, 14, 0, 0, 0, time.UTC)
	quota := int64(100)
	newer := QuotaStatus{HasSubscription: true, Used5HTokens: 9, Quota5HTokens: &quota}
	older := QuotaStatus{HasSubscription: true, Used5HTokens: 99, Quota5HTokens: &quota}
	if err := service.UpsertQuotaState(ctx, user.ID, newer, now); err != nil {
		t.Fatalf("write newer quota projection: %v", err)
	}
	if err := service.UpsertQuotaState(ctx, user.ID, older, now.Add(-time.Minute)); err != nil {
		t.Fatalf("write older quota projection: %v", err)
	}
	var state SubscriptionQuotaState
	if err := service.db.Where("user_id = ?", user.ID).Take(&state).Error; err != nil {
		t.Fatalf("load quota projection: %v", err)
	}
	if state.Used5HTokens != 9 || !state.SyncedAt.Equal(now) {
		t.Fatalf("older projection overwrote newer state: %#v", state)
	}
}

func quotaResetTimePtr(value time.Time) *time.Time {
	return &value
}

func quotaResetStringPtr(value string) *string {
	return &value
}
