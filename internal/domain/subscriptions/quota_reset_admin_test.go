package subscriptions

import (
	"context"
	"errors"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"gorm.io/gorm"
)

func TestAdminQuotaStatesClassifiesFiltersAndPaginatesProjection(t *testing.T) {
	service, db := newQuotaRiskTestService(t)
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(2 * time.Hour)
	reset7d := now.Add(5 * 24 * time.Hour)
	fixtures := []quotaRiskFixture{
		{
			id: "00000000-0000-0000-0000-000000000001", name: "Critical service", accountType: auth.UserTypeService,
			hasSubscription: true, used5h: 100, quota5h: quotaRiskInt64(100), reset5h: &reset5h,
			used7d: 5, quota7d: nil, reset7d: &reset7d, syncedAt: now.Add(-time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000002", name: "Warning human", accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 80, quota5h: quotaRiskInt64(100), reset5h: &reset5h,
			used7d: 20, quota7d: quotaRiskInt64(100), reset7d: &reset7d, syncedAt: now,
		},
		{
			id: "00000000-0000-0000-0000-000000000003", name: "Healthy human", accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 79, quota5h: quotaRiskInt64(100), reset5h: &reset5h,
			syncedAt: now.Add(-2 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000004", name: "Unlimited human", accountType: auth.UserTypeUser,
			hasSubscription: true, used5h: 900, quota5h: nil, used7d: 900, quota7d: nil,
			syncedAt: now.Add(-3 * time.Minute),
		},
		{
			id: "00000000-0000-0000-0000-000000000005", name: "Unassigned human", accountType: auth.UserTypeUser,
			hasSubscription: false, used5h: 900, quota5h: quotaRiskInt64(100), reset5h: &reset5h,
			syncedAt: now.Add(-4 * time.Minute),
		},
	}
	for _, fixture := range fixtures {
		seedQuotaRiskFixture(t, db, fixture)
	}

	result, err := service.AdminQuotaStates(context.Background(), AdminQuotaStateParams{}, now)
	if err != nil {
		t.Fatalf("list default quota states: %v", err)
	}
	if result.Page != 1 || result.PageSize != 20 || result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("unexpected default page: %+v", result)
	}
	if result.Summary != (AdminQuotaStateSummary{ProjectedAccounts: 5, AtRiskAccounts: 2, ExhaustedAccounts: 1}) {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.ProjectionSyncedAt == nil || !result.ProjectionSyncedAt.Equal(now) {
		t.Fatalf("expected latest projection sync %s, got %v", now, result.ProjectionSyncedAt)
	}
	if result.Items[0].Status != AdminQuotaStateCritical || result.Items[1].Status != AdminQuotaStateWarning {
		t.Fatalf("expected critical then warning, got %+v", result.Items)
	}
	critical := result.Items[0]
	if critical.MostSaturatedWindow == nil || *critical.MostSaturatedWindow != Window5H || critical.MaximumPercent == nil || *critical.MaximumPercent != 100 {
		t.Fatalf("unexpected critical saturation: %+v", critical)
	}
	if len(critical.Windows) != 2 || critical.Windows[1].QuotaTokens != nil || critical.Windows[1].Percent != nil || critical.Windows[1].ResetAt != nil {
		t.Fatalf("expected an explicit unlimited 7d window: %+v", critical.Windows)
	}
	if critical.NextResetAt == nil || !critical.NextResetAt.Equal(reset5h) {
		t.Fatalf("expected finite-window reset at %s, got %v", reset5h, critical.NextResetAt)
	}

	all, err := service.AdminQuotaStates(context.Background(), AdminQuotaStateParams{
		View:        AdminQuotaStateViewAll,
		AccountType: auth.UserTypeUser,
		Search:      "HUMAN",
		Page:        2,
		PageSize:    2,
		SortBy:      "displayName",
		SortDir:     "asc",
	}, now)
	if err != nil {
		t.Fatalf("list filtered quota states: %v", err)
	}
	if all.Total != 4 || len(all.Items) != 2 || all.Items[0].DisplayName != "Unlimited human" || all.Items[1].DisplayName != "Warning human" {
		t.Fatalf("unexpected stable filtered page: %+v", all)
	}
	if all.Summary.ProjectedAccounts != 5 {
		t.Fatalf("summary must ignore list filters: %+v", all.Summary)
	}
	unassigned, err := service.AdminQuotaStates(context.Background(), AdminQuotaStateParams{
		View: AdminQuotaStateViewAll, Search: "Unassigned", PageSize: 20,
	}, now)
	if err != nil {
		t.Fatalf("list unassigned quota state: %v", err)
	}
	if len(unassigned.Items) != 1 || unassigned.Items[0].Status != AdminQuotaStateUnassigned {
		t.Fatalf("expected one unassigned projection: %+v", unassigned.Items)
	}
	for _, window := range unassigned.Items[0].Windows {
		if window.QuotaTokens != nil || window.RemainingTokens != nil || window.Percent != nil || window.ResetAt != nil {
			t.Fatalf("unassigned window must not expose stale quota details: %+v", window)
		}
	}
	beyondLastPage, err := service.AdminQuotaStates(context.Background(), AdminQuotaStateParams{
		View: AdminQuotaStateViewAll, Page: int(^uint(0) >> 1), PageSize: 100,
	}, now)
	if err != nil {
		t.Fatalf("list a page beyond the result set: %v", err)
	}
	if len(beyondLastPage.Items) != 0 || beyondLastPage.Total != 5 {
		t.Fatalf("expected an empty stable page: %+v", beyondLastPage)
	}
}

func TestAdminQuotaStatesRejectsInvalidFiltersAndSort(t *testing.T) {
	service, _ := newQuotaRiskTestService(t)
	tests := []struct {
		name   string
		params AdminQuotaStateParams
		want   error
	}{
		{name: "view", params: AdminQuotaStateParams{View: "warning"}, want: ErrInvalidQuotaStateView},
		{name: "account type", params: AdminQuotaStateParams{AccountType: auth.UserType("robot")}, want: ErrInvalidQuotaAccountType},
		{name: "sort field", params: AdminQuotaStateParams{SortBy: "quota"}, want: ErrInvalidSort},
		{name: "sort direction", params: AdminQuotaStateParams{SortDir: "sideways"}, want: ErrInvalidSort},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.AdminQuotaStates(context.Background(), test.params, time.Now())
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestAdminQuotaResetEventsUsesUTCBoundsAndGlobalPendingBacklog(t *testing.T) {
	service, db := newQuotaRiskTestService(t)
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatalf("enable sqlite foreign keys: %v", err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	start30d := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	target := createAdminQuotaResetTestUser(t, db, "10000000-0000-0000-0000-000000000001", "Primary target", auth.UserTypeUser)
	otherTarget := createAdminQuotaResetTestUser(t, db, "10000000-0000-0000-0000-000000000002", "Other target", auth.UserTypeUser)
	administrator := createAdminQuotaResetTestUser(t, db, "10000000-0000-0000-0000-000000000003", "Deleted administrator", auth.UserTypeUser)
	actorID := target.ID
	administratorID := administrator.ID
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	appliedRecent := now.Add(-24 * time.Hour)
	cutoff := now.Add(-2 * time.Hour)
	reconcileAttempt := now.Add(-90 * time.Minute)
	idempotency := "not-allowed-in-admin-output"
	events := []QuotaResetEvent{
		{
			ID: "20000000-0000-0000-0000-000000000001", UserID: target.ID,
			Type: QuotaResetEventMonthlyGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 1,
			ActorType: QuotaResetActorSystem, GrantMonth: &month, AppliedAt: &start30d, CreatedAt: start30d,
		},
		{
			ID: "20000000-0000-0000-0000-000000000002", UserID: target.ID,
			Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 2,
			ActorType: QuotaResetActorAdminSession, ActorUserID: &administratorID, IdempotencyKey: &idempotency,
			AppliedAt: &appliedRecent, CreatedAt: now.Add(-48 * time.Hour),
		},
		{
			ID: "20000000-0000-0000-0000-000000000003", UserID: target.ID,
			Type: QuotaResetEventUsed, Status: QuotaResetEventApplied, Delta: -1, BalanceAfter: 1,
			ActorType: QuotaResetActorUser, ActorUserID: &actorID, ResetCutoffAt: &cutoff,
			ReconcileAttempts: 1, LastReconcileAttemptAt: &reconcileAttempt, AppliedAt: &appliedRecent, CreatedAt: now.Add(-24 * time.Hour),
		},
		{
			ID: "20000000-0000-0000-0000-000000000004", UserID: otherTarget.ID,
			Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 1,
			ActorType: QuotaResetActorAdminAPIKey, AppliedAt: &appliedRecent, CreatedAt: now.Add(-72 * time.Hour),
		},
		{
			ID: "20000000-0000-0000-0000-000000000005", UserID: target.ID,
			Type: QuotaResetEventUsed, Status: QuotaResetEventPending, Delta: -1, BalanceAfter: 0,
			ActorType: QuotaResetActorUser, ActorUserID: &actorID, CreatedAt: start30d.Add(-time.Nanosecond),
		},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("create quota reset events: %v", err)
	}
	if err := db.Delete(&administrator).Error; err != nil {
		t.Fatalf("delete administrator: %v", err)
	}

	result, err := service.AdminQuotaResetEvents(context.Background(), AdminQuotaResetEventParams{
		Type:     QuotaResetEventAdminGrant,
		Status:   QuotaResetEventApplied,
		Search:   "primary",
		PageSize: 1000,
	}, now)
	if err != nil {
		t.Fatalf("list quota reset events: %v", err)
	}
	if result.Window != AdminQuotaResetWindow30D || result.StartsAt == nil || !result.StartsAt.Equal(start30d) || !result.EndsAt.Equal(now) {
		t.Fatalf("unexpected UTC window: %+v", result)
	}
	if result.PageSize != 100 || result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected filtered result: %+v", result)
	}
	if result.Summary != (AdminQuotaResetEventSummary{MonthlyGrants: 1, AdminGrants: 2, AppliedResetUses: 1, PendingResetUsesTotal: 1}) {
		t.Fatalf("summary must use the period but ignore table filters: %+v", result.Summary)
	}
	item := result.Items[0]
	if item.Actor.Type != QuotaResetActorAdminSession || item.Actor.UserID != nil || item.Actor.DisplayName != nil {
		t.Fatalf("expected deleted administrator projection, got %+v", item.Actor)
	}
	if item.GrantMonth != nil {
		t.Fatalf("admin grant must not expose a grant month: %v", item.GrantMonth)
	}

	all, err := service.AdminQuotaResetEvents(context.Background(), AdminQuotaResetEventParams{
		Window:   AdminQuotaResetWindowAll,
		SortBy:   "createdAt",
		SortDir:  "asc",
		PageSize: 20,
	}, now)
	if err != nil {
		t.Fatalf("list all quota reset events: %v", err)
	}
	if all.StartsAt != nil || all.Total != 5 || len(all.Items) != 5 || all.Items[0].Status != QuotaResetEventPending {
		t.Fatalf("unexpected all-time ordering: %+v", all)
	}
	if all.Items[1].GrantMonth == nil || *all.Items[1].GrantMonth != "2026-09" {
		t.Fatalf("expected YYYY-MM grant month, got %+v", all.Items[1].GrantMonth)
	}
	actorTypes := make(map[QuotaResetActorType]bool, 4)
	for _, ledgerItem := range all.Items {
		actorTypes[ledgerItem.Actor.Type] = true
	}
	for _, actorType := range []QuotaResetActorType{
		QuotaResetActorSystem,
		QuotaResetActorUser,
		QuotaResetActorAdminSession,
		QuotaResetActorAdminAPIKey,
	} {
		if !actorTypes[actorType] {
			t.Fatalf("missing %q actor projection in %+v", actorType, all.Items)
		}
	}
}

func TestAdminQuotaResetEventsRejectsInvalidFiltersAndSort(t *testing.T) {
	service, _ := newQuotaRiskTestService(t)
	tests := []struct {
		name   string
		params AdminQuotaResetEventParams
		want   error
	}{
		{name: "window", params: AdminQuotaResetEventParams{Window: "90d"}, want: ErrInvalidQuotaResetWindow},
		{name: "type", params: AdminQuotaResetEventParams{Type: QuotaResetEventType("refund")}, want: ErrInvalidQuotaResetEventType},
		{name: "status", params: AdminQuotaResetEventParams{Status: QuotaResetEventStatus("failed")}, want: ErrInvalidQuotaResetEventStatus},
		{name: "sort field", params: AdminQuotaResetEventParams{SortBy: "balance"}, want: ErrInvalidSort},
		{name: "sort direction", params: AdminQuotaResetEventParams{SortDir: "sideways"}, want: ErrInvalidSort},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.AdminQuotaResetEvents(context.Background(), test.params, time.Now())
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestAdminQuotaResetEventsSevenDayWindowStartsAtUTCDayBoundary(t *testing.T) {
	service, db := newQuotaRiskTestService(t)
	target := createAdminQuotaResetTestUser(t, db, "30000000-0000-0000-0000-000000000001", "UTC target", auth.UserTypeUser)
	parisOffset := time.FixedZone("CEST", 2*60*60)
	now := time.Date(2026, 10, 25, 12, 30, 0, 0, parisOffset)
	expectedEnd := time.Date(2026, 10, 25, 10, 30, 0, 0, time.UTC)
	expectedStart := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	events := []QuotaResetEvent{
		{
			ID: "30000000-0000-0000-0000-000000000002", UserID: target.ID,
			Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 1,
			ActorType: QuotaResetActorAdminAPIKey, AppliedAt: &expectedStart, CreatedAt: expectedStart,
		},
		{
			ID: "30000000-0000-0000-0000-000000000003", UserID: target.ID,
			Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 2,
			ActorType: QuotaResetActorAdminAPIKey, CreatedAt: expectedStart.Add(-time.Nanosecond),
		},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("create seven-day boundary events: %v", err)
	}

	result, err := service.AdminQuotaResetEvents(context.Background(), AdminQuotaResetEventParams{
		Window: AdminQuotaResetWindow7D,
	}, now)
	if err != nil {
		t.Fatalf("list seven-day quota reset events: %v", err)
	}
	if result.StartsAt == nil || !result.StartsAt.Equal(expectedStart) || !result.EndsAt.Equal(expectedEnd) {
		t.Fatalf("unexpected seven-day UTC bounds: starts=%v ends=%s", result.StartsAt, result.EndsAt)
	}
	if result.Total != 1 || result.Summary.AdminGrants != 1 || result.Items[0].ID != events[0].ID {
		t.Fatalf("expected only the inclusive-boundary event: %+v", result)
	}
}

func TestAdminQuotaResetEventsUsesStableSQLPaginationAndLiteralSearch(t *testing.T) {
	service, db := newQuotaRiskTestService(t)
	target := createAdminQuotaResetTestUser(t, db, "50000000-0000-0000-0000-000000000001", "Literal %_! target", auth.UserTypeUser)
	other := createAdminQuotaResetTestUser(t, db, "50000000-0000-0000-0000-000000000002", "Literal wildcard target", auth.UserTypeUser)
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-time.Hour)
	events := []QuotaResetEvent{
		{ID: "50000000-0000-0000-0000-000000000011", UserID: target.ID, Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 1, ActorType: QuotaResetActorAdminAPIKey, CreatedAt: createdAt},
		{ID: "50000000-0000-0000-0000-000000000012", UserID: target.ID, Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 2, ActorType: QuotaResetActorAdminAPIKey, CreatedAt: createdAt},
		{ID: "50000000-0000-0000-0000-000000000013", UserID: target.ID, Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 2, ActorType: QuotaResetActorAdminAPIKey, CreatedAt: createdAt},
		{ID: "50000000-0000-0000-0000-000000000014", UserID: other.ID, Type: QuotaResetEventAdminGrant, Status: QuotaResetEventApplied, Delta: 1, BalanceAfter: 1, ActorType: QuotaResetActorAdminAPIKey, CreatedAt: createdAt},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("create pagination events: %v", err)
	}

	first, err := service.AdminQuotaResetEvents(context.Background(), AdminQuotaResetEventParams{
		Window: AdminQuotaResetWindowAll, Search: "%_!", Page: 1, PageSize: 2,
	}, now)
	if err != nil {
		t.Fatalf("list first SQL page: %v", err)
	}
	second, err := service.AdminQuotaResetEvents(context.Background(), AdminQuotaResetEventParams{
		Window: AdminQuotaResetWindowAll, Search: "%_!", Page: 2, PageSize: 2,
	}, now)
	if err != nil {
		t.Fatalf("list second SQL page: %v", err)
	}
	if first.Total != 3 || len(first.Items) != 2 || first.Items[0].ID != events[0].ID || first.Items[1].ID != events[1].ID {
		t.Fatalf("unexpected first stable page: %+v", first)
	}
	if second.Total != 3 || len(second.Items) != 1 || second.Items[0].ID != events[2].ID {
		t.Fatalf("unexpected second stable page: %+v", second)
	}
	if first.Summary.AdminGrants != 4 {
		t.Fatalf("summary must ignore the literal search filter: %+v", first.Summary)
	}

	beyondLastPage, err := service.AdminQuotaResetEvents(context.Background(), AdminQuotaResetEventParams{
		Window: AdminQuotaResetWindowAll, Search: "%_!", Page: int(^uint(0) >> 1), PageSize: 100,
	}, now)
	if err != nil {
		t.Fatalf("list extreme event page: %v", err)
	}
	if beyondLastPage.Total != 3 || len(beyondLastPage.Items) != 0 {
		t.Fatalf("expected an empty extreme event page: %+v", beyondLastPage)
	}
}

func createAdminQuotaResetTestUser(t *testing.T, db *gorm.DB, id, name string, accountType auth.UserType) users.User {
	t.Helper()
	account := users.User{
		ID: id, Email: id + "@example.com", PreferredUsername: id, Name: name,
		Type: accountType, Role: auth.RoleUser, IsActive: true,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create quota reset test user: %v", err)
	}
	return account
}
