package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newQuotaResetJobsTestApp(t *testing.T) (*App, *gorm.DB, users.User) {
	t.Helper()
	ctx := context.Background()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	userService := users.NewService(db)
	if err := userService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	subscriptionService := subscriptions.NewService(db)
	if err := subscriptionService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}

	quota := int64(100)
	if _, err := subscriptionService.CreatePlan(ctx, subscriptions.PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create subscription plan: %v", err)
	}
	externalSub := "scheduler-reset-user"
	lastLoginAt := time.Now().UTC()
	user := users.User{
		ExternalSub:       &externalSub,
		Email:             "scheduler-reset@example.com",
		PreferredUsername: "scheduler-reset-user",
		Name:              "Scheduler Reset User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       &lastLoginAt,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return &App{DB: db, Users: userService, Subscriptions: subscriptionService}, db, user
}

func waitForQuotaResetJob(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for quota reset scheduler job")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartQuotaResetDistributionRunsImmediately(t *testing.T) {
	application, _, user := newQuotaResetJobsTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	application.StartQuotaResetDistribution(ctx, time.Hour)

	waitForQuotaResetJob(t, func() bool {
		summary, err := application.Subscriptions.QuotaResetSummary(ctx, user.ID, time.Now().UTC())
		return err == nil && summary.Available == 1
	})
}

func TestStartQuotaStateMaintenanceReconcilesBeforeProjectionSync(t *testing.T) {
	application, db, user := newQuotaResetJobsTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	redisServer := miniredis.RunT(t)
	redisBackend, err := redisstore.NewRequired(ctx, "redis://"+redisServer.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("create redis store: %v", err)
	}
	t.Cleanup(func() { _ = redisBackend.Close() })
	quotaStore := subscriptions.NewRedisStore(redisBackend, application.Subscriptions, time.Minute, nil)
	quotaStore.SyncVersion(ctx)
	if err := quotaStore.WarmSnapshot(ctx); err != nil {
		t.Fatalf("warm subscription snapshot: %v", err)
	}
	application.Redis = redisBackend
	application.QuotaRedis = quotaStore

	now := time.Now().UTC()
	if err := quotaStore.IncrementUsage(ctx, user.ID, 100, now); err != nil {
		t.Fatalf("increment quota usage: %v", err)
	}
	if _, err := application.Subscriptions.GrantQuotaResetCredit(
		ctx,
		user.ID,
		subscriptions.QuotaResetActor{Type: subscriptions.QuotaResetActorAdminAPIKey},
		"scheduler-grant",
		now,
	); err != nil {
		t.Fatalf("grant quota reset credit: %v", err)
	}
	use, err := application.Subscriptions.UseQuotaReset(ctx, nil, user.ID, "scheduler-use", now)
	if err != nil {
		t.Fatalf("reserve pending quota reset: %v", err)
	}
	if use.Status != subscriptions.QuotaResetUsePending {
		t.Fatalf("expected pending quota reset, got %q", use.Status)
	}

	application.StartQuotaStateMaintenance(ctx, time.Hour)

	waitForQuotaResetJob(t, func() bool {
		var event subscriptions.QuotaResetEvent
		if err := db.Where("user_id = ? AND type = ?", user.ID, subscriptions.QuotaResetEventUsed).
			Take(&event).Error; err != nil || event.Status != subscriptions.QuotaResetEventApplied {
			return false
		}
		var state subscriptions.SubscriptionQuotaState
		if err := db.Take(&state, "user_id = ?", user.ID).Error; err != nil {
			return false
		}
		return state.Used5HTokens == 0 && state.Used7DTokens == 0
	})
}
