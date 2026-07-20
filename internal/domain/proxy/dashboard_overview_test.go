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

	seedProxyInteraction(t, countedDB, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Old prompt", "gpt-old", oldAt, 100, 200)
	seedProxyInteraction(t, countedDB, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Recent prompt", "gpt-5", recentAt, 11, 13)
	seedProxyInteraction(t, countedDB, otherUserID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Hidden prompt", "gpt-5", recentAt, 1000, 1000)
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
	if recent.Totals.Requests != 1 || recent.Totals.Prompts != 1 || recent.Totals.ToolCalls != 1 {
		t.Fatalf("unexpected recent request totals: %#v", recent.Totals)
	}
	if recent.Totals.InputTokens != 11 || recent.Totals.OutputTokens != 13 || recent.Totals.TotalTokens != 31 {
		t.Fatalf("unexpected recent token totals: %#v", recent.Totals)
	}
	if recent.TotalDurationMs != 90000 {
		t.Fatalf("unexpected recent duration: %d", recent.TotalDurationMs)
	}
	recentDay := recent.Daily[len(recent.Daily)-2]
	if recentDay.Date != "2026-01-29" || recentDay.Requests != 1 || recentDay.TotalTokens != 31 {
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
	if allTime.Totals.Requests != 2 || allTime.Totals.Prompts != 2 || allTime.Totals.ToolCalls != 1 {
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
}

// TestDashboardOverviewRejectsInvalidWindow verifies invalid windows are rejected before aggregation.
func TestDashboardOverviewRejectsInvalidWindow(t *testing.T) {
	_, service := newProxyServiceTestDB(t)

	_, err := service.DashboardOverview(context.Background(), "11111111-1111-1111-1111-111111111111", UsageWindow("14d"), time.Now())
	if !errors.Is(err, ErrInvalidUsageWindow) {
		t.Fatalf("expected invalid usage window, got %v", err)
	}
}
