package proxy

import "time"

// UsageWindow selects a dashboard reporting window.
type UsageWindow string

// DashboardBreakdownMetric selects how dashboard breakdowns are ranked.
type DashboardBreakdownMetric string

const (
	UsageWindow7Days   UsageWindow = "7d"
	UsageWindow30Days  UsageWindow = "30d"
	UsageWindow365Days UsageWindow = "365d"
	UsageWindowAll     UsageWindow = "all"
)

const (
	DashboardBreakdownRequests DashboardBreakdownMetric = "requests"
	DashboardBreakdownTokens   DashboardBreakdownMetric = "tokens"
	DashboardBreakdownCost     DashboardBreakdownMetric = "cost"
)

// CostCoverageStatus describes whether raw usage can support the cost estimate
// for the whole requested period.
type CostCoverageStatus string

const (
	CostCoverageComplete    CostCoverageStatus = "complete"
	CostCoveragePartial     CostCoverageStatus = "partial"
	CostCoverageUnavailable CostCoverageStatus = "unavailable"
)

type CostRates struct {
	InputUSDPer1MTokens     float64 `json:"inputUsdPer1MTokens"`
	OutputUSDPer1MTokens    float64 `json:"outputUsdPer1MTokens"`
	EmbeddingUSDPer1MTokens float64 `json:"embeddingUsdPer1MTokens"`
}

type UsageCostConfig struct {
	Rates CostRates
}

type EstimatedCost struct {
	InputUSD     float64   `json:"inputUsd"`
	OutputUSD    float64   `json:"outputUsd"`
	EmbeddingUSD float64   `json:"embeddingUsd"`
	TotalUSD     float64   `json:"totalUsd"`
	Rates        CostRates `json:"rates"`
}

type UsageTotals struct {
	Requests               int64          `json:"requests"`
	ToolCalls              int64          `json:"toolCalls"`
	InputTokens            int64          `json:"inputTokens"`
	OutputTokens           int64          `json:"outputTokens"`
	CacheReadInputTokens   int64          `json:"cacheReadInputTokens"`
	CacheWriteInputTokens  int64          `json:"cacheWriteInputTokens"`
	CompletionInputTokens  int64          `json:"completionInputTokens"`
	CompletionOutputTokens int64          `json:"completionOutputTokens"`
	CompletionTokens       int64          `json:"completionTokens"`
	EmbeddingTokens        int64          `json:"embeddingTokens"`
	TotalTokens            int64          `json:"totalTokens"`
	EstimatedCost          *EstimatedCost `json:"estimatedCost,omitempty"`
}

type DailyUsage struct {
	Date                   string         `json:"date"`
	Requests               int64          `json:"requests"`
	ToolCalls              int64          `json:"toolCalls"`
	InputTokens            int64          `json:"inputTokens"`
	OutputTokens           int64          `json:"outputTokens"`
	CompletionInputTokens  int64          `json:"completionInputTokens"`
	CompletionOutputTokens int64          `json:"completionOutputTokens"`
	CompletionTokens       int64          `json:"completionTokens"`
	EmbeddingTokens        int64          `json:"embeddingTokens"`
	TotalTokens            int64          `json:"totalTokens"`
	EstimatedCost          *EstimatedCost `json:"estimatedCost,omitempty"`
}

type UsageBreakdown struct {
	Name                   string         `json:"name"`
	Requests               int64          `json:"requests"`
	TotalTokens            int64          `json:"totalTokens"`
	EstimatedCost          *EstimatedCost `json:"estimatedCost,omitempty"`
	key                    string
	completionInputTokens  int64
	completionOutputTokens int64
	embeddingTokens        int64
}

type UsageSummary struct {
	Days         int              `json:"days"`
	StartsAt     time.Time        `json:"startsAt"`
	EndsAt       time.Time        `json:"endsAt"`
	Totals       UsageTotals      `json:"totals"`
	Daily        []DailyUsage     `json:"daily"`
	TopModels    []UsageBreakdown `json:"topModels"`
	TopProviders []UsageBreakdown `json:"topProviders"`
}

type UsageWindowMeta struct {
	Window   UsageWindow `json:"window"`
	StartsAt time.Time   `json:"startsAt"`
	EndsAt   time.Time   `json:"endsAt"`
}

// CostCoverage exposes the bounds of raw usage used for a current-price
// estimate. Bounds are omitted when no estimate can be produced.
type CostCoverage struct {
	Status   CostCoverageStatus `json:"status"`
	StartsAt *time.Time         `json:"startsAt,omitempty"`
	EndsAt   *time.Time         `json:"endsAt,omitempty"`
}

// DashboardOverviewPeriod is a comparable dashboard period.
type DashboardOverviewPeriod struct {
	UsageWindowMeta
	Totals            UsageTotals   `json:"totals"`
	TotalDurationMs   int64         `json:"totalDurationMs"`
	AverageDurationMs *float64      `json:"averageDurationMs"`
	Daily             []DailyUsage  `json:"daily"`
	CostCoverage      *CostCoverage `json:"costCoverage,omitempty"`
}

type DashboardOverviewResponse struct {
	UsageWindowMeta
	Totals            UsageTotals                `json:"totals"`
	TotalDurationMs   int64                      `json:"totalDurationMs"`
	AverageDurationMs *float64                   `json:"averageDurationMs"`
	Daily             []DailyUsage               `json:"daily"`
	CostCoverage      *CostCoverage              `json:"costCoverage,omitempty"`
	Previous          *DashboardOverviewPeriod   `json:"previous,omitempty"`
	Adoption          *DashboardAdoptionResponse `json:"adoption,omitempty"`
}

type DashboardTokensResponse struct {
	UsageWindowMeta
	InputTokens            int64          `json:"inputTokens"`
	OutputTokens           int64          `json:"outputTokens"`
	CacheReadInputTokens   int64          `json:"cacheReadInputTokens"`
	CacheWriteInputTokens  int64          `json:"cacheWriteInputTokens"`
	CompletionInputTokens  int64          `json:"completionInputTokens"`
	CompletionOutputTokens int64          `json:"completionOutputTokens"`
	CompletionTokens       int64          `json:"completionTokens"`
	EmbeddingTokens        int64          `json:"embeddingTokens"`
	TotalTokens            int64          `json:"totalTokens"`
	EstimatedCost          *EstimatedCost `json:"estimatedCost,omitempty"`
}

type DashboardMessagesResponse struct {
	UsageWindowMeta
	Messages int64 `json:"messages"`
}

type DashboardDurationResponse struct {
	UsageWindowMeta
	TotalDurationMs int64 `json:"totalDurationMs"`
}

type DashboardActivityResponse struct {
	UsageWindowMeta
	Daily []DailyUsage `json:"daily"`
}

type DashboardBreakdownResponse struct {
	UsageWindowMeta
	Items []UsageBreakdown `json:"items"`
}

// ToolUsageBreakdown reports observed model tool calls by tool name. Calls are
// counted independently from API requests because one request can invoke more
// than one tool.
type ToolUsageBreakdown struct {
	Name  string `json:"name"`
	Calls int64  `json:"calls"`
}

// DashboardTopToolsResponse exposes the top observed tool calls and enough
// totals for clients to identify periods whose older named history predates
// durable per-tool aggregation.
type DashboardTopToolsResponse struct {
	UsageWindowMeta
	TotalCalls      int64                `json:"totalCalls"`
	AttributedCalls int64                `json:"attributedCalls"`
	Items           []ToolUsageBreakdown `json:"items"`
}

type DashboardAdoptionResponse struct {
	UsageWindowMeta
	ActiveUsers           int64 `json:"activeUsers"`
	ActiveServiceAccounts int64 `json:"activeServiceAccounts"`
	ActiveVirtualKeys     int64 `json:"activeVirtualKeys"`
}
