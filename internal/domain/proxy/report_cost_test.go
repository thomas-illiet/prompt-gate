package proxy

import (
	"context"
	"promptgate/backend/internal/domain/pricing"
	"testing"
	"time"
)

func TestReportEmbeddingCostMatchesDashboardAndRejectsPartialHistory(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	ctx := context.Background()
	userID := "11111111-1111-1111-1111-111111111111"
	id := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	// Last day of the month catches date-vs-timestamp boundary mistakes.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	seedProxyInteraction(t, db, userID, id, "embedding", end.Add(-time.Hour), 1_000_000, 0)
	if err := db.Model(&TokenUsage{}).Where("interception_id = ?", id).Updates(map[string]any{"type": tokenUsageTypeEmbedding, "cache_read_input_tokens": 0, "cache_write_input_tokens": 0}).Error; err != nil {
		t.Fatal(err)
	}
	mustAggregateUsageKPIs(t, service)
	cost, err := service.EstimateReportCost(ctx, userID, start, end)
	if err != nil || cost == nil {
		t.Fatalf("report cost %v: %v", cost, err)
	}
	assertFloatClose(t, *cost, 0.02)
	dashboard, err := service.estimateAggregateUsageCost(ctx, currentUserDashboardScope(userID), start, end)
	if err != nil {
		t.Fatal(err)
	}
	assertFloatClose(t, *cost, dashboard.TotalUSD)
	prices := pricing.NewService(db, nil)
	if err := prices.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&pricing.UsagePrice{Scope: pricing.ScopeModel, ProviderName: "openai", Model: "embedding", InputUSDPer1MTokens: 0.07}).Error; err != nil {
		t.Fatal(err)
	}
	service.priceResolver = prices
	cost, err = service.EstimateReportCost(ctx, userID, start, end)
	if err != nil || cost == nil {
		t.Fatalf("model price: %v %v", cost, err)
	}
	assertFloatClose(t, *cost, 0.07)
	dashboard, err = service.estimateAggregateUsageCost(ctx, currentUserDashboardScope(userID), start, end)
	if err != nil {
		t.Fatal(err)
	}
	assertFloatClose(t, *cost, dashboard.TotalUSD)
	if err := db.Delete(&Interception{}, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if cost, err := service.EstimateReportCost(ctx, userID, start, end); err != nil || cost != nil {
		t.Fatalf("incomplete history presented as full cost: %v %v", cost, err)
	}
}
