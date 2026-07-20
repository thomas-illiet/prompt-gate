package proxy

import "time"

// UsageWindow selects a dashboard reporting window.
type UsageWindow string

const (
	UsageWindow7Days  UsageWindow = "7d"
	UsageWindow30Days UsageWindow = "30d"
	UsageWindowAll    UsageWindow = "all"
)

type CostRates struct {
	InputUSDPer1MTokens     float64 `json:"inputUsdPer1MTokens"`
	OutputUSDPer1MTokens    float64 `json:"outputUsdPer1MTokens"`
	EmbeddingUSDPer1MTokens float64 `json:"embeddingUsdPer1MTokens"`
}

type UsageCostConfig struct {
	Enabled bool
	Rates   CostRates
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
	Prompts                int64          `json:"prompts"`
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
	Prompts                int64          `json:"prompts"`
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
	Days          int                 `json:"days"`
	StartsAt      time.Time           `json:"startsAt"`
	EndsAt        time.Time           `json:"endsAt"`
	Totals        UsageTotals         `json:"totals"`
	Daily         []DailyUsage        `json:"daily"`
	TopModels     []UsageBreakdown    `json:"topModels"`
	TopProviders  []UsageBreakdown    `json:"topProviders"`
	RecentPrompts []PromptHistoryItem `json:"recentPrompts"`
}

type UsageWindowMeta struct {
	Window   UsageWindow `json:"window"`
	StartsAt time.Time   `json:"startsAt"`
	EndsAt   time.Time   `json:"endsAt"`
}

type DashboardOverviewResponse struct {
	UsageWindowMeta
	Totals          UsageTotals  `json:"totals"`
	TotalDurationMs int64        `json:"totalDurationMs"`
	Daily           []DailyUsage `json:"daily"`
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

type DashboardAdoptionResponse struct {
	UsageWindowMeta
	ActiveUsers           int64 `json:"activeUsers"`
	ActiveServiceAccounts int64 `json:"activeServiceAccounts"`
	ActiveVirtualKeys     int64 `json:"activeVirtualKeys"`
}

type PromptHistoryItem struct {
	ID                 string    `json:"id"`
	InterceptionID     string    `json:"interceptionId"`
	ProviderResponseID string    `json:"providerResponseId"`
	Provider           string    `json:"provider"`
	ProviderType       string    `json:"providerType"`
	Model              string    `json:"model"`
	Prompt             string    `json:"prompt"`
	InputTokens        int64     `json:"inputTokens"`
	OutputTokens       int64     `json:"outputTokens"`
	TotalTokens        int64     `json:"totalTokens"`
	DurationMs         *int64    `json:"durationMs"`
	CreatedAt          time.Time `json:"createdAt"`
}

type AdminPromptHistoryItem struct {
	ID                    string    `json:"id"`
	InterceptionID        string    `json:"interceptionId"`
	ProviderResponseID    string    `json:"providerResponseId"`
	Provider              string    `json:"provider"`
	ProviderType          string    `json:"providerType"`
	Model                 string    `json:"model"`
	Prompt                string    `json:"prompt"`
	UserID                string    `json:"userId"`
	UserName              string    `json:"userName"`
	UserEmail             string    `json:"userEmail"`
	UserPreferredUsername string    `json:"userPreferredUsername"`
	ClientIP              string    `json:"clientIp"`
	InputTokens           int64     `json:"inputTokens"`
	OutputTokens          int64     `json:"outputTokens"`
	TotalTokens           int64     `json:"totalTokens"`
	DurationMs            *int64    `json:"durationMs"`
	CreatedAt             time.Time `json:"createdAt"`
}

type PromptListParams struct {
	Page     int
	PageSize int
	Search   string
	SortBy   string
	SortDir  string
}

type AdminPromptListParams struct {
	Page     int
	PageSize int
	Search   string
	SortBy   string
	SortDir  string
	UserID   string
}

type PromptListResult struct {
	Items    []PromptHistoryItem `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Total    int64               `json:"total"`
}

type AdminPromptListResult struct {
	Items    []AdminPromptHistoryItem `json:"items"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
	Total    int64                    `json:"total"`
}
