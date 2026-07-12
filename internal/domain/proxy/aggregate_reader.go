package proxy

import (
	"context"
	"fmt"
	"time"

	"promptgate/backend/internal/domain/auth"
)

func (s *Service) loadAggregatedUsage(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time, summary *UsageSummary, daily map[string]*DailyUsage) error {
	var rows []ProxyDailyUsageKPI
	query := s.db.WithContext(ctx).Model(&ProxyDailyUsageKPI{})
	startDay, endDay := usageDayBounds(startsAt, endsAt)
	query = scope.applyInitiatorFilter(query, "initiator_id").
		Where("day >= ? AND day <= ?", startDay, endDay)
	if err := query.Find(&rows).Error; err != nil {
		return fmt.Errorf("load aggregated usage: %w", err)
	}
	for _, row := range rows {
		accumulateKPIIntoTotals(&summary.Totals, row)
		if bucket := daily[dateKey(row.Day)]; bucket != nil {
			bucket.Requests += row.Requests
			bucket.Prompts += row.Prompts
			bucket.InputTokens += row.InputTokens
			bucket.OutputTokens += row.OutputTokens
			bucket.CompletionInputTokens += row.CompletionInputTokens
			bucket.CompletionOutputTokens += row.CompletionOutputTokens
			bucket.CompletionTokens += row.CompletionTokens
			bucket.EmbeddingTokens += row.EmbeddingTokens
			bucket.TotalTokens += row.TotalTokens
		}
	}
	return nil
}

func (s *Service) aggregatedUsageTotals(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time) (dashboardAggregateTotals, error) {
	var row struct {
		Requests               int64
		Prompts                int64
		ToolCalls              int64
		TotalDurationMs        int64
		InputTokens            int64
		OutputTokens           int64
		CacheReadInputTokens   int64
		CacheWriteInputTokens  int64
		CompletionInputTokens  int64
		CompletionOutputTokens int64
		CompletionTokens       int64
		EmbeddingTokens        int64
		TotalTokens            int64
	}
	startDay, endDay := usageDayBounds(startsAt, endsAt)
	query := s.db.WithContext(ctx).
		Table("proxy_daily_usage_kpis").
		Select(`COALESCE(SUM(requests), 0) AS requests,
			COALESCE(SUM(prompts), 0) AS prompts,
			COALESCE(SUM(tool_calls), 0) AS tool_calls,
			COALESCE(SUM(total_duration_ms), 0) AS total_duration_ms,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(cache_read_input_tokens), 0) AS cache_read_input_tokens,
			COALESCE(SUM(cache_write_input_tokens), 0) AS cache_write_input_tokens,
			COALESCE(SUM(completion_input_tokens), 0) AS completion_input_tokens,
			COALESCE(SUM(completion_output_tokens), 0) AS completion_output_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(embedding_tokens), 0) AS embedding_tokens,
			COALESCE(SUM(total_tokens), 0) AS total_tokens`)
	query = scope.applyInitiatorFilter(query, "initiator_id").
		Where("day >= ? AND day <= ?", startDay, endDay)
	if err := query.Scan(&row).Error; err != nil {
		return dashboardAggregateTotals{}, fmt.Errorf("load aggregated usage totals: %w", err)
	}
	return dashboardAggregateTotals{
		UsageTotals: UsageTotals{
			Requests:               row.Requests,
			Prompts:                row.Prompts,
			ToolCalls:              row.ToolCalls,
			InputTokens:            row.InputTokens,
			OutputTokens:           row.OutputTokens,
			CacheReadInputTokens:   row.CacheReadInputTokens,
			CacheWriteInputTokens:  row.CacheWriteInputTokens,
			CompletionInputTokens:  row.CompletionInputTokens,
			CompletionOutputTokens: row.CompletionOutputTokens,
			CompletionTokens:       row.CompletionTokens,
			EmbeddingTokens:        row.EmbeddingTokens,
			TotalTokens:            row.TotalTokens,
		},
		TotalDurationMs: row.TotalDurationMs,
	}, nil
}

func (s *Service) aggregatedBreakdowns(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time, target dashboardBreakdownTarget) (map[string]*UsageBreakdown, error) {
	var rows []aggregateBreakdownRow
	startDay, endDay := usageDayBounds(startsAt, endsAt)
	query := s.db.WithContext(ctx).
		Table("proxy_daily_usage_breakdowns").
		Select(`name,
			COALESCE(SUM(requests), 0) AS requests,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(cache_read_input_tokens), 0) AS cache_read_input_tokens,
			COALESCE(SUM(cache_write_input_tokens), 0) AS cache_write_input_tokens,
			COALESCE(SUM(completion_input_tokens), 0) AS completion_input_tokens,
			COALESCE(SUM(completion_output_tokens), 0) AS completion_output_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(embedding_tokens), 0) AS embedding_tokens,
			COALESCE(SUM(total_tokens), 0) AS total_tokens`).
		Where("dimension = ? AND day >= ? AND day <= ?", string(target), startDay, endDay).
		Group("name")
	query = scope.applyInitiatorFilter(query, "initiator_id")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load aggregated breakdowns: %w", err)
	}

	values := make(map[string]*UsageBreakdown, len(rows))
	for _, row := range rows {
		item := breakdown(values, row.Name)
		item.Requests = row.Requests
		item.TotalTokens = row.TotalTokens
		item.completionInputTokens = row.CompletionInputTokens
		item.completionOutputTokens = row.CompletionOutputTokens
		item.embeddingTokens = row.EmbeddingTokens
	}
	return values, nil
}

func (s *Service) aggregatedTopIdentities(ctx context.Context, startsAt, endsAt time.Time) (map[string]*UsageBreakdown, error) {
	var rows []topIdentityAggregateRow
	startDay, endDay := usageDayBounds(startsAt, endsAt)
	if err := s.db.WithContext(ctx).
		Table("proxy_daily_usage_kpis").
		Select(`proxy_daily_usage_kpis.initiator_id,
			COALESCE(NULLIF(users.name, ''), NULLIF(users.preferred_username, ''), NULLIF(users.email, ''), CAST(users.id AS TEXT)) AS name,
			COALESCE(SUM(proxy_daily_usage_kpis.requests), 0) AS requests,
			COALESCE(SUM(proxy_daily_usage_kpis.completion_input_tokens), 0) AS completion_input_tokens,
			COALESCE(SUM(proxy_daily_usage_kpis.completion_output_tokens), 0) AS completion_output_tokens,
			COALESCE(SUM(proxy_daily_usage_kpis.embedding_tokens), 0) AS embedding_tokens,
			COALESCE(SUM(proxy_daily_usage_kpis.total_tokens), 0) AS total_tokens`).
		Joins("JOIN users ON users.id = proxy_daily_usage_kpis.initiator_id").
		Where("proxy_daily_usage_kpis.day >= ? AND proxy_daily_usage_kpis.day <= ?", startDay, endDay).
		Group("proxy_daily_usage_kpis.initiator_id, users.id, users.name, users.preferred_username, users.email").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load dashboard top identities: %w", err)
	}

	values := make(map[string]*UsageBreakdown, len(rows))
	for _, row := range rows {
		item := breakdownByKey(values, row.InitiatorID, row.Name)
		item.Requests = row.Requests
		item.TotalTokens = row.TotalTokens
		item.completionInputTokens = row.CompletionInputTokens
		item.completionOutputTokens = row.CompletionOutputTokens
		item.embeddingTokens = row.EmbeddingTokens
	}
	return values, nil
}

func (s *Service) countActiveIdentitiesFromKPIs(ctx context.Context, userType auth.UserType, startsAt, endsAt time.Time) (int64, error) {
	var total int64
	startDay, endDay := usageDayBounds(startsAt, endsAt)
	if err := s.db.WithContext(ctx).
		Table("proxy_daily_usage_kpis").
		Joins("JOIN users ON users.id = proxy_daily_usage_kpis.initiator_id").
		Where("users.type = ? AND proxy_daily_usage_kpis.day >= ? AND proxy_daily_usage_kpis.day <= ? AND proxy_daily_usage_kpis.requests > 0", userType, startDay, endDay).
		Distinct("proxy_daily_usage_kpis.initiator_id").
		Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count active identities: %w", err)
	}
	return total, nil
}
