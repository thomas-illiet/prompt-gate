package proxy

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
