package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUsageEventDependencyMissing = errors.New("usage event dependency missing")

type dashboardAggregateTotals struct {
	UsageTotals
	TotalDurationMs int64
}

type aggregateBreakdownRow struct {
	Name                   string
	Requests               int64
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

type topIdentityAggregateRow struct {
	InitiatorID            string
	Name                   string
	Requests               int64
	CompletionInputTokens  int64
	CompletionOutputTokens int64
	EmbeddingTokens        int64
	TotalTokens            int64
}

// StartRawUsageCleanup periodically deletes raw proxy usage rows older than retention.
func (s *Service) StartRawUsageCleanup(ctx context.Context, retention, interval time.Duration) {
	if retention <= 0 || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanupRawUsageLog(context.Background(), retention)
			}
		}
	}()
}

// DeleteRawUsageBefore deletes raw usage roots older than cutoff and relies on FK cascade for child rows.
func (s *Service) DeleteRawUsageBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Where("started_at < ?", cutoff.UTC()).Delete(&Interception{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete raw proxy usage: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (s *Service) cleanupRawUsageLog(ctx context.Context, retention time.Duration) {
	cutoff := time.Now().UTC().Add(-retention)
	count, err := s.DeleteRawUsageBefore(ctx, cutoff)
	if err != nil {
		slog.Error("failed to cleanup raw proxy usage", "error", err)
		return
	}
	if count > 0 {
		slog.Info("cleaned up raw proxy usage", "interceptions", count, "cutoff", cutoff)
	}
}

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

func aggregateInterceptionStarted(tx *gorm.DB, interception Interception) error {
	day := dayStart(interception.StartedAt)
	if err := upsertDailyUsageKPI(tx, ProxyDailyUsageKPI{
		Day:         day,
		InitiatorID: interception.InitiatorID,
		Requests:    1,
	}); err != nil {
		return err
	}
	for _, dimension := range usageBreakdownDimensions(interception) {
		if err := upsertDailyUsageBreakdown(tx, ProxyDailyUsageBreakdown{
			Day:         day,
			InitiatorID: interception.InitiatorID,
			Dimension:   string(dimension.target),
			Name:        dimension.name,
			Requests:    1,
		}); err != nil {
			return err
		}
	}
	return nil
}

func aggregateInterceptionDuration(tx *gorm.DB, interception Interception) error {
	duration := durationMilliseconds(interception.StartedAt, interception.EndedAt)
	if duration == nil {
		return nil
	}
	return upsertDailyUsageKPI(tx, ProxyDailyUsageKPI{
		Day:             dayStart(interception.StartedAt),
		InitiatorID:     interception.InitiatorID,
		TotalDurationMs: *duration,
	})
}

func aggregatePromptUsage(tx *gorm.DB, interception Interception, prompt UserPrompt) error {
	return upsertDailyUsageKPI(tx, ProxyDailyUsageKPI{
		Day:         dayStart(prompt.CreatedAt),
		InitiatorID: interception.InitiatorID,
		Prompts:     1,
	})
}

func aggregateToolUsage(tx *gorm.DB, interception Interception, usage ToolUsage) error {
	return upsertDailyUsageKPI(tx, ProxyDailyUsageKPI{
		Day:         dayStart(usage.CreatedAt),
		InitiatorID: interception.InitiatorID,
		ToolCalls:   1,
	})
}

func aggregateTokenUsage(tx *gorm.DB, interception Interception, usage TokenUsage) error {
	row := tokenUsageRow{
		InputTokens:           usage.InputTokens,
		OutputTokens:          usage.OutputTokens,
		CacheReadInputTokens:  usage.CacheReadInputTokens,
		CacheWriteInputTokens: usage.CacheWriteInputTokens,
		Type:                  usage.Type,
		Metadata:              usage.Metadata,
	}
	day := dayStart(usage.CreatedAt)
	delta := ProxyDailyUsageKPI{
		Day:                   day,
		InitiatorID:           interception.InitiatorID,
		InputTokens:           usage.InputTokens,
		OutputTokens:          usage.OutputTokens,
		CacheReadInputTokens:  usage.CacheReadInputTokens,
		CacheWriteInputTokens: usage.CacheWriteInputTokens,
	}
	accumulateTokenKPIDelta(&delta, row)
	if err := upsertDailyUsageKPI(tx, delta); err != nil {
		return err
	}
	for _, dimension := range usageBreakdownDimensions(interception) {
		breakdownDelta := ProxyDailyUsageBreakdown{
			Day:                   day,
			InitiatorID:           interception.InitiatorID,
			Dimension:             string(dimension.target),
			Name:                  dimension.name,
			InputTokens:           usage.InputTokens,
			OutputTokens:          usage.OutputTokens,
			CacheReadInputTokens:  usage.CacheReadInputTokens,
			CacheWriteInputTokens: usage.CacheWriteInputTokens,
		}
		accumulateTokenBreakdownDelta(&breakdownDelta, row)
		if err := upsertDailyUsageBreakdown(tx, breakdownDelta); err != nil {
			return err
		}
	}
	return nil
}

func upsertDailyUsageKPI(tx *gorm.DB, delta ProxyDailyUsageKPI) error {
	now := time.Now().UTC()
	delta.Day = dayStart(delta.Day)
	delta.CreatedAt = now
	delta.UpdatedAt = now
	table := ProxyDailyUsageKPI{}.TableName()
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "day"}, {Name: "initiator_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"requests":                 incrementColumn(table, "requests", delta.Requests),
			"prompts":                  incrementColumn(table, "prompts", delta.Prompts),
			"tool_calls":               incrementColumn(table, "tool_calls", delta.ToolCalls),
			"total_duration_ms":        incrementColumn(table, "total_duration_ms", delta.TotalDurationMs),
			"input_tokens":             incrementColumn(table, "input_tokens", delta.InputTokens),
			"output_tokens":            incrementColumn(table, "output_tokens", delta.OutputTokens),
			"cache_read_input_tokens":  incrementColumn(table, "cache_read_input_tokens", delta.CacheReadInputTokens),
			"cache_write_input_tokens": incrementColumn(table, "cache_write_input_tokens", delta.CacheWriteInputTokens),
			"completion_input_tokens":  incrementColumn(table, "completion_input_tokens", delta.CompletionInputTokens),
			"completion_output_tokens": incrementColumn(table, "completion_output_tokens", delta.CompletionOutputTokens),
			"completion_tokens":        incrementColumn(table, "completion_tokens", delta.CompletionTokens),
			"embedding_tokens":         incrementColumn(table, "embedding_tokens", delta.EmbeddingTokens),
			"total_tokens":             incrementColumn(table, "total_tokens", delta.TotalTokens),
			"updated_at":               now,
		}),
	}).Create(&delta).Error
}

func upsertDailyUsageBreakdown(tx *gorm.DB, delta ProxyDailyUsageBreakdown) error {
	now := time.Now().UTC()
	delta.Day = dayStart(delta.Day)
	delta.Name = normalizeBreakdownName(delta.Name)
	delta.CreatedAt = now
	delta.UpdatedAt = now
	table := ProxyDailyUsageBreakdown{}.TableName()
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "day"}, {Name: "initiator_id"}, {Name: "dimension"}, {Name: "name"}},
		DoUpdates: clause.Assignments(map[string]any{
			"requests":                 incrementColumn(table, "requests", delta.Requests),
			"input_tokens":             incrementColumn(table, "input_tokens", delta.InputTokens),
			"output_tokens":            incrementColumn(table, "output_tokens", delta.OutputTokens),
			"cache_read_input_tokens":  incrementColumn(table, "cache_read_input_tokens", delta.CacheReadInputTokens),
			"cache_write_input_tokens": incrementColumn(table, "cache_write_input_tokens", delta.CacheWriteInputTokens),
			"completion_input_tokens":  incrementColumn(table, "completion_input_tokens", delta.CompletionInputTokens),
			"completion_output_tokens": incrementColumn(table, "completion_output_tokens", delta.CompletionOutputTokens),
			"completion_tokens":        incrementColumn(table, "completion_tokens", delta.CompletionTokens),
			"embedding_tokens":         incrementColumn(table, "embedding_tokens", delta.EmbeddingTokens),
			"total_tokens":             incrementColumn(table, "total_tokens", delta.TotalTokens),
			"updated_at":               now,
		}),
	}).Create(&delta).Error
}

func incrementColumn(table, column string, delta int64) clause.Expr {
	return clause.Expr{
		SQL:  "? + ?",
		Vars: []any{clause.Column{Table: table, Name: column}, delta},
	}
}

func accumulateKPIIntoTotals(totals *UsageTotals, row ProxyDailyUsageKPI) {
	totals.Requests += row.Requests
	totals.Prompts += row.Prompts
	totals.ToolCalls += row.ToolCalls
	totals.InputTokens += row.InputTokens
	totals.OutputTokens += row.OutputTokens
	totals.CacheReadInputTokens += row.CacheReadInputTokens
	totals.CacheWriteInputTokens += row.CacheWriteInputTokens
	totals.CompletionInputTokens += row.CompletionInputTokens
	totals.CompletionOutputTokens += row.CompletionOutputTokens
	totals.CompletionTokens += row.CompletionTokens
	totals.EmbeddingTokens += row.EmbeddingTokens
	totals.TotalTokens += row.TotalTokens
}

func accumulateTokenKPIDelta(delta *ProxyDailyUsageKPI, row tokenUsageRow) {
	total := tokenUsageTotal(row)
	delta.TotalTokens += total
	if isEmbeddingTokenUsage(row.Type, row.Metadata) {
		delta.EmbeddingTokens += total
		return
	}
	delta.CompletionInputTokens += completionInputTokens(row)
	delta.CompletionOutputTokens += row.OutputTokens
	delta.CompletionTokens += total
}

func accumulateTokenBreakdownDelta(delta *ProxyDailyUsageBreakdown, row tokenUsageRow) {
	total := tokenUsageTotal(row)
	delta.TotalTokens += total
	if isEmbeddingTokenUsage(row.Type, row.Metadata) {
		delta.EmbeddingTokens += total
		return
	}
	delta.CompletionInputTokens += completionInputTokens(row)
	delta.CompletionOutputTokens += row.OutputTokens
	delta.CompletionTokens += total
}

type usageBreakdownDimension struct {
	target dashboardBreakdownTarget
	name   string
}

func usageBreakdownDimensions(interception Interception) []usageBreakdownDimension {
	return []usageBreakdownDimension{
		{target: dashboardBreakdownModels, name: interception.Model},
		{target: dashboardBreakdownProviderNames, name: interception.Provider},
		{target: dashboardBreakdownProviderTypes, name: interception.ProviderType},
	}
}

func normalizeBreakdownName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "unknown"
	}
	return name
}

func usageDayBounds(startsAt, endsAt time.Time) (time.Time, time.Time) {
	return dayStart(startsAt), dayStart(endsAt)
}

func loadUsageInterception(tx *gorm.DB, interceptionID string) (Interception, error) {
	var interception Interception
	if err := tx.First(&interception, "id = ?", interceptionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Interception{}, ErrUsageEventDependencyMissing
		}
		return Interception{}, err
	}
	return interception, nil
}
