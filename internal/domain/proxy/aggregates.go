package proxy

import "errors"

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
