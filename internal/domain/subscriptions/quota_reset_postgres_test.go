package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestQuotaResetPostgresConcurrentGrantsRespectMaximum proves that the
// SELECT FOR UPDATE wallet lock enforces the two-credit cap on PostgreSQL.
// It is opt-in so the regular unit suite stays self-contained.
func TestQuotaResetPostgresConcurrentGrantsRespectMaximum(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("PROMPTGATE_TEST_POSTGRES_URL"))
	if databaseURL == "" {
		t.Skip("PROMPTGATE_TEST_POSTGRES_URL is not set")
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil || parsedURL.Scheme == "" {
		t.Fatalf("PROMPTGATE_TEST_POSTGRES_URL must be a PostgreSQL URL: %v", err)
	}

	baseDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL integration database: %v", err)
	}
	baseSQL, err := baseDB.DB()
	if err != nil {
		t.Fatalf("expose PostgreSQL integration database: %v", err)
	}
	defer baseSQL.Close()

	schema := fmt.Sprintf("quota_reset_%d", time.Now().UTC().UnixNano())
	if err := baseDB.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create PostgreSQL integration schema: %v", err)
	}
	defer func() {
		if err := baseDB.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop PostgreSQL integration schema: %v", err)
		}
	}()

	query := parsedURL.Query()
	query.Set("search_path", schema)
	parsedURL.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsedURL.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL integration schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("expose isolated PostgreSQL integration schema: %v", err)
	}
	defer sqlDB.Close()

	ctx := context.Background()
	userService := users.NewService(db)
	if err := userService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate PostgreSQL users: %v", err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate PostgreSQL quota reset tables: %v", err)
	}
	quota := int64(100)
	if _, err := service.CreatePlan(ctx, PlanInput{
		Name: "PostgreSQL quota reset", Quota5HTokens: &quota, IsDefault: true,
	}); err != nil {
		t.Fatalf("create PostgreSQL resettable plan: %v", err)
	}
	profile, err := userService.SyncUser(ctx, auth.Identity{
		Sub:               "postgres-quota-reset",
		PreferredUsername: "postgres-quota-reset",
		Email:             "postgres-quota-reset@example.com",
		Name:              "PostgreSQL Quota Reset",
	})
	if err != nil {
		t.Fatalf("create PostgreSQL quota reset user: %v", err)
	}

	actor := QuotaResetActor{Type: QuotaResetActorAdminAPIKey}
	start := make(chan struct{})
	errorsByRequest := make(chan error, 8)
	var waitGroup sync.WaitGroup
	for index := 0; index < 8; index++ {
		waitGroup.Add(1)
		go func(requestIndex int) {
			defer waitGroup.Done()
			<-start
			_, grantErr := service.GrantQuotaResetCredit(
				ctx,
				profile.ID,
				actor,
				fmt.Sprintf("postgres-concurrent-%d", requestIndex),
				time.Now().UTC(),
			)
			errorsByRequest <- grantErr
		}(index)
	}
	close(start)
	waitGroup.Wait()
	close(errorsByRequest)

	var granted, full int
	for grantErr := range errorsByRequest {
		switch {
		case grantErr == nil:
			granted++
		case errors.Is(grantErr, ErrQuotaResetBalanceFull):
			full++
		default:
			t.Fatalf("unexpected concurrent PostgreSQL grant error: %v", grantErr)
		}
	}
	if granted != MaxQuotaResetCredits || full != 8-MaxQuotaResetCredits {
		t.Fatalf("unexpected PostgreSQL grant results: granted=%d full=%d", granted, full)
	}

	var wallet QuotaResetWallet
	if err := db.Where("user_id = ?", profile.ID).Take(&wallet).Error; err != nil {
		t.Fatalf("load PostgreSQL quota reset wallet: %v", err)
	}
	if wallet.Available != MaxQuotaResetCredits {
		t.Fatalf("PostgreSQL row lock allowed balance %d", wallet.Available)
	}

	useStart := make(chan struct{})
	useErrors := make(chan error, 2)
	for index := 0; index < 2; index++ {
		waitGroup.Add(1)
		go func(requestIndex int) {
			defer waitGroup.Done()
			<-useStart
			_, useErr := service.UseQuotaReset(
				ctx,
				nil,
				profile.ID,
				fmt.Sprintf("postgres-use-%d", requestIndex),
				time.Now().UTC(),
			)
			useErrors <- useErr
		}(index)
	}
	close(useStart)
	waitGroup.Wait()
	close(useErrors)

	var reserved, blockedPending int
	for useErr := range useErrors {
		switch {
		case useErr == nil:
			reserved++
		case errors.Is(useErr, ErrQuotaResetPending):
			blockedPending++
		default:
			t.Fatalf("unexpected concurrent PostgreSQL use error: %v", useErr)
		}
	}
	if reserved != 1 || blockedPending != 1 {
		t.Fatalf("unexpected PostgreSQL use results: reserved=%d pending=%d", reserved, blockedPending)
	}
	if err := db.Where("user_id = ?", profile.ID).Take(&wallet).Error; err != nil {
		t.Fatalf("reload PostgreSQL quota reset wallet: %v", err)
	}
	if wallet.Available != MaxQuotaResetCredits-1 {
		t.Fatalf("concurrent uses debited the wallet more than once: %#v", wallet)
	}
	var pendingEvents int64
	if err := db.Model(&QuotaResetEvent{}).
		Where("user_id = ? AND type = ? AND status = ?", profile.ID, QuotaResetEventUsed, QuotaResetEventPending).
		Count(&pendingEvents).Error; err != nil {
		t.Fatalf("count PostgreSQL pending reset events: %v", err)
	}
	if pendingEvents != 1 {
		t.Fatalf("expected one pending PostgreSQL reset event, got %d", pendingEvents)
	}
}
