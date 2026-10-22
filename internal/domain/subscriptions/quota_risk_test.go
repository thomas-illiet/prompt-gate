package subscriptions

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type quotaRiskFixture struct {
	id              string
	name            string
	accountType     auth.UserType
	hasSubscription bool
	used5h          int64
	quota5h         *int64
	reset5h         *time.Time
	used7d          int64
	quota7d         *int64
	reset7d         *time.Time
	syncedAt        time.Time
}

func newQuotaRiskTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := users.NewService(db).AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}
	return service, db
}

func seedQuotaRiskFixture(t *testing.T, db *gorm.DB, fixture quotaRiskFixture) {
	t.Helper()
	user := users.User{
		ID:                fixture.id,
		ExternalSub:       "sub-" + fixture.id,
		Email:             fixture.id + "@example.com",
		PreferredUsername: "account-" + fixture.id,
		Name:              fixture.name,
		Type:              fixture.accountType,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       fixture.syncedAt,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed quota-risk account: %v", err)
	}
	state := SubscriptionQuotaState{
		UserID:          fixture.id,
		HasSubscription: fixture.hasSubscription,
		PlanName:        "Team",
		Used5HTokens:    fixture.used5h,
		Quota5HTokens:   fixture.quota5h,
		Reset5HAt:       fixture.reset5h,
		Used7DTokens:    fixture.used7d,
		Quota7DTokens:   fixture.quota7d,
		Reset7DAt:       fixture.reset7d,
		SyncedAt:        fixture.syncedAt,
	}
	if err := db.Create(&state).Error; err != nil {
		t.Fatalf("seed quota-risk state: %v", err)
	}
}

func quotaRiskInt64(value int64) *int64 {
	return &value
}

// TestDashboardQuotaRiskClassifiesBothAccountTypes verifies the SQL projection
// includes humans and service accounts, applies 80/100 thresholds, and ranks by
// severity before percentage while retaining counts beyond the top five.
func TestDashboardQuotaRiskClassifiesBothAccountTypes(t *testing.T) {
	service, db := newQuotaRiskTestService(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(2 * time.Hour)
	reset7d := now.Add(3 * 24 * time.Hour)
	fixtures := []quotaRiskFixture{
		{
			id: "00000000-0000-0000-0000-000000000001", name: "Human critical", accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 90, quota5h: quotaRiskInt64(100), reset5h: &reset5h,
			used7d: 120, quota7d: quotaRiskInt64(100), reset7d: &reset7d, syncedAt: now.Add(-8 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000002", name: "Service critical", accountType: auth.UserTypeService,
			hasSubscription: true, used5h: 100, quota5h: quotaRiskInt64(100), reset5h: &reset5h,
			used7d: 20, quota7d: quotaRiskInt64(100), reset7d: &reset7d, syncedAt: now.Add(-7 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000003", name: "Warning 99", accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 99, quota5h: quotaRiskInt64(100), syncedAt: now.Add(-6 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000004", name: "Warning 90", accountType: auth.UserTypeService,
			hasSubscription: true, used7d: 90, quota7d: quotaRiskInt64(100), syncedAt: now.Add(-5 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000005", name: "Warning 85", accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 85, quota5h: quotaRiskInt64(100), syncedAt: now.Add(-4 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000006", name: "Warning 80", accountType: auth.UserTypeService,
			hasSubscription: true, used5h: 80, quota5h: quotaRiskInt64(100), syncedAt: now.Add(-3 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000007", name: "Below threshold", accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 79, quota5h: quotaRiskInt64(100), syncedAt: now.Add(-2 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000008", name: "Unlimited", accountType: auth.UserTypeService,
			hasSubscription: true, used5h: 999, used7d: 999, syncedAt: now.Add(-time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000009", name: "No plan", accountType: auth.UserTypeUser,
			hasSubscription: false, used5h: 999, quota5h: quotaRiskInt64(100), syncedAt: now,
		},
	}
	for _, fixture := range fixtures {
		seedQuotaRiskFixture(t, db, fixture)
	}

	result, err := service.DashboardQuotaRisk(context.Background(), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("load dashboard quota risk: %v", err)
	}
	if result.GeneratedAt != now.Add(time.Minute) {
		t.Fatalf("unexpected generatedAt: %s", result.GeneratedAt)
	}
	if result.ProjectionSyncedAt == nil || !result.ProjectionSyncedAt.Equal(now) {
		t.Fatalf("expected latest SQL projection timestamp, got %v", result.ProjectionSyncedAt)
	}
	if result.AtRiskCount != 6 || result.ExhaustedCount != 2 {
		t.Fatalf("unexpected quota-risk counts: %#v", result)
	}
	if len(result.Items) != 5 {
		t.Fatalf("expected top five items, got %d", len(result.Items))
	}

	expectedIDs := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
		"00000000-0000-0000-0000-000000000004",
		"00000000-0000-0000-0000-000000000005",
	}
	for i, expectedID := range expectedIDs {
		if result.Items[i].AccountID != expectedID {
			t.Fatalf("unexpected item at %d: %#v", i, result.Items[i])
		}
	}
	if result.Items[0].AccountType != auth.UserTypeUser || result.Items[0].Window != Window7D || result.Items[0].Percent != 120 {
		t.Fatalf("expected human's worst 7d window first, got %#v", result.Items[0])
	}
	if result.Items[0].RemainingTokens != 0 || result.Items[0].ResetAt == nil || !result.Items[0].ResetAt.Equal(reset7d) {
		t.Fatalf("unexpected critical quota details: %#v", result.Items[0])
	}
	if result.Items[1].AccountType != auth.UserTypeService || result.Items[1].Status != QuotaRiskStatusCritical {
		t.Fatalf("expected critical service account second, got %#v", result.Items[1])
	}
}

// TestDashboardQuotaRiskReturnsAnExplicitEmptyList verifies an empty projection
// remains a stable JSON array and has no fabricated projection timestamp.
func TestDashboardQuotaRiskReturnsAnExplicitEmptyList(t *testing.T) {
	service, _ := newQuotaRiskTestService(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

	result, err := service.DashboardQuotaRisk(context.Background(), now)
	if err != nil {
		t.Fatalf("load empty dashboard quota risk: %v", err)
	}
	if result.Items == nil || len(result.Items) != 0 {
		t.Fatalf("expected explicit empty items, got %#v", result.Items)
	}
	if result.ProjectionSyncedAt != nil || result.AtRiskCount != 0 || result.ExhaustedCount != 0 {
		t.Fatalf("unexpected empty quota-risk response: %#v", result)
	}
}

// TestPruneQuotaStatesRemovesInactiveProjectionRows verifies the SQL snapshot
// cannot retain accounts after they leave Redis' rolling active set.
func TestPruneQuotaStatesRemovesInactiveProjectionRows(t *testing.T) {
	service, db := newQuotaRiskTestService(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	quota := quotaRiskInt64(100)
	activeID := "00000000-0000-0000-0000-000000000001"
	staleID := "00000000-0000-0000-0000-000000000002"
	for _, id := range []string{activeID, staleID} {
		seedQuotaRiskFixture(t, db, quotaRiskFixture{
			id: id, name: id, accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 90, quota5h: quota, syncedAt: now,
		})
	}

	if err := service.PruneQuotaStates(context.Background(), []string{activeID}); err != nil {
		t.Fatalf("prune inactive quota states: %v", err)
	}
	var states []SubscriptionQuotaState
	if err := db.Order("user_id").Find(&states).Error; err != nil {
		t.Fatalf("load retained quota states: %v", err)
	}
	if len(states) != 1 || states[0].UserID != activeID {
		t.Fatalf("unexpected retained quota states: %#v", states)
	}

	if err := service.PruneQuotaStates(context.Background(), nil); err != nil {
		t.Fatalf("prune empty active set: %v", err)
	}
	var count int64
	if err := db.Model(&SubscriptionQuotaState{}).Count(&count).Error; err != nil {
		t.Fatalf("count quota states: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty quota projection, got %d rows", count)
	}
}
