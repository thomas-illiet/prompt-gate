package proxy

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDashboardTopToolsUsesDurableScopedCallCounts(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userOne := "11111111-1111-1111-1111-111111111111"
	userTwo := "22222222-2222-2222-2222-222222222222"
	firstInterception := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	secondInterception := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	seedProxyInteraction(t, db, userOne, firstInterception, "gpt-5", now.Add(-time.Hour), 0, 0)
	seedProxyInteraction(t, db, userTwo, secondInterception, "claude", now.Add(-2*time.Hour), 0, 0)

	seedToolCalls := func(interceptionID string, names ...string) {
		t.Helper()
		for index, name := range names {
			if err := db.Create(&ToolUsage{
				InterceptionID:     interceptionID,
				ProviderResponseID: fmt.Sprintf("response-%s-%d", interceptionID, index),
				Tool:               name,
				Input:              "{}",
				Metadata:           "{}",
				CreatedAt:          now.Add(-30 * time.Minute),
			}).Error; err != nil {
				t.Fatalf("seed tool usage: %v", err)
			}
		}
	}
	seedToolCalls(firstInterception,
		"linear.list", "linear.list", "linear.list",
		"github.search", "github.search", "github.search",
		"   ", "",
		"calendar.list", "docs.read", "mail.search", "weather.lookup",
	)
	seedToolCalls(secondInterception,
		"linear.list", "linear.list", "linear.list", "linear.list",
		"github.search",
	)
	mustAggregateUsageKPIs(t, service)

	personal, err := service.DashboardTopTools(context.Background(), userOne, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("load personal top tools: %v", err)
	}
	if personal.TotalCalls != 12 || personal.AttributedCalls != 12 {
		t.Fatalf("unexpected personal coverage: %#v", personal)
	}
	if len(personal.Items) != 5 {
		t.Fatalf("expected top five tools, got %#v", personal.Items)
	}
	wantNames := []string{"github.search", "linear.list", "unknown", "calendar.list", "docs.read"}
	wantCalls := []int64{3, 3, 2, 1, 1}
	for index := range wantNames {
		if personal.Items[index].Name != wantNames[index] || personal.Items[index].Calls != wantCalls[index] {
			t.Fatalf("personal item %d: got %#v want %s/%d", index, personal.Items[index], wantNames[index], wantCalls[index])
		}
	}

	global, err := service.AdminDashboardTopTools(context.Background(), UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("load global top tools: %v", err)
	}
	if global.TotalCalls != 17 || global.AttributedCalls != 17 || len(global.Items) != 5 {
		t.Fatalf("unexpected global response: %#v", global)
	}
	if global.Items[0].Name != "linear.list" || global.Items[0].Calls != 7 {
		t.Fatalf("unexpected global leader: %#v", global.Items)
	}

	if err := db.Where("id IS NOT NULL").Delete(&ToolUsage{}).Error; err != nil {
		t.Fatalf("delete retained raw tool usage: %v", err)
	}
	durable, err := service.DashboardTopTools(context.Background(), userOne, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("load durable top tools: %v", err)
	}
	if len(durable.Items) != 5 || durable.Items[0] != personal.Items[0] {
		t.Fatalf("tool ranking changed after raw retention cleanup: %#v", durable.Items)
	}
}

func TestDashboardTopToolsReportsIncompleteHistoricalAttribution(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	day := dayStart(now)
	createdAt := now.Add(-time.Hour)

	if err := db.Create(&ProxyDailyUsageKPI{
		Day:         day,
		InitiatorID: userID,
		ToolCalls:   9,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}).Error; err != nil {
		t.Fatalf("seed tool total: %v", err)
	}
	if err := db.Create(&ProxyDailyUsageBreakdown{
		Day:         day,
		InitiatorID: userID,
		Dimension:   string(dashboardBreakdownTools),
		Name:        "newly.tracked",
		Requests:    4,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}).Error; err != nil {
		t.Fatalf("seed attributed tool calls: %v", err)
	}

	response, err := service.DashboardTopTools(context.Background(), userID, UsageWindowAll, now)
	if err != nil {
		t.Fatalf("load partial top tools: %v", err)
	}
	if response.TotalCalls != 9 || response.AttributedCalls != 4 {
		t.Fatalf("expected explicit partial attribution, got %#v", response)
	}
	if len(response.Items) != 1 || response.Items[0].Name != "newly.tracked" || response.Items[0].Calls != 4 {
		t.Fatalf("unexpected items: %#v", response.Items)
	}
}
