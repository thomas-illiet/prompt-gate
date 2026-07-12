package proxy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"promptgate/backend/internal/domain/pricing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDashboardTokensDifferentiatesCompletionAndEmbeddingTokens verifies dashboard tokens differentiates completion and embedding tokens.
func TestDashboardTokensDifferentiatesCompletionAndEmbeddingTokens(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	startedAt := now.Add(-time.Hour)

	for _, record := range []Interception{
		{
			ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			InitiatorID:  userID,
			Provider:     "openai-main",
			ProviderType: "openai",
			Model:        "gpt-5",
			StartedAt:    startedAt,
			Metadata:     "{}",
		},
		{
			ID:           "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			InitiatorID:  userID,
			Provider:     "openai-main",
			ProviderType: "openai",
			Model:        "text-embedding-3-small",
			StartedAt:    startedAt,
			Metadata:     "{}",
		},
	} {
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("seed interception: %v", err)
		}
	}
	if err := db.Create(&[]TokenUsage{
		{
			InterceptionID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			ProviderResponseID:    "completion-response",
			InputTokens:           5,
			OutputTokens:          6,
			CacheReadInputTokens:  1,
			CacheWriteInputTokens: 2,
			Metadata:              "{}",
			CreatedAt:             startedAt,
		},
		{
			InterceptionID:     "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			ProviderResponseID: "embedding-response",
			InputTokens:        17,
			Type:               tokenUsageTypeEmbedding,
			Metadata:           `{"type":"embedding"}`,
			CreatedAt:          startedAt,
		},
	}).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}
	mustAggregateUsageKPIs(t, service)

	tokens, err := service.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.CompletionTokens != 14 || tokens.EmbeddingTokens != 17 || tokens.TotalTokens != 31 {
		t.Fatalf("unexpected dashboard token totals: %#v", tokens)
	}
	if tokens.CompletionInputTokens != 8 || tokens.CompletionOutputTokens != 6 {
		t.Fatalf("unexpected dashboard completion split: %#v", tokens)
	}
	if tokens.EstimatedCost == nil {
		t.Fatal("expected dashboard estimated cost")
	}
	assertFloatClose(t, tokens.EstimatedCost.InputUSD, 0.00004)
	assertFloatClose(t, tokens.EstimatedCost.OutputUSD, 0.00018)
	assertFloatClose(t, tokens.EstimatedCost.EmbeddingUSD, 0.00000034)
	assertFloatClose(t, tokens.EstimatedCost.TotalUSD, 0.00022034)
	if tokens.EstimatedCost.Rates.InputUSDPer1MTokens != 5 || tokens.EstimatedCost.Rates.OutputUSDPer1MTokens != 30 || tokens.EstimatedCost.Rates.EmbeddingUSDPer1MTokens != 0.02 {
		t.Fatalf("unexpected default cost rates: %#v", tokens.EstimatedCost.Rates)
	}

	summary, err := service.UsageSummary(context.Background(), userID, 7, now)
	if err != nil {
		t.Fatalf("usage summary: %v", err)
	}
	if summary.Totals.CompletionTokens != 14 || summary.Totals.EmbeddingTokens != 17 || summary.Totals.TotalTokens != 31 {
		t.Fatalf("unexpected summary token totals: %#v", summary.Totals)
	}
	if summary.Totals.CompletionInputTokens != 8 || summary.Totals.CompletionOutputTokens != 6 {
		t.Fatalf("unexpected summary completion split: %#v", summary.Totals)
	}
	if summary.Totals.EstimatedCost == nil {
		t.Fatal("expected summary estimated cost")
	}

	activity, err := service.DashboardActivity(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard activity: %v", err)
	}
	latest := activity.Daily[len(activity.Daily)-1]
	if latest.CompletionTokens != 14 || latest.EmbeddingTokens != 17 || latest.TotalTokens != 31 {
		t.Fatalf("unexpected daily token totals: %#v", latest)
	}
	if latest.CompletionInputTokens != 8 || latest.CompletionOutputTokens != 6 {
		t.Fatalf("unexpected daily completion split: %#v", latest)
	}
	if latest.EstimatedCost == nil {
		t.Fatal("expected daily estimated cost")
	}
	assertFloatClose(t, latest.EstimatedCost.TotalUSD, 0.00022034)

	topModels, err := service.DashboardTopModels(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard top models: %v", err)
	}
	var embeddingModel *UsageBreakdown
	for i := range topModels.Items {
		if topModels.Items[i].Name == "text-embedding-3-small" {
			embeddingModel = &topModels.Items[i]
			break
		}
	}
	if embeddingModel == nil || embeddingModel.EstimatedCost == nil {
		t.Fatalf("expected embedding model estimated cost: %#v", topModels.Items)
	}
	assertFloatClose(t, embeddingModel.EstimatedCost.EmbeddingUSD, 0.00000034)
}

// TestUsageCostEstimatesCanBeOverriddenAndDisabled verifies usage cost estimates can be overridden and disabled.
func TestUsageCostEstimatesCanBeOverriddenAndDisabled(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	service := NewService(db, WithUsageCost(UsageCostConfig{
		Enabled: true,
		Rates: CostRates{
			InputUSDPer1MTokens:     100,
			OutputUSDPer1MTokens:    200,
			EmbeddingUSDPer1MTokens: 300,
		},
	}))
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	startedAt := now.Add(-time.Hour)

	if err := db.Create(&Interception{
		ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		InitiatorID:  userID,
		Provider:     "openai-main",
		ProviderType: "openai",
		Model:        "gpt-5",
		StartedAt:    startedAt,
		Metadata:     "{}",
	}).Error; err != nil {
		t.Fatalf("seed interception: %v", err)
	}
	if err := db.Create(&TokenUsage{
		InterceptionID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		ProviderResponseID:    "completion-response",
		InputTokens:           5,
		OutputTokens:          6,
		CacheReadInputTokens:  1,
		CacheWriteInputTokens: 2,
		Metadata:              "{}",
		CreatedAt:             startedAt,
	}).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}
	mustAggregateUsageKPIs(t, service)

	tokens, err := service.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.EstimatedCost == nil {
		t.Fatal("expected overridden estimated cost")
	}
	assertFloatClose(t, tokens.EstimatedCost.InputUSD, 0.0008)
	assertFloatClose(t, tokens.EstimatedCost.OutputUSD, 0.0012)
	assertFloatClose(t, tokens.EstimatedCost.TotalUSD, 0.002)

	disabled := NewService(db, WithUsageCost(UsageCostConfig{Enabled: false}))
	disabledTokens, err := disabled.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens disabled: %v", err)
	}
	if disabledTokens.EstimatedCost != nil {
		t.Fatalf("expected no dashboard estimated cost when disabled, got %#v", disabledTokens.EstimatedCost)
	}
	disabledSummary, err := disabled.UsageSummary(context.Background(), userID, 7, now)
	if err != nil {
		t.Fatalf("usage summary disabled: %v", err)
	}
	if disabledSummary.Totals.EstimatedCost != nil {
		t.Fatalf("expected no summary estimated cost when disabled, got %#v", disabledSummary.Totals.EstimatedCost)
	}
	disabledActivity, err := disabled.DashboardActivity(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard activity disabled: %v", err)
	}
	for _, daily := range disabledActivity.Daily {
		if daily.EstimatedCost != nil {
			t.Fatalf("expected no daily estimated cost when disabled, got %#v", daily.EstimatedCost)
		}
	}
}

// TestDashboardCostEstimatesUseConfiguredModelPrices verifies dashboard costs use one loaded pricing config.
func TestDashboardCostEstimatesUseConfiguredModelPrices(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	ctx := context.Background()
	pricingService := pricing.NewService(db, nil)
	if err := pricingService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}
	if _, err := pricingService.UpdateConfig(ctx, pricing.UpdateConfigInput{
		Fallback: pricing.PriceRates{Input: 1, Output: 2},
		Models: []pricing.ModelPriceRecord{{
			ProviderName: "openai-main",
			Model:        "gpt-5",
			Input:        10,
			Output:       20,
		}},
	}); err != nil {
		t.Fatalf("configure pricing: %v", err)
	}
	service := NewService(db, WithPriceResolver(pricingService))
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	otherUserID := "22222222-2222-2222-2222-222222222222"
	at := now.AddDate(0, 0, -1)

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Priced model", "gpt-5", at, 100, 50)
	setInterceptionProvider(t, db, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "openai-main", "openai")
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Fallback model", "claude-4", at, 200, 25)
	setInterceptionProvider(t, db, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "anthropic-main", "anthropic")
	seedProxyInteraction(t, db, userID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Shared model fallback provider", "gpt-5", at, 20, 10)
	setInterceptionProvider(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", "anthropic-main", "anthropic")
	seedProxyInteraction(t, db, otherUserID, "dddddddd-dddd-dddd-dddd-dddddddddddd", "Other user", "gpt-5", at, 10, 10)
	setInterceptionProvider(t, db, "dddddddd-dddd-dddd-dddd-dddddddddddd", "openai-main", "openai")
	mustAggregateUsageKPIs(t, service)

	tokens, err := service.DashboardTokens(ctx, userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.EstimatedCost == nil {
		t.Fatal("expected dashboard estimated cost")
	}
	assertFloatClose(t, tokens.EstimatedCost.InputUSD, 0.001304)
	assertFloatClose(t, tokens.EstimatedCost.OutputUSD, 0.00107)
	assertFloatClose(t, tokens.EstimatedCost.TotalUSD, 0.002374)

	activity, err := service.DashboardActivity(ctx, userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard activity: %v", err)
	}
	latest := activity.Daily[len(activity.Daily)-2]
	if latest.EstimatedCost == nil {
		t.Fatal("expected daily estimated cost")
	}
	assertFloatClose(t, latest.EstimatedCost.TotalUSD, 0.002374)

	topModels, err := service.DashboardTopModels(ctx, userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard top models: %v", err)
	}
	if len(topModels.Items) != 2 || topModels.Items[0].Name != "claude-4" || topModels.Items[1].Name != "gpt-5" {
		t.Fatalf("unexpected top models: %#v", topModels.Items)
	}
	if topModels.Items[0].EstimatedCost == nil || topModels.Items[1].EstimatedCost == nil {
		t.Fatalf("expected top model costs: %#v", topModels.Items)
	}
	assertFloatClose(t, topModels.Items[0].EstimatedCost.TotalUSD, 0.000257)
	assertFloatClose(t, topModels.Items[1].EstimatedCost.TotalUSD, 0.002117)

	topIdentities, err := service.AdminDashboardTopIdentities(ctx, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("admin dashboard top identities: %v", err)
	}
	if len(topIdentities.Items) != 2 || topIdentities.Items[0].Name != "One" {
		t.Fatalf("unexpected top identities: %#v", topIdentities.Items)
	}
	if topIdentities.Items[0].EstimatedCost == nil {
		t.Fatalf("expected top identity estimated cost: %#v", topIdentities.Items)
	}
	assertFloatClose(t, topIdentities.Items[0].EstimatedCost.TotalUSD, 0.002374)
}

// TestDashboardCostPricingQueriesAreNotPerUsageRow verifies pricing lookups stay constant.
func TestDashboardCostPricingQueriesAreNotPerUsageRow(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	counter := &usagePriceCountingLogger{Interface: logger.Default.LogMode(logger.Silent)}
	countedDB := db.Session(&gorm.Session{Logger: counter})
	ctx := context.Background()
	pricingService := pricing.NewService(countedDB, nil)
	if err := pricingService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate pricing: %v", err)
	}
	if _, err := pricingService.UpdateConfig(ctx, pricing.UpdateConfigInput{
		Fallback: pricing.PriceRates{Input: 1, Output: 2},
		Models: []pricing.ModelPriceRecord{{
			ProviderName: "openai-main",
			Model:        "gpt-5",
			Input:        10,
			Output:       20,
		}},
	}); err != nil {
		t.Fatalf("configure pricing: %v", err)
	}
	service := NewService(countedDB, WithPriceResolver(pricingService))
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("aaaaaaaa-aaaa-aaaa-aaaa-%012d", i)
		seedProxyInteraction(t, countedDB, userID, id, "", "gpt-5", now.Add(-time.Duration(i+1)*time.Minute), 10, 5)
		setInterceptionProvider(t, countedDB, id, "openai-main", "openai")
		if err := countedDB.Model(&TokenUsage{}).
			Where("interception_id = ?", id).
			Update("metadata", fmt.Sprintf(`{"request":"%d"}`, i)).Error; err != nil {
			t.Fatalf("seed unique token metadata: %v", err)
		}
	}
	mustAggregateUsageKPIs(t, service)
	counter.reset()

	tokens, err := service.DashboardTokens(ctx, userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.EstimatedCost == nil {
		t.Fatal("expected dashboard estimated cost")
	}
	if counter.pricingQueries > 2 {
		t.Fatalf("expected pricing queries to stay constant, got %d", counter.pricingQueries)
	}
	if counter.usageCostQueries != 1 {
		t.Fatalf("expected one token usage cost query, got %d", counter.usageCostQueries)
	}
	if counter.usageCostMaxRows > 1 {
		t.Fatalf("expected metadata-independent cost grouping, got %d rows", counter.usageCostMaxRows)
	}
}
