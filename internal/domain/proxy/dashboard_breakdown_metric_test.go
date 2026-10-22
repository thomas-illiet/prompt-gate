package proxy

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"promptgate/backend/internal/domain/pricing"
)

// TestDashboardBreakdownRanksAllItemsByRequests verifies the request leader is
// not discarded by a token-ranked top-five preselection.
func TestDashboardBreakdownRanksAllItemsByRequests(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"

	for i := 0; i < 7; i++ {
		seedProxyInteraction(
			t,
			db,
			userID,
			fmt.Sprintf("10000000-0000-0000-0000-%012d", i+1),
			"request-leader",
			now.AddDate(0, 0, -(i+1)),
			1,
			1,
		)
	}
	for i := 0; i < 6; i++ {
		seedProxyInteraction(
			t,
			db,
			userID,
			fmt.Sprintf("20000000-0000-0000-0000-%012d", i+1),
			fmt.Sprintf("token-leader-%d", i+1),
			now.AddDate(0, 0, -(i+10)),
			int64(100+i),
			100,
		)
	}
	mustAggregateUsageKPIs(t, service)

	byTokens, err := service.DashboardTopModels(context.Background(), userID, UsageWindow30Days, now)
	if err != nil {
		t.Fatalf("token-ranked breakdown: %v", err)
	}
	if len(byTokens.Items) != 5 || byTokens.Items[0].Name == "request-leader" {
		t.Fatalf("expected compatibility token ranking, got %#v", byTokens.Items)
	}

	byRequests, err := service.DashboardTopModels(context.Background(), userID, UsageWindow30Days, now, DashboardBreakdownRequests)
	if err != nil {
		t.Fatalf("request-ranked breakdown: %v", err)
	}
	if len(byRequests.Items) != 5 || byRequests.Items[0].Name != "request-leader" || byRequests.Items[0].Requests != 7 {
		t.Fatalf("expected request leader from full item set, got %#v", byRequests.Items)
	}
}

// TestDashboardBreakdownEstimatesAllItemsBeforeCostRanking verifies a low-token
// high-price model can enter the cost top five.
func TestDashboardBreakdownEstimatesAllItemsBeforeCostRanking(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	ctx := context.Background()
	pricingService := pricing.NewService(db, nil)
	if err := pricingService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}
	if _, err := pricingService.UpdateConfig(ctx, pricing.UpdateConfigInput{
		Fallback: pricing.PriceRates{Input: 0.001, Output: 0.001},
		Models: []pricing.ModelPriceRecord{{
			ProviderName: "openai",
			Model:        "expensive",
			Input:        1000,
			Output:       1000,
		}},
	}); err != nil {
		t.Fatalf("configure pricing: %v", err)
	}
	service := NewService(db, WithPriceResolver(pricingService))
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	for i := 0; i < 6; i++ {
		seedProxyInteraction(
			t,
			db,
			userID,
			fmt.Sprintf("30000000-0000-0000-0000-%012d", i+1),
			fmt.Sprintf("cheap-%d", i+1),
			now.Add(-time.Duration(i+1)*time.Hour),
			1000,
			1000,
		)
	}
	seedProxyInteraction(t, db, userID, "40000000-0000-0000-0000-000000000001", "expensive", now.Add(-8*time.Hour), 1, 1)
	mustAggregateUsageKPIs(t, service)

	byCost, err := service.DashboardTopModels(ctx, userID, UsageWindow7Days, now, DashboardBreakdownCost)
	if err != nil {
		t.Fatalf("cost-ranked breakdown: %v", err)
	}
	if len(byCost.Items) != 5 || byCost.Items[0].Name != "expensive" || byCost.Items[0].EstimatedCost == nil {
		t.Fatalf("expected fully evaluated cost leader, got %#v", byCost.Items)
	}
}

// TestDashboardBreakdownRejectsInvalidMetric verifies service callers cannot
// silently fall back when an unsupported ranking metric is supplied.
func TestDashboardBreakdownRejectsInvalidMetric(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	_, err := service.DashboardTopModels(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		UsageWindow7Days,
		time.Now(),
		DashboardBreakdownMetric("latency"),
	)
	if !errors.Is(err, ErrInvalidDashboardMetric) {
		t.Fatalf("expected invalid dashboard metric, got %v", err)
	}
}
