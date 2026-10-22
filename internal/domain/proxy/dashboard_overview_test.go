package proxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDashboardOverviewAggregatesScopedUsage verifies the overview aggregates one user's KPIs once per window.
func TestDashboardOverviewAggregatesScopedUsage(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	counter := &usagePriceCountingLogger{Interface: logger.Default.LogMode(logger.Silent)}
	countedDB := db.Session(&gorm.Session{Logger: counter})
	service := NewService(countedDB)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	otherUserID := "22222222-2222-2222-2222-222222222222"
	oldAt := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	recentAt := now.AddDate(0, 0, -1)

	seedProxyInteraction(t, countedDB, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "gpt-old", oldAt, 100, 200)
	seedProxyInteraction(t, countedDB, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "gpt-5", recentAt, 11, 13)
	seedProxyInteraction(t, countedDB, otherUserID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "gpt-5", recentAt, 1000, 1000)
	if err := countedDB.Create(&ToolUsage{
		InterceptionID:     "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		ProviderResponseID: "response-bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Tool:               "linear.list",
		Input:              "{}",
		Metadata:           "{}",
		CreatedAt:          recentAt.Add(3 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed tool usage: %v", err)
	}
	mustAggregateUsageKPIs(t, service)

	counter.reset()
	recent, err := service.DashboardOverview(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard overview: %v", err)
	}
	if recent.Window != UsageWindow7Days || len(recent.Daily) != 7 {
		t.Fatalf("unexpected 7 day window: %#v", recent.UsageWindowMeta)
	}
	if recent.Totals.Requests != 1 || recent.Totals.ToolCalls != 1 {
		t.Fatalf("unexpected recent request totals: %#v", recent.Totals)
	}
	if recent.Totals.InputTokens != 11 || recent.Totals.OutputTokens != 13 || recent.Totals.TotalTokens != 31 {
		t.Fatalf("unexpected recent token totals: %#v", recent.Totals)
	}
	if recent.TotalDurationMs != 90000 {
		t.Fatalf("unexpected recent duration: %d", recent.TotalDurationMs)
	}
	if recent.AverageDurationMs == nil || *recent.AverageDurationMs != 90000 {
		t.Fatalf("unexpected recent average duration: %#v", recent.AverageDurationMs)
	}
	recentDay := recent.Daily[len(recent.Daily)-2]
	if recentDay.Date != "2026-01-29" || recentDay.Requests != 1 || recentDay.ToolCalls != 1 || recentDay.TotalTokens != 31 {
		t.Fatalf("unexpected recent daily bucket: %#v", recentDay)
	}
	if recent.Totals.EstimatedCost == nil || recentDay.EstimatedCost == nil {
		t.Fatalf("expected recent estimated costs: totals=%#v daily=%#v", recent.Totals.EstimatedCost, recentDay.EstimatedCost)
	}
	assertFloatClose(t, recent.Totals.EstimatedCost.TotalUSD, 0.00048)
	assertFloatClose(t, recentDay.EstimatedCost.TotalUSD, 0.00048)
	if counter.usageCostQueries != 1 {
		t.Fatalf("expected one usage cost query, got %d", counter.usageCostQueries)
	}

	counter.reset()
	allTime, err := service.DashboardOverview(context.Background(), userID, UsageWindowAll, now)
	if err != nil {
		t.Fatalf("all-time dashboard overview: %v", err)
	}
	if allTime.Window != UsageWindowAll || len(allTime.Daily) != 29 || allTime.Daily[0].Date != "2026-01-02" {
		t.Fatalf("unexpected all-time window: meta=%#v daily=%#v", allTime.UsageWindowMeta, allTime.Daily)
	}
	if allTime.Totals.Requests != 2 || allTime.Totals.ToolCalls != 1 {
		t.Fatalf("unexpected all-time request totals: %#v", allTime.Totals)
	}
	if allTime.Totals.InputTokens != 111 || allTime.Totals.OutputTokens != 213 || allTime.Totals.TotalTokens != 338 {
		t.Fatalf("unexpected all-time token totals: %#v", allTime.Totals)
	}
	if allTime.TotalDurationMs != 180000 {
		t.Fatalf("unexpected all-time duration: %d", allTime.TotalDurationMs)
	}
	if allTime.Daily[0].Requests != 1 || allTime.Daily[0].TotalTokens != 307 {
		t.Fatalf("unexpected first all-time bucket: %#v", allTime.Daily[0])
	}
	if allTime.Totals.EstimatedCost == nil || allTime.Daily[0].EstimatedCost == nil {
		t.Fatalf("expected all-time estimated costs: totals=%#v daily=%#v", allTime.Totals.EstimatedCost, allTime.Daily[0].EstimatedCost)
	}
	assertFloatClose(t, allTime.Totals.EstimatedCost.TotalUSD, 0.007015)
	assertFloatClose(t, allTime.Daily[0].EstimatedCost.TotalUSD, 0.006535)
	if counter.usageCostQueries != 1 {
		t.Fatalf("expected one all-time usage cost query, got %d", counter.usageCostQueries)
	}
}

// TestDashboardOverviewAllTimeWithoutActivity verifies empty all-time usage stays a successful empty response.
func TestDashboardOverviewAllTimeWithoutActivity(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)

	overview, err := service.DashboardOverview(context.Background(), "11111111-1111-1111-1111-111111111111", UsageWindowAll, now)
	if err != nil {
		t.Fatalf("dashboard overview: %v", err)
	}
	if overview.Window != UsageWindowAll || !overview.StartsAt.Equal(now) || !overview.EndsAt.Equal(now) {
		t.Fatalf("unexpected empty all-time window: %#v", overview.UsageWindowMeta)
	}
	if overview.Daily == nil || len(overview.Daily) != 0 {
		t.Fatalf("expected a non-nil empty daily list, got %#v", overview.Daily)
	}
	if overview.Totals.Requests != 0 || overview.Totals.TotalTokens != 0 || overview.Totals.EstimatedCost != nil || overview.TotalDurationMs != 0 {
		t.Fatalf("unexpected empty overview totals: %#v", overview)
	}
	if overview.AverageDurationMs != nil {
		t.Fatalf("expected empty average duration to be null, got %#v", overview.AverageDurationMs)
	}
}

// TestCurrentUserDashboardOverviewIncludesPreviousCalendarPeriod verifies that
// 7-day windows compare against the preceding seven full calendar days.
func TestCurrentUserDashboardOverviewIncludesPreviousCalendarPeriod(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "gpt-current", time.Date(2026, 1, 29, 9, 0, 0, 0, time.UTC), 10, 20)
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "gpt-previous", time.Date(2026, 1, 20, 9, 0, 0, 0, time.UTC), 4, 6)
	mustAggregateUsageKPIs(t, service)

	overview, err := service.CurrentUserDashboardOverview(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("current user dashboard overview: %v", err)
	}
	if overview.Totals.Requests != 1 || overview.Previous == nil || overview.Previous.Totals.Requests != 1 {
		t.Fatalf("unexpected current/previous totals: %#v", overview)
	}
	if got, want := overview.Previous.StartsAt, time.Date(2026, 1, 17, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("previous startsAt: got %s want %s", got, want)
	}
	if got, want := overview.Previous.EndsAt, time.Date(2026, 1, 23, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC); !got.Equal(want) {
		t.Fatalf("previous endsAt: got %s want %s", got, want)
	}
	if overview.CostCoverage == nil || overview.CostCoverage.Status != CostCoverageComplete {
		t.Fatalf("expected complete current cost coverage, got %#v", overview.CostCoverage)
	}
	if overview.Previous.CostCoverage == nil || overview.Previous.CostCoverage.Status != CostCoverageComplete {
		t.Fatalf("expected complete previous cost coverage, got %#v", overview.Previous.CostCoverage)
	}
}

// TestDashboardOverviewAverageUsesCompletedRequests verifies an in-flight
// request cannot lower the completed-response duration average.
func TestDashboardOverviewAverageUsesCompletedRequests(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	completedID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	inFlightID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	seedProxyInteraction(t, db, userID, completedID, "gpt-completed", now.Add(-2*time.Hour), 10, 20)
	seedProxyInteraction(t, db, userID, inFlightID, "gpt-streaming", now.Add(-time.Hour), 10, 20)
	if err := db.Model(&Interception{}).Where("id = ?", inFlightID).Update("ended_at", nil).Error; err != nil {
		t.Fatalf("mark request in flight: %v", err)
	}
	mustAggregateUsageKPIs(t, service)

	overview, err := service.CurrentUserDashboardOverview(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard overview: %v", err)
	}
	if overview.Totals.Requests != 2 || overview.TotalDurationMs != 90000 {
		t.Fatalf("unexpected duration totals: %#v", overview)
	}
	if overview.AverageDurationMs == nil || *overview.AverageDurationMs != 90000 {
		t.Fatalf("expected completed-request average 90000ms, got %#v", overview.AverageDurationMs)
	}
}

// TestCurrentUserDashboardOverviewOmitsComparisonForAllTime verifies all-time
// usage has no arbitrary comparison period.
func TestCurrentUserDashboardOverviewOmitsComparisonForAllTime(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	overview, err := service.CurrentUserDashboardOverview(context.Background(), "11111111-1111-1111-1111-111111111111", UsageWindowAll, time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("all-time overview: %v", err)
	}
	if overview.Previous != nil {
		t.Fatalf("expected no all-time comparison, got %#v", overview.Previous)
	}
	if overview.AverageDurationMs != nil {
		t.Fatalf("expected null average without requests, got %#v", overview.AverageDurationMs)
	}
}

// TestDashboardOverviewCostCoverageDistinguishesPartialAndUnavailable verifies
// retained raw usage never turns missing estimates into a displayed zero.
func TestDashboardOverviewCostCoverageDistinguishesPartialAndUnavailable(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	oldID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recentID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	seedProxyInteraction(t, db, userID, oldID, "gpt-old", time.Date(2025, 10, 1, 9, 0, 0, 0, time.UTC), 100, 200)
	seedProxyInteraction(t, db, userID, recentID, "gpt-current", time.Date(2026, 1, 29, 9, 0, 0, 0, time.UTC), 10, 20)
	mustAggregateUsageKPIs(t, service)
	if err := db.Delete(&Interception{}, "id = ?", oldID).Error; err != nil {
		t.Fatalf("remove expired raw usage: %v", err)
	}

	partial, err := service.CurrentUserDashboardOverview(context.Background(), userID, UsageWindowAll, now)
	if err != nil {
		t.Fatalf("partial overview: %v", err)
	}
	if partial.CostCoverage == nil || partial.CostCoverage.Status != CostCoveragePartial || partial.CostCoverage.StartsAt == nil {
		t.Fatalf("expected partial cost coverage, got %#v", partial.CostCoverage)
	}
	if partial.Totals.EstimatedCost == nil || partial.Totals.EstimatedCost.TotalUSD == 0 {
		t.Fatalf("expected retained partial estimate, got %#v", partial.Totals.EstimatedCost)
	}

	if err := db.Delete(&Interception{}, "id = ?", recentID).Error; err != nil {
		t.Fatalf("remove all raw usage: %v", err)
	}
	unavailable, err := service.CurrentUserDashboardOverview(context.Background(), userID, UsageWindowAll, now)
	if err != nil {
		t.Fatalf("unavailable overview: %v", err)
	}
	if unavailable.CostCoverage == nil || unavailable.CostCoverage.Status != CostCoverageUnavailable {
		t.Fatalf("expected unavailable cost coverage, got %#v", unavailable.CostCoverage)
	}
	if unavailable.Totals.EstimatedCost != nil {
		t.Fatalf("unavailable estimate must be absent, got %#v", unavailable.Totals.EstimatedCost)
	}
}

// TestDashboardOverviewDoesNotPriceRequestsWithoutUsage verifies missing token
// telemetry is not presented as an exact zero-dollar estimate.
func TestDashboardOverviewDoesNotPriceRequestsWithoutUsage(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	interceptionID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	seedProxyInteraction(t, db, userID, interceptionID, "gpt-missing-usage", now.Add(-time.Hour), 10, 20)
	if err := db.Where("interception_id = ?", interceptionID).Delete(&TokenUsage{}).Error; err != nil {
		t.Fatalf("remove token telemetry: %v", err)
	}
	mustAggregateUsageKPIs(t, service)

	overview, err := service.CurrentUserDashboardOverview(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard overview: %v", err)
	}
	if overview.Totals.Requests != 1 || overview.Totals.TotalTokens != 0 {
		t.Fatalf("unexpected request totals: %#v", overview.Totals)
	}
	if overview.CostCoverage == nil || overview.CostCoverage.Status != CostCoverageUnavailable {
		t.Fatalf("expected unavailable cost coverage, got %#v", overview.CostCoverage)
	}
	if overview.Totals.EstimatedCost != nil {
		t.Fatalf("missing token telemetry must not become a zero estimate: %#v", overview.Totals.EstimatedCost)
	}
}

// TestDashboardActivityAcceptsAnnualWindow verifies the annual heatmap is
// bounded to exactly 365 daily buckets.
func TestDashboardActivityAcceptsAnnualWindow(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "gpt-5", time.Date(2025, 1, 31, 9, 0, 0, 0, time.UTC), 1, 1)
	mustAggregateUsageKPIs(t, service)

	activity, err := service.DashboardActivity(context.Background(), userID, UsageWindow365Days, now)
	if err != nil {
		t.Fatalf("annual dashboard activity: %v", err)
	}
	if activity.Window != UsageWindow365Days || len(activity.Daily) != 365 || activity.Daily[0].Date != "2025-01-31" {
		t.Fatalf("unexpected annual activity bounds: %#v", activity)
	}
	if activity.Daily[0].Requests != 1 {
		t.Fatalf("expected first annual day activity, got %#v", activity.Daily[0])
	}
}

// TestDashboardOverviewRejectsInvalidWindow verifies invalid windows are rejected before aggregation.
func TestDashboardOverviewRejectsInvalidWindow(t *testing.T) {
	_, service := newProxyServiceTestDB(t)

	_, err := service.DashboardOverview(context.Background(), "11111111-1111-1111-1111-111111111111", UsageWindow("14d"), time.Now())
	if !errors.Is(err, ErrInvalidUsageWindow) {
		t.Fatalf("expected invalid usage window, got %v", err)
	}
}
