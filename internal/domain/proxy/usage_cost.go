package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *Service) attachEstimatedCosts(ctx context.Context, scope dashboardUsageScope, totals *UsageTotals, daily []DailyUsage) error {
	if !s.usageCost.Enabled {
		return nil
	}
	if len(daily) == 0 {
		return nil
	}
	rates, err := s.usageCostRateBook(ctx)
	if err != nil {
		return err
	}
	return s.attachEstimatedCostsWithRates(ctx, rates, scope, totals, daily)
}

func (s *Service) attachEstimatedCostsWithRates(ctx context.Context, rates usageCostRateBook, scope dashboardUsageScope, totals *UsageTotals, daily []DailyUsage) error {
	if !s.usageCost.Enabled {
		return nil
	}
	if len(daily) == 0 {
		return nil
	}
	startsAt, err := time.Parse("2006-01-02", daily[0].Date)
	if err != nil {
		return err
	}
	endsDay, err := time.Parse("2006-01-02", daily[len(daily)-1].Date)
	if err != nil {
		return err
	}
	endsAt := endsDay.Add(24*time.Hour - time.Nanosecond)
	costsByDate, totalCost, err := s.estimateDailyUsageCosts(ctx, rates, scope, startsAt, endsAt)
	if err != nil {
		return err
	}
	totals.EstimatedCost = totalCost
	for i := range daily {
		daily[i].EstimatedCost = costsByDate[daily[i].Date]
	}
	return nil
}

// attachBreakdownEstimatedCosts adds optional cost estimates to token breakdowns.
func (s *Service) attachBreakdownEstimatedCosts(ctx context.Context, rates usageCostRateBook, scope dashboardUsageScope, target dashboardBreakdownTarget, items []UsageBreakdown, startsAt, endsAt time.Time) error {
	if !s.usageCost.Enabled || len(items) == 0 {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	costs, err := s.estimateBreakdownUsageCosts(ctx, rates, scope, target, names, startsAt, endsAt)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].EstimatedCost = costs[items[i].Name]
	}
	return nil
}

func (s *Service) attachIdentityEstimatedCosts(ctx context.Context, rates usageCostRateBook, items []UsageBreakdown, startsAt, endsAt time.Time) error {
	if !s.usageCost.Enabled || len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.key)
	}
	costs, err := s.estimateGroupedUsageCosts(ctx, rates, globalDashboardScope(), startsAt, endsAt, "interceptions.initiator_id", ids)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].EstimatedCost = costs[items[i].key]
	}
	return nil
}

// estimateUsageCost converts token buckets into an optional USD cost estimate.
func (s *Service) estimateUsageCost(rates usageCostRateBook, providerName, model string, completionInputTokens, completionOutputTokens, embeddingTokens int64) *EstimatedCost {
	if !s.usageCost.Enabled {
		return nil
	}

	resolvedRates := rates.ratesFor(providerName, model)
	inputTokens := completionInputTokens
	if rates.priceEmbeddingAsInput {
		inputTokens += embeddingTokens
	}
	inputCost := usageTokenCost(inputTokens, resolvedRates.InputUSDPer1MTokens)
	outputCost := usageTokenCost(completionOutputTokens, resolvedRates.OutputUSDPer1MTokens)
	embeddingCost := usageTokenCost(embeddingTokens, resolvedRates.EmbeddingUSDPer1MTokens)
	return &EstimatedCost{
		InputUSD:     inputCost,
		OutputUSD:    outputCost,
		EmbeddingUSD: embeddingCost,
		TotalUSD:     inputCost + outputCost + embeddingCost,
		Rates:        resolvedRates,
	}
}

func (s *Service) estimateAggregateUsageCost(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time) (*EstimatedCost, error) {
	if !s.usageCost.Enabled {
		return nil, nil
	}
	rates, err := s.usageCostRateBook(ctx)
	if err != nil {
		return nil, err
	}
	return s.estimateAggregateUsageCostWithRates(ctx, rates, scope, startsAt, endsAt)
}

func (s *Service) estimateAggregateUsageCostWithRates(ctx context.Context, rates usageCostRateBook, scope dashboardUsageScope, startsAt, endsAt time.Time) (*EstimatedCost, error) {
	_, total, err := s.estimateDailyUsageCosts(ctx, rates, scope, startsAt, endsAt)
	return total, err
}

func (s *Service) estimateBreakdownUsageCosts(ctx context.Context, rates usageCostRateBook, scope dashboardUsageScope, target dashboardBreakdownTarget, names []string, startsAt, endsAt time.Time) (map[string]*EstimatedCost, error) {
	column := ""
	switch target {
	case dashboardBreakdownModels:
		column = "interceptions.model"
	case dashboardBreakdownProviderNames:
		column = "interceptions.provider"
	case dashboardBreakdownProviderTypes:
		column = "interceptions.provider_type"
	default:
		return nil, ErrInvalidSort
	}
	return s.estimateGroupedUsageCosts(ctx, rates, scope, startsAt, endsAt, column, names)
}

func (s *Service) estimateDailyUsageCosts(ctx context.Context, rates usageCostRateBook, scope dashboardUsageScope, startsAt, endsAt time.Time) (map[string]*EstimatedCost, *EstimatedCost, error) {
	return s.estimateUsageCosts(ctx, rates, scope, startsAt, endsAt)
}

func (s *Service) estimateUsageCosts(ctx context.Context, rates usageCostRateBook, scope dashboardUsageScope, startsAt, endsAt time.Time) (map[string]*EstimatedCost, *EstimatedCost, error) {
	if !s.usageCost.Enabled {
		return map[string]*EstimatedCost{}, nil, nil
	}
	type usageCostAggregateRow struct {
		Day                   string
		Provider              string
		Model                 string
		Type                  string
		InputTokens           int64
		OutputTokens          int64
		CacheReadInputTokens  int64
		CacheWriteInputTokens int64
	}
	dateExpr := s.usageCostDateExpression("token_usages.created_at")
	var rows []usageCostAggregateRow
	query := s.db.WithContext(ctx).
		Table("token_usages").
		Select(fmt.Sprintf(`%s AS day,
			interceptions.provider,
			interceptions.model,
			token_usages.type,
			COALESCE(SUM(token_usages.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(token_usages.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(token_usages.cache_read_input_tokens), 0) AS cache_read_input_tokens,
			COALESCE(SUM(token_usages.cache_write_input_tokens), 0) AS cache_write_input_tokens`, dateExpr)).
		Joins("JOIN interceptions ON interceptions.id = token_usages.interception_id")
	query = scope.applyInitiatorFilter(query, "interceptions.initiator_id").
		Where("token_usages.created_at >= ? AND token_usages.created_at <= ?", startsAt, endsAt).
		Group(dateExpr + ", interceptions.provider, interceptions.model, token_usages.type")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("load usage cost rows: %w", err)
	}

	byDate := map[string]*EstimatedCost{}
	total := &EstimatedCost{Rates: rates.fallback}
	for _, row := range rows {
		tokenRow := tokenUsageRow{
			InputTokens:           row.InputTokens,
			OutputTokens:          row.OutputTokens,
			CacheReadInputTokens:  row.CacheReadInputTokens,
			CacheWriteInputTokens: row.CacheWriteInputTokens,
			Type:                  row.Type,
		}
		inputTokens := completionInputTokens(tokenRow)
		outputTokens := row.OutputTokens
		embeddingTokens := int64(0)
		if isEmbeddingTokenUsage(row.Type, "") {
			inputTokens = 0
			outputTokens = 0
			embeddingTokens = tokenUsageTotal(tokenRow)
		}
		cost := s.estimateUsageCost(rates, row.Provider, row.Model, inputTokens, outputTokens, embeddingTokens)
		if cost == nil {
			continue
		}
		key := row.Day
		bucket := byDate[key]
		if bucket == nil {
			bucket = &EstimatedCost{Rates: cost.Rates}
			byDate[key] = bucket
		}
		addEstimatedCost(bucket, cost)
		addEstimatedCost(total, cost)
	}
	return byDate, total, nil
}

func (s *Service) estimateGroupedUsageCosts(ctx context.Context, rates usageCostRateBook, scope dashboardUsageScope, startsAt, endsAt time.Time, groupColumn string, groupValues []string) (map[string]*EstimatedCost, error) {
	if !s.usageCost.Enabled || len(groupValues) == 0 {
		return map[string]*EstimatedCost{}, nil
	}
	type usageCostAggregateRow struct {
		CostGroup             string
		Provider              string
		Model                 string
		Type                  string
		InputTokens           int64
		OutputTokens          int64
		CacheReadInputTokens  int64
		CacheWriteInputTokens int64
	}
	var rows []usageCostAggregateRow
	query := s.db.WithContext(ctx).
		Table("token_usages").
		Select(fmt.Sprintf(`%s AS cost_group,
			interceptions.provider,
			interceptions.model,
			token_usages.type,
			COALESCE(SUM(token_usages.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(token_usages.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(token_usages.cache_read_input_tokens), 0) AS cache_read_input_tokens,
			COALESCE(SUM(token_usages.cache_write_input_tokens), 0) AS cache_write_input_tokens`, groupColumn)).
		Joins("JOIN interceptions ON interceptions.id = token_usages.interception_id")
	query = scope.applyInitiatorFilter(query, "interceptions.initiator_id").
		Where("token_usages.created_at >= ? AND token_usages.created_at <= ?", startsAt, endsAt).
		Where(groupColumn+" IN ?", groupValues).
		Group(groupColumn + ", interceptions.provider, interceptions.model, token_usages.type")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load grouped usage cost rows: %w", err)
	}

	out := map[string]*EstimatedCost{}
	for _, row := range rows {
		tokenRow := tokenUsageRow{
			InputTokens:           row.InputTokens,
			OutputTokens:          row.OutputTokens,
			CacheReadInputTokens:  row.CacheReadInputTokens,
			CacheWriteInputTokens: row.CacheWriteInputTokens,
			Type:                  row.Type,
		}
		inputTokens := completionInputTokens(tokenRow)
		outputTokens := row.OutputTokens
		embeddingTokens := int64(0)
		if isEmbeddingTokenUsage(row.Type, "") {
			inputTokens = 0
			outputTokens = 0
			embeddingTokens = tokenUsageTotal(tokenRow)
		}
		cost := s.estimateUsageCost(rates, row.Provider, row.Model, inputTokens, outputTokens, embeddingTokens)
		if cost == nil {
			continue
		}
		bucket := out[row.CostGroup]
		if bucket == nil {
			bucket = &EstimatedCost{Rates: cost.Rates}
			out[row.CostGroup] = bucket
		}
		addEstimatedCost(bucket, cost)
	}
	return out, nil
}

func (s *Service) usageCostRateBook(ctx context.Context) (usageCostRateBook, error) {
	rates := usageCostRateBook{fallback: s.usageCost.Rates}
	if s.priceResolver == nil {
		return rates, nil
	}
	config, err := s.priceResolver.Config(ctx)
	if err != nil {
		return usageCostRateBook{}, err
	}
	rates.fallback = CostRates{
		InputUSDPer1MTokens:  config.Fallback.Input,
		OutputUSDPer1MTokens: config.Fallback.Output,
	}
	rates.models = make(map[string]CostRates, len(config.Models))
	rates.priceEmbeddingAsInput = true
	for _, model := range config.Models {
		rates.models[modelPriceRateKey(model.ProviderName, model.Model)] = CostRates{
			InputUSDPer1MTokens:  model.Input,
			OutputUSDPer1MTokens: model.Output,
		}
	}
	return rates, nil
}

func (rates usageCostRateBook) ratesFor(providerName, model string) CostRates {
	if rates.models != nil {
		if modelRates, ok := rates.models[modelPriceRateKey(providerName, model)]; ok {
			return modelRates
		}
	}
	return rates.fallback
}

func modelPriceRateKey(providerName, model string) string {
	return strings.TrimSpace(providerName) + "\x00" + strings.TrimSpace(model)
}

func (s *Service) usageCostDateExpression(column string) string {
	if s.db.Dialector.Name() == "postgres" {
		return "TO_CHAR(" + column + " AT TIME ZONE 'UTC', 'YYYY-MM-DD')"
	}
	return "date(" + column + ")"
}

func addEstimatedCost(dst, src *EstimatedCost) {
	dst.InputUSD += src.InputUSD
	dst.OutputUSD += src.OutputUSD
	dst.EmbeddingUSD += src.EmbeddingUSD
	dst.TotalUSD += src.TotalUSD
}

// usageTokenCost prices tokens using a USD per 1M token rate.
func usageTokenCost(tokens int64, rate float64) float64 {
	return float64(tokens) * rate / usageCostTokenUnit
}

// tokenUsageTotal returns all counted token fields for a usage row.
func tokenUsageTotal(row tokenUsageRow) int64 {
	return row.InputTokens + row.OutputTokens + row.CacheReadInputTokens + row.CacheWriteInputTokens
}

// completionInputTokens returns completion input tokens including cache token fields.
func completionInputTokens(row tokenUsageRow) int64 {
	return row.InputTokens + row.CacheReadInputTokens + row.CacheWriteInputTokens
}

// isEmbeddingTokenUsage reports whether a token usage row belongs to an embeddings request.
func isEmbeddingTokenUsage(tokenType, metadata string) bool {
	if tokenType == tokenUsageTypeEmbedding {
		return true
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(metadata), &values); err != nil {
		return false
	}
	metadataType, _ := values["type"].(string)
	if metadataType == tokenUsageTypeEmbedding {
		return true
	}
	metadataEndpoint, _ := values["endpoint"].(string)
	return metadataEndpoint == tokenUsageEndpointEmbeddings
}
