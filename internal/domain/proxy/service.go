package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/pricing"

	"gorm.io/gorm"
)

var (
	ErrInvalidUsageWindow = errors.New("usage window must be 7 days, 30 days, or all time")
	ErrInvalidPagination  = errors.New("pagination must use page >= 1 and pageSize between 1 and 100")
	ErrInvalidSort        = errors.New("invalid_sort")
)

type UsageWindow string

const (
	UsageWindow7Days  UsageWindow = "7d"
	UsageWindow30Days UsageWindow = "30d"
	UsageWindowAll    UsageWindow = "all"
)

type Service struct {
	db            *gorm.DB
	usageCost     UsageCostConfig
	priceResolver *pricing.Service
}

const usageCostTokenUnit = 1_000_000

var defaultUsageCostConfig = UsageCostConfig{
	Enabled: true,
	Rates: CostRates{
		InputUSDPer1MTokens:     5.00,
		OutputUSDPer1MTokens:    30.00,
		EmbeddingUSDPer1MTokens: 0.02,
	},
}

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

type usageCostRateBook struct {
	fallback              CostRates
	models                map[string]CostRates
	priceEmbeddingAsInput bool
}

type ServiceOption func(*Service)

// WithUsageCost configures dashboard usage cost estimates.
func WithUsageCost(config UsageCostConfig) ServiceOption {
	return func(s *Service) {
		s.usageCost = config
	}
}

// WithPriceResolver configures database-backed per provider/model prices.
func WithPriceResolver(resolver *pricing.Service) ServiceOption {
	return func(s *Service) {
		s.priceResolver = resolver
	}
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

type promptRow struct {
	ID                 string
	InterceptionID     string
	ProviderResponseID string
	Provider           string
	ProviderType       string
	Model              string
	Prompt             string
	StartedAt          time.Time
	EndedAt            *time.Time
	CreatedAt          time.Time
}

type adminPromptRow struct {
	ID                    string
	InterceptionID        string
	ProviderResponseID    string
	Provider              string
	ProviderType          string
	Model                 string
	Prompt                string
	UserID                string
	UserName              string
	UserEmail             string
	UserPreferredUsername string
	ClientIP              string
	StartedAt             time.Time
	EndedAt               *time.Time
	CreatedAt             time.Time
}

type tokenUsageRow struct {
	InterceptionID        string
	ProviderResponseID    string
	Provider              string
	ProviderType          string
	Model                 string
	InputTokens           int64
	OutputTokens          int64
	CacheReadInputTokens  int64
	CacheWriteInputTokens int64
	Type                  string
	Metadata              string
	CreatedAt             time.Time
}

type tokenTotals struct {
	Input  int64
	Output int64
}

type usageRange struct {
	UsageWindowMeta
	Days int
}

type dashboardBreakdownTarget string

const (
	dashboardBreakdownModels        dashboardBreakdownTarget = "models"
	dashboardBreakdownProviderNames dashboardBreakdownTarget = "provider-names"
	dashboardBreakdownProviderTypes dashboardBreakdownTarget = "provider-types"
)

type dashboardUsageScope struct {
	userID string
}

// currentUserDashboardScope creates a dashboard scope limited to one user.
func currentUserDashboardScope(userID string) dashboardUsageScope {
	return dashboardUsageScope{userID: strings.TrimSpace(userID)}
}

// globalDashboardScope creates a dashboard scope across all identities.
func globalDashboardScope() dashboardUsageScope {
	return dashboardUsageScope{}
}

// applyInitiatorFilter restricts a query to the scoped user when needed.
func (scope dashboardUsageScope) applyInitiatorFilter(query *gorm.DB, column string) *gorm.DB {
	if scope.userID == "" {
		return query
	}
	return query.Where(column+" = ?", scope.userID)
}

// NewService creates a proxy usage and prompt history service.
func NewService(db *gorm.DB, options ...ServiceOption) *Service {
	service := &Service{
		db:        db,
		usageCost: defaultUsageCostConfig,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

// UsageSummary builds usage totals, daily buckets, and recent prompts for a user.
func (s *Service) UsageSummary(ctx context.Context, userID string, days int, now time.Time) (UsageSummary, error) {
	window, err := usageWindowForDays(days)
	if err != nil {
		return UsageSummary{}, ErrInvalidUsageWindow
	}
	resolved, err := s.resolveUsageWindow(ctx, userID, window, now)
	if err != nil {
		return UsageSummary{}, err
	}
	summary := UsageSummary{
		Days:     days,
		StartsAt: resolved.StartsAt,
		EndsAt:   resolved.EndsAt,
		Daily:    buildDailyBuckets(resolved.StartsAt, resolved.Days),
	}
	dailyByDate := make(map[string]*DailyUsage, len(summary.Daily))
	for i := range summary.Daily {
		dailyByDate[summary.Daily[i].Date] = &summary.Daily[i]
	}

	scope := currentUserDashboardScope(userID)
	if err := s.loadAggregatedUsage(ctx, scope, resolved.StartsAt, resolved.EndsAt, &summary, dailyByDate); err != nil {
		return UsageSummary{}, err
	}
	var rates usageCostRateBook
	if s.usageCost.Enabled {
		rates, err = s.usageCostRateBook(ctx)
		if err != nil {
			return UsageSummary{}, err
		}
		if err := s.attachEstimatedCostsWithRates(ctx, rates, scope, &summary.Totals, summary.Daily); err != nil {
			return UsageSummary{}, err
		}
	}

	models, err := s.aggregatedBreakdowns(ctx, scope, resolved.StartsAt, resolved.EndsAt, dashboardBreakdownModels)
	if err != nil {
		return UsageSummary{}, err
	}
	providers, err := s.aggregatedBreakdowns(ctx, scope, resolved.StartsAt, resolved.EndsAt, dashboardBreakdownProviderNames)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.TopModels = sortedBreakdowns(models, 5)
	summary.TopProviders = sortedBreakdowns(providers, 5)
	if s.usageCost.Enabled {
		if err := s.attachBreakdownEstimatedCosts(ctx, rates, scope, dashboardBreakdownModels, summary.TopModels, resolved.StartsAt, resolved.EndsAt); err != nil {
			return UsageSummary{}, err
		}
		if err := s.attachBreakdownEstimatedCosts(ctx, rates, scope, dashboardBreakdownProviderNames, summary.TopProviders, resolved.StartsAt, resolved.EndsAt); err != nil {
			return UsageSummary{}, err
		}
	}

	recent, err := s.ListPrompts(ctx, userID, PromptListParams{Page: 1, PageSize: 5})
	if err != nil {
		return UsageSummary{}, err
	}
	summary.RecentPrompts = recent.Items

	return summary, nil
}

// DashboardTokens returns token totals for one dashboard window.
func (s *Service) DashboardTokens(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardTokensResponse, error) {
	return s.dashboardTokens(ctx, currentUserDashboardScope(userID), window, now)
}

// AdminDashboardTokens returns token totals across all identities for one dashboard window.
func (s *Service) AdminDashboardTokens(ctx context.Context, window UsageWindow, now time.Time) (DashboardTokensResponse, error) {
	return s.dashboardTokens(ctx, globalDashboardScope(), window, now)
}

// dashboardTokens loads token totals for the requested dashboard scope and window.
func (s *Service) dashboardTokens(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (DashboardTokensResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardTokensResponse{}, err
	}

	totals, err := s.aggregatedUsageTotals(ctx, scope, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardTokensResponse{}, err
	}

	response := DashboardTokensResponse{
		UsageWindowMeta:        resolved.UsageWindowMeta,
		InputTokens:            totals.InputTokens,
		OutputTokens:           totals.OutputTokens,
		CacheReadInputTokens:   totals.CacheReadInputTokens,
		CacheWriteInputTokens:  totals.CacheWriteInputTokens,
		CompletionInputTokens:  totals.CompletionInputTokens,
		CompletionOutputTokens: totals.CompletionOutputTokens,
		CompletionTokens:       totals.CompletionTokens,
		EmbeddingTokens:        totals.EmbeddingTokens,
		TotalTokens:            totals.TotalTokens,
		EstimatedCost:          nil,
	}
	response.EstimatedCost, err = s.estimateAggregateUsageCost(ctx, scope, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardTokensResponse{}, err
	}

	return response, nil
}

// DashboardMessages returns the request count used as the dashboard message KPI.
func (s *Service) DashboardMessages(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardMessagesResponse, error) {
	return s.dashboardMessages(ctx, currentUserDashboardScope(userID), window, now)
}

// AdminDashboardMessages returns the request count across all identities for one dashboard window.
func (s *Service) AdminDashboardMessages(ctx context.Context, window UsageWindow, now time.Time) (DashboardMessagesResponse, error) {
	return s.dashboardMessages(ctx, globalDashboardScope(), window, now)
}

// dashboardMessages counts proxied requests for the requested dashboard scope and window.
func (s *Service) dashboardMessages(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (DashboardMessagesResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardMessagesResponse{}, err
	}

	totals, err := s.aggregatedUsageTotals(ctx, scope, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardMessagesResponse{}, err
	}

	return DashboardMessagesResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		Messages:        totals.Requests,
	}, nil
}

// DashboardDuration returns the summed duration of completed requests.
func (s *Service) DashboardDuration(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardDurationResponse, error) {
	return s.dashboardDuration(ctx, currentUserDashboardScope(userID), window, now)
}

// AdminDashboardDuration returns the summed duration across all identities for one dashboard window.
func (s *Service) AdminDashboardDuration(ctx context.Context, window UsageWindow, now time.Time) (DashboardDurationResponse, error) {
	return s.dashboardDuration(ctx, globalDashboardScope(), window, now)
}

// dashboardDuration sums completed request durations for the requested dashboard scope and window.
func (s *Service) dashboardDuration(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (DashboardDurationResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardDurationResponse{}, err
	}

	totals, err := s.aggregatedUsageTotals(ctx, scope, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardDurationResponse{}, err
	}

	return DashboardDurationResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		TotalDurationMs: totals.TotalDurationMs,
	}, nil
}

// DashboardActivity returns daily usage buckets for the requested dashboard window.
func (s *Service) DashboardActivity(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardActivityResponse, error) {
	return s.dashboardActivity(ctx, currentUserDashboardScope(userID), window, now)
}

// AdminDashboardActivity returns daily usage buckets across all identities for one window.
func (s *Service) AdminDashboardActivity(ctx context.Context, window UsageWindow, now time.Time) (DashboardActivityResponse, error) {
	return s.dashboardActivity(ctx, globalDashboardScope(), window, now)
}

// dashboardActivity builds daily usage buckets for the requested dashboard scope and window.
func (s *Service) dashboardActivity(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (DashboardActivityResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardActivityResponse{}, err
	}

	daily := []DailyUsage{}
	if resolved.Days > 0 {
		daily = buildDailyBuckets(resolved.StartsAt, resolved.Days)
	}
	summary := UsageSummary{Daily: daily}
	dailyByDate := make(map[string]*DailyUsage, len(summary.Daily))
	for i := range summary.Daily {
		dailyByDate[summary.Daily[i].Date] = &summary.Daily[i]
	}

	if err := s.loadAggregatedUsage(ctx, scope, resolved.StartsAt, resolved.EndsAt, &summary, dailyByDate); err != nil {
		return DashboardActivityResponse{}, err
	}
	if err := s.attachEstimatedCosts(ctx, scope, &summary.Totals, summary.Daily); err != nil {
		return DashboardActivityResponse{}, err
	}

	return DashboardActivityResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		Daily:           summary.Daily,
	}, nil
}

// DashboardTopModels returns top model usage for one dashboard window.
func (s *Service) DashboardTopModels(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardBreakdownResponse, error) {
	return s.dashboardBreakdown(ctx, currentUserDashboardScope(userID), window, now, dashboardBreakdownModels)
}

// AdminDashboardTopModels returns top model usage across all identities for one dashboard window.
func (s *Service) AdminDashboardTopModels(ctx context.Context, window UsageWindow, now time.Time) (DashboardBreakdownResponse, error) {
	return s.dashboardBreakdown(ctx, globalDashboardScope(), window, now, dashboardBreakdownModels)
}

// DashboardTopProviderNames returns top provider-name usage for one dashboard window.
func (s *Service) DashboardTopProviderNames(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardBreakdownResponse, error) {
	return s.dashboardBreakdown(ctx, currentUserDashboardScope(userID), window, now, dashboardBreakdownProviderNames)
}

// AdminDashboardTopProviderNames returns top provider-name usage across all identities for one dashboard window.
func (s *Service) AdminDashboardTopProviderNames(ctx context.Context, window UsageWindow, now time.Time) (DashboardBreakdownResponse, error) {
	return s.dashboardBreakdown(ctx, globalDashboardScope(), window, now, dashboardBreakdownProviderNames)
}

// DashboardTopProviderTypes returns top provider-type usage for one dashboard window.
func (s *Service) DashboardTopProviderTypes(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardBreakdownResponse, error) {
	return s.dashboardBreakdown(ctx, currentUserDashboardScope(userID), window, now, dashboardBreakdownProviderTypes)
}

// AdminDashboardTopProviderTypes returns top provider-type usage across all identities for one dashboard window.
func (s *Service) AdminDashboardTopProviderTypes(ctx context.Context, window UsageWindow, now time.Time) (DashboardBreakdownResponse, error) {
	return s.dashboardBreakdown(ctx, globalDashboardScope(), window, now, dashboardBreakdownProviderTypes)
}

// dashboardBreakdown returns top models, provider names, or provider types for a dashboard scope.
func (s *Service) dashboardBreakdown(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time, target dashboardBreakdownTarget) (DashboardBreakdownResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}

	switch target {
	case dashboardBreakdownModels, dashboardBreakdownProviderNames, dashboardBreakdownProviderTypes:
	default:
		return DashboardBreakdownResponse{}, ErrInvalidSort
	}

	values, err := s.aggregatedBreakdowns(ctx, scope, resolved.StartsAt, resolved.EndsAt, target)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	items := sortedBreakdowns(values, 5)
	if s.usageCost.Enabled {
		rates, err := s.usageCostRateBook(ctx)
		if err != nil {
			return DashboardBreakdownResponse{}, err
		}
		if err := s.attachBreakdownEstimatedCosts(ctx, rates, scope, target, items, resolved.StartsAt, resolved.EndsAt); err != nil {
			return DashboardBreakdownResponse{}, err
		}
	}

	return DashboardBreakdownResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		Items:           items,
	}, nil
}

// AdminDashboardAdoption returns adoption KPIs across all identities for one dashboard window.
func (s *Service) AdminDashboardAdoption(ctx context.Context, window UsageWindow, now time.Time) (DashboardAdoptionResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, globalDashboardScope(), window, now)
	if err != nil {
		return DashboardAdoptionResponse{}, err
	}

	activeUsers, err := s.countActiveIdentitiesFromKPIs(ctx, auth.UserTypeUser, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardAdoptionResponse{}, err
	}
	activeServiceAccounts, err := s.countActiveIdentitiesFromKPIs(ctx, auth.UserTypeService, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardAdoptionResponse{}, err
	}
	activeVirtualKeys, err := s.countActiveVirtualKeys(ctx, resolved.EndsAt)
	if err != nil {
		return DashboardAdoptionResponse{}, err
	}

	return DashboardAdoptionResponse{
		UsageWindowMeta:       resolved.UsageWindowMeta,
		ActiveUsers:           activeUsers,
		ActiveServiceAccounts: activeServiceAccounts,
		ActiveVirtualKeys:     activeVirtualKeys,
	}, nil
}

// AdminDashboardTopIdentities returns top users and service accounts by token volume.
func (s *Service) AdminDashboardTopIdentities(ctx context.Context, window UsageWindow, now time.Time) (DashboardBreakdownResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, globalDashboardScope(), window, now)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}

	itemsByID, err := s.aggregatedTopIdentities(ctx, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	items := sortedBreakdowns(itemsByID, 5)
	if s.usageCost.Enabled {
		rates, err := s.usageCostRateBook(ctx)
		if err != nil {
			return DashboardBreakdownResponse{}, err
		}
		if err := s.attachIdentityEstimatedCosts(ctx, rates, items, resolved.StartsAt, resolved.EndsAt); err != nil {
			return DashboardBreakdownResponse{}, err
		}
	}

	return DashboardBreakdownResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		Items:           items,
	}, nil
}

// countActiveIdentities counts distinct active users or service accounts in a time range.
func (s *Service) countActiveIdentities(ctx context.Context, userType auth.UserType, startsAt, endsAt time.Time) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).
		Table("interceptions").
		Joins("JOIN users ON users.id = interceptions.initiator_id").
		Where("users.type = ? AND interceptions.started_at >= ? AND interceptions.started_at <= ?", userType, startsAt, endsAt).
		Distinct("interceptions.initiator_id").
		Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count active identities: %w", err)
	}
	return total, nil
}

// countActiveVirtualKeys counts non-revoked, non-expired virtual keys at the given time.
func (s *Service) countActiveVirtualKeys(ctx context.Context, now time.Time) (int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).
		Table("tokens").
		Where("revoked_at IS NULL AND expired_at IS NULL AND expires_at > ?", now.UTC()).
		Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count active virtual keys: %w", err)
	}
	return total, nil
}

// ListPrompts returns paginated prompt history enriched with token totals.
func (s *Service) ListPrompts(ctx context.Context, userID string, params PromptListParams) (PromptListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		return PromptListResult{}, ErrInvalidPagination
	}
	if params.SortBy == "" {
		params.SortBy = "createdAt"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	query := s.db.WithContext(ctx).
		Table("user_prompts").
		Joins("JOIN interceptions ON interceptions.id = user_prompts.interception_id").
		Where("interceptions.initiator_id = ?", userID)
	if promptSortNeedsTokenTotals(params.SortBy) {
		query = query.Joins(promptTokenTotalsJoin())
	}
	if search := strings.TrimSpace(strings.ToLower(params.Search)); search != "" {
		query = query.Where("LOWER(user_prompts.prompt) LIKE ?", "%"+search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return PromptListResult{}, fmt.Errorf("count prompt history: %w", err)
	}

	var rows []promptRow
	var err error
	query, err = applyPromptSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return PromptListResult{}, err
	}
	if err := query.
		Select(`user_prompts.id,
			user_prompts.interception_id,
			user_prompts.provider_response_id,
			interceptions.provider,
			interceptions.provider_type,
			interceptions.model,
			user_prompts.prompt,
			interceptions.started_at,
			interceptions.ended_at,
			user_prompts.created_at`).
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Scan(&rows).Error; err != nil {
		return PromptListResult{}, fmt.Errorf("list prompt history: %w", err)
	}

	items := promptRowsToItems(rows)
	if err := s.attachPromptTokenTotals(ctx, items); err != nil {
		return PromptListResult{}, err
	}

	return PromptListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// ListAdminPrompts returns paginated prompt history across all users.
func (s *Service) ListAdminPrompts(ctx context.Context, params AdminPromptListParams) (AdminPromptListResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		return AdminPromptListResult{}, ErrInvalidPagination
	}
	if params.SortBy == "" {
		params.SortBy = "createdAt"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}

	query := s.db.WithContext(ctx).
		Table("user_prompts").
		Joins("JOIN interceptions ON interceptions.id = user_prompts.interception_id").
		Joins("JOIN users ON users.id = interceptions.initiator_id")
	if promptSortNeedsTokenTotals(params.SortBy) {
		query = query.Joins(promptTokenTotalsJoin())
	}
	if search := strings.TrimSpace(strings.ToLower(params.Search)); search != "" {
		query = query.Where("LOWER(user_prompts.prompt) LIKE ?", "%"+search+"%")
	}
	if userID := strings.TrimSpace(params.UserID); userID != "" {
		query = query.Where("interceptions.initiator_id = ?", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return AdminPromptListResult{}, fmt.Errorf("count admin prompt history: %w", err)
	}

	var rows []adminPromptRow
	var err error
	query, err = applyAdminPromptSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return AdminPromptListResult{}, err
	}
	if err := query.
		Select(`user_prompts.id,
			user_prompts.interception_id,
			user_prompts.provider_response_id,
			interceptions.provider,
			interceptions.provider_type,
			interceptions.model,
			user_prompts.prompt,
			interceptions.initiator_id AS user_id,
			users.name AS user_name,
			users.email AS user_email,
			users.preferred_username AS user_preferred_username,
			interceptions.client_ip,
			interceptions.started_at,
			interceptions.ended_at,
			user_prompts.created_at`).
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Scan(&rows).Error; err != nil {
		return AdminPromptListResult{}, fmt.Errorf("list admin prompt history: %w", err)
	}

	items := adminPromptRowsToItems(rows)
	if err := s.attachAdminPromptTokenTotals(ctx, items); err != nil {
		return AdminPromptListResult{}, err
	}

	return AdminPromptListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// loadRequestUsage accumulates request counts by day and requested breakdown maps.
func (s *Service) loadRequestUsage(ctx context.Context, userID string, startsAt, endsAt time.Time, summary *UsageSummary, daily map[string]*DailyUsage, models, providers, providerTypes map[string]*UsageBreakdown) error {
	return s.loadRequestUsageScoped(ctx, currentUserDashboardScope(userID), startsAt, endsAt, summary, daily, models, providers, providerTypes)
}

// loadRequestUsageScoped accumulates request counts for any dashboard usage scope.
func (s *Service) loadRequestUsageScoped(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time, summary *UsageSummary, daily map[string]*DailyUsage, models, providers, providerTypes map[string]*UsageBreakdown) error {
	var rows []Interception
	query := s.db.WithContext(ctx)
	query = scope.applyInitiatorFilter(query, "initiator_id").
		Where("started_at >= ? AND started_at <= ?", startsAt, endsAt)
	if err := query.Find(&rows).Error; err != nil {
		return fmt.Errorf("load request usage: %w", err)
	}

	summary.Totals.Requests = int64(len(rows))
	for _, row := range rows {
		if bucket := daily[dateKey(row.StartedAt)]; bucket != nil {
			bucket.Requests++
		}
		if models != nil {
			breakdown(models, row.Model).Requests++
		}
		if providers != nil {
			breakdown(providers, row.Provider).Requests++
		}
		if providerTypes != nil {
			breakdown(providerTypes, row.ProviderType).Requests++
		}
	}
	return nil
}

// loadPromptUsage accumulates prompt counts into the summary and daily buckets.
func (s *Service) loadPromptUsage(ctx context.Context, userID string, startsAt, endsAt time.Time, summary *UsageSummary, daily map[string]*DailyUsage) error {
	return s.loadPromptUsageScoped(ctx, currentUserDashboardScope(userID), startsAt, endsAt, summary, daily)
}

// loadPromptUsageScoped accumulates prompt counts for any dashboard usage scope.
func (s *Service) loadPromptUsageScoped(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time, summary *UsageSummary, daily map[string]*DailyUsage) error {
	var rows []struct {
		CreatedAt time.Time
	}
	query := s.db.WithContext(ctx).
		Table("user_prompts").
		Select("user_prompts.created_at").
		Joins("JOIN interceptions ON interceptions.id = user_prompts.interception_id")
	query = scope.applyInitiatorFilter(query, "interceptions.initiator_id").
		Where("user_prompts.created_at >= ? AND user_prompts.created_at <= ?", startsAt, endsAt)
	if err := query.Scan(&rows).Error; err != nil {
		return fmt.Errorf("load prompt usage: %w", err)
	}

	summary.Totals.Prompts = int64(len(rows))
	for _, row := range rows {
		if bucket := daily[dateKey(row.CreatedAt)]; bucket != nil {
			bucket.Prompts++
		}
	}
	return nil
}

// loadTokenUsage accumulates token totals by day and requested breakdown maps.
func (s *Service) loadTokenUsage(ctx context.Context, userID string, startsAt, endsAt time.Time, summary *UsageSummary, daily map[string]*DailyUsage, models, providers, providerTypes map[string]*UsageBreakdown) error {
	return s.loadTokenUsageScoped(ctx, currentUserDashboardScope(userID), startsAt, endsAt, summary, daily, models, providers, providerTypes)
}

// loadTokenUsageScoped accumulates token totals for any dashboard usage scope.
func (s *Service) loadTokenUsageScoped(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time, summary *UsageSummary, daily map[string]*DailyUsage, models, providers, providerTypes map[string]*UsageBreakdown) error {
	var rows []tokenUsageRow
	query := s.db.WithContext(ctx).
		Table("token_usages").
		Select(`token_usages.interception_id,
				token_usages.provider_response_id,
				interceptions.provider,
			interceptions.provider_type,
			interceptions.model,
			token_usages.input_tokens,
			token_usages.output_tokens,
			token_usages.cache_read_input_tokens,
			token_usages.cache_write_input_tokens,
				token_usages.type,
				token_usages.metadata,
				token_usages.created_at`).
		Joins("JOIN interceptions ON interceptions.id = token_usages.interception_id")
	query = scope.applyInitiatorFilter(query, "interceptions.initiator_id").
		Where("token_usages.created_at >= ? AND token_usages.created_at <= ?", startsAt, endsAt)
	if err := query.Scan(&rows).Error; err != nil {
		return fmt.Errorf("load token usage: %w", err)
	}

	for _, row := range rows {
		total := tokenUsageTotal(row)
		accumulateTokenTotals(&summary.Totals, row)
		if bucket := daily[dateKey(row.CreatedAt)]; bucket != nil {
			bucket.InputTokens += row.InputTokens
			bucket.OutputTokens += row.OutputTokens
			bucket.TotalTokens += total
			if isEmbeddingTokenUsage(row.Type, row.Metadata) {
				bucket.EmbeddingTokens += total
			} else {
				bucket.CompletionInputTokens += completionInputTokens(row)
				bucket.CompletionOutputTokens += row.OutputTokens
				bucket.CompletionTokens += total
			}
		}
		if models != nil {
			accumulateBreakdownTokenTotals(breakdown(models, row.Model), row)
		}
		if providers != nil {
			accumulateBreakdownTokenTotals(breakdown(providers, row.Provider), row)
		}
		if providerTypes != nil {
			accumulateBreakdownTokenTotals(breakdown(providerTypes, row.ProviderType), row)
		}
	}
	return nil
}

// accumulateTokenTotals adds one token usage row into aggregate totals.
func accumulateTokenTotals(totals *UsageTotals, row tokenUsageRow) {
	total := tokenUsageTotal(row)
	totals.InputTokens += row.InputTokens
	totals.OutputTokens += row.OutputTokens
	totals.CacheReadInputTokens += row.CacheReadInputTokens
	totals.CacheWriteInputTokens += row.CacheWriteInputTokens
	totals.TotalTokens += total
	if isEmbeddingTokenUsage(row.Type, row.Metadata) {
		totals.EmbeddingTokens += total
		return
	}
	totals.CompletionInputTokens += completionInputTokens(row)
	totals.CompletionOutputTokens += row.OutputTokens
	totals.CompletionTokens += total
}

// attachEstimatedCosts adds optional dashboard-only cost estimates to token aggregates.
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

// loadToolUsage counts tool calls for the requested usage window.
func (s *Service) loadToolUsage(ctx context.Context, userID string, startsAt, endsAt time.Time, summary *UsageSummary) error {
	if err := s.db.WithContext(ctx).
		Table("tool_usages").
		Joins("JOIN interceptions ON interceptions.id = tool_usages.interception_id").
		Where("interceptions.initiator_id = ? AND tool_usages.created_at >= ? AND tool_usages.created_at <= ?", userID, startsAt, endsAt).
		Count(&summary.Totals.ToolCalls).Error; err != nil {
		return fmt.Errorf("load tool usage: %w", err)
	}
	return nil
}

// attachPromptTokenTotals fills token totals on prompt history items.
func (s *Service) attachPromptTokenTotals(ctx context.Context, items []PromptHistoryItem) error {
	if len(items) == 0 {
		return nil
	}

	interceptionIDs := make([]string, 0, len(items))
	responseIDs := make([]string, 0, len(items))
	for _, item := range items {
		interceptionIDs = append(interceptionIDs, item.InterceptionID)
		responseIDs = append(responseIDs, item.ProviderResponseID)
	}

	var rows []tokenUsageRow
	if err := s.db.WithContext(ctx).
		Table("token_usages").
		Select("interception_id, provider_response_id, input_tokens, output_tokens").
		Where("interception_id IN ? AND provider_response_id IN ?", interceptionIDs, responseIDs).
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load prompt token totals: %w", err)
	}

	totals := map[string]tokenTotals{}
	for _, row := range rows {
		key := promptTokenKey(row.InterceptionID, row.ProviderResponseID)
		current := totals[key]
		current.Input += row.InputTokens
		current.Output += row.OutputTokens
		totals[key] = current
	}
	for i := range items {
		total := totals[promptTokenKey(items[i].InterceptionID, items[i].ProviderResponseID)]
		items[i].InputTokens = total.Input
		items[i].OutputTokens = total.Output
		items[i].TotalTokens = total.Input + total.Output
	}
	return nil
}

// attachAdminPromptTokenTotals fills token totals on admin prompt history items.
func (s *Service) attachAdminPromptTokenTotals(ctx context.Context, items []AdminPromptHistoryItem) error {
	if len(items) == 0 {
		return nil
	}

	interceptionIDs := make([]string, 0, len(items))
	responseIDs := make([]string, 0, len(items))
	for _, item := range items {
		interceptionIDs = append(interceptionIDs, item.InterceptionID)
		responseIDs = append(responseIDs, item.ProviderResponseID)
	}

	var rows []tokenUsageRow
	if err := s.db.WithContext(ctx).
		Table("token_usages").
		Select("interception_id, provider_response_id, input_tokens, output_tokens").
		Where("interception_id IN ? AND provider_response_id IN ?", interceptionIDs, responseIDs).
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load admin prompt token totals: %w", err)
	}

	totals := map[string]tokenTotals{}
	for _, row := range rows {
		key := promptTokenKey(row.InterceptionID, row.ProviderResponseID)
		current := totals[key]
		current.Input += row.InputTokens
		current.Output += row.OutputTokens
		totals[key] = current
	}
	for i := range items {
		total := totals[promptTokenKey(items[i].InterceptionID, items[i].ProviderResponseID)]
		items[i].InputTokens = total.Input
		items[i].OutputTokens = total.Output
		items[i].TotalTokens = total.Input + total.Output
	}
	return nil
}

// promptSortNeedsTokenTotals reports whether sorting requires the token totals join.
func promptSortNeedsTokenTotals(sortBy string) bool {
	return sortBy == "inputTokens" || sortBy == "outputTokens" || sortBy == "totalTokens"
}

// promptTokenTotalsJoin returns the SQL join used for prompt token aggregate sorting.
func promptTokenTotalsJoin() string {
	return `LEFT JOIN (
		SELECT interception_id,
			provider_response_id,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(input_tokens + output_tokens), 0) AS total_tokens
		FROM token_usages
		GROUP BY interception_id, provider_response_id
	) AS prompt_token_totals
	ON prompt_token_totals.interception_id = user_prompts.interception_id
	AND prompt_token_totals.provider_response_id = user_prompts.provider_response_id`
}

// applyPromptSort applies a validated prompt history order to the query.
func applyPromptSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"prompt":       "user_prompts.prompt",
		"provider":     "interceptions.provider",
		"model":        "interceptions.model",
		"createdAt":    "user_prompts.created_at",
		"durationMs":   durationSortExpression(query),
		"inputTokens":  "COALESCE(prompt_token_totals.input_tokens, 0)",
		"outputTokens": "COALESCE(prompt_token_totals.output_tokens, 0)",
		"totalTokens":  "COALESCE(prompt_token_totals.total_tokens, 0)",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	if sortBy == "durationMs" {
		return applyDurationSort(query, column, dir), nil
	}
	return query.Order(column + " " + dir).Order("user_prompts.id ASC"), nil
}

// applyAdminPromptSort applies a validated admin prompt history order to the query.
func applyAdminPromptSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"prompt":       "user_prompts.prompt",
		"provider":     "interceptions.provider",
		"model":        "interceptions.model",
		"createdAt":    "user_prompts.created_at",
		"durationMs":   durationSortExpression(query),
		"inputTokens":  "COALESCE(prompt_token_totals.input_tokens, 0)",
		"outputTokens": "COALESCE(prompt_token_totals.output_tokens, 0)",
		"totalTokens":  "COALESCE(prompt_token_totals.total_tokens, 0)",
		"userName":     "users.name",
		"userEmail":    "users.email",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	if sortBy == "durationMs" {
		return applyDurationSort(query, column, dir), nil
	}
	return query.Order(column + " " + dir).Order("user_prompts.id ASC"), nil
}

// applyDurationSort orders completed interceptions by duration and leaves pending rows last.
func applyDurationSort(query *gorm.DB, column, dir string) *gorm.DB {
	return query.
		Order("CASE WHEN interceptions.ended_at IS NULL OR interceptions.ended_at < interceptions.started_at THEN 1 ELSE 0 END ASC").
		Order(column + " " + dir).
		Order("user_prompts.id ASC")
}

// durationSortExpression returns a dialect-aware millisecond duration expression.
func durationSortExpression(query *gorm.DB) string {
	if query.Dialector.Name() == "sqlite" {
		return "((julianday(interceptions.ended_at) - julianday(interceptions.started_at)) * 86400000)"
	}

	return "EXTRACT(EPOCH FROM (interceptions.ended_at - interceptions.started_at)) * 1000"
}

// normalizeSortDir converts a prompt sort direction into SQL syntax.
func normalizeSortDir(sortDir string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sortDir)) {
	case "asc":
		return "ASC", nil
	case "desc":
		return "DESC", nil
	default:
		return "", ErrInvalidSort
	}
}

// promptRowsToItems maps database prompt rows into API items.
func promptRowsToItems(rows []promptRow) []PromptHistoryItem {
	items := make([]PromptHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, PromptHistoryItem{
			ID:                 row.ID,
			InterceptionID:     row.InterceptionID,
			ProviderResponseID: row.ProviderResponseID,
			Provider:           row.Provider,
			ProviderType:       row.ProviderType,
			Model:              row.Model,
			Prompt:             row.Prompt,
			DurationMs:         durationMilliseconds(row.StartedAt, row.EndedAt),
			CreatedAt:          row.CreatedAt,
		})
	}
	return items
}

// adminPromptRowsToItems maps admin database prompt rows into API items.
func adminPromptRowsToItems(rows []adminPromptRow) []AdminPromptHistoryItem {
	items := make([]AdminPromptHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, AdminPromptHistoryItem{
			ID:                    row.ID,
			InterceptionID:        row.InterceptionID,
			ProviderResponseID:    row.ProviderResponseID,
			Provider:              row.Provider,
			ProviderType:          row.ProviderType,
			Model:                 row.Model,
			Prompt:                row.Prompt,
			UserID:                row.UserID,
			UserName:              row.UserName,
			UserEmail:             row.UserEmail,
			UserPreferredUsername: row.UserPreferredUsername,
			ClientIP:              row.ClientIP,
			DurationMs:            durationMilliseconds(row.StartedAt, row.EndedAt),
			CreatedAt:             row.CreatedAt,
		})
	}
	return items
}

// durationMilliseconds returns a completed interception duration in milliseconds.
func durationMilliseconds(startedAt time.Time, endedAt *time.Time) *int64 {
	if startedAt.IsZero() || endedAt == nil || endedAt.Before(startedAt) {
		return nil
	}

	duration := endedAt.Sub(startedAt).Milliseconds()
	return &duration
}

// usageWindowForDays converts legacy day counts into dashboard usage windows.
func usageWindowForDays(days int) (UsageWindow, error) {
	switch days {
	case 7:
		return UsageWindow7Days, nil
	case 30:
		return UsageWindow30Days, nil
	default:
		return "", ErrInvalidUsageWindow
	}
}

// resolveUsageWindow resolves a current-user usage window into concrete timestamps.
func (s *Service) resolveUsageWindow(ctx context.Context, userID string, window UsageWindow, now time.Time) (usageRange, error) {
	return s.resolveDashboardWindow(ctx, currentUserDashboardScope(userID), window, now)
}

// resolveDashboardWindow resolves a dashboard window into concrete UTC boundaries.
func (s *Service) resolveDashboardWindow(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (usageRange, error) {
	if now.IsZero() {
		now = time.Now()
	}
	endsAt := now.UTC()

	switch window {
	case "":
		window = UsageWindow30Days
	case UsageWindow7Days, UsageWindow30Days, UsageWindowAll:
	default:
		return usageRange{}, ErrInvalidUsageWindow
	}

	if window == UsageWindowAll {
		firstActivityAt, ok, err := s.firstActivityAt(ctx, scope)
		if err != nil {
			return usageRange{}, err
		}
		if !ok {
			return usageRange{
				UsageWindowMeta: UsageWindowMeta{
					Window:   UsageWindowAll,
					StartsAt: endsAt,
					EndsAt:   endsAt,
				},
			}, nil
		}

		startsAt := dayStart(firstActivityAt)
		days := daysBetween(startsAt, dayStart(endsAt)) + 1
		if days < 1 {
			days = 1
		}
		return usageRange{
			UsageWindowMeta: UsageWindowMeta{
				Window:   UsageWindowAll,
				StartsAt: startsAt,
				EndsAt:   endsAt,
			},
			Days: days,
		}, nil
	}

	days := 30
	if window == UsageWindow7Days {
		days = 7
	}
	startsAt := dayStart(endsAt).AddDate(0, 0, -(days - 1))
	return usageRange{
		UsageWindowMeta: UsageWindowMeta{
			Window:   window,
			StartsAt: startsAt,
			EndsAt:   endsAt,
		},
		Days: days,
	}, nil
}

// firstUserActivityAt returns the earliest recorded request for one user.
func (s *Service) firstUserActivityAt(ctx context.Context, userID string) (time.Time, bool, error) {
	return s.firstActivityAt(ctx, currentUserDashboardScope(userID))
}

// firstActivityAt returns the earliest recorded request for a dashboard scope.
func (s *Service) firstActivityAt(ctx context.Context, scope dashboardUsageScope) (time.Time, bool, error) {
	var row struct {
		Day time.Time
	}
	query := s.db.WithContext(ctx).
		Model(&ProxyDailyUsageKPI{}).
		Select("day")
	query = scope.applyInitiatorFilter(query, "initiator_id").
		Order("day ASC")
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("load first activity: %w", err)
	}
	return row.Day, true, nil
}

// buildDailyBuckets creates empty daily usage buckets for a window.
func buildDailyBuckets(startsAt time.Time, days int) []DailyUsage {
	buckets := make([]DailyUsage, 0, days)
	for i := 0; i < days; i++ {
		buckets = append(buckets, DailyUsage{Date: startsAt.AddDate(0, 0, i).Format("2006-01-02")})
	}
	return buckets
}

// breakdown returns an existing or new usage breakdown for a display name.
func breakdown(values map[string]*UsageBreakdown, name string) *UsageBreakdown {
	return breakdownByKey(values, name, name)
}

// breakdownByKey returns an existing or new usage breakdown for a stable key.
func breakdownByKey(values map[string]*UsageBreakdown, key, name string) *UsageBreakdown {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = name
	}
	if values[key] == nil {
		values[key] = &UsageBreakdown{Name: name, key: key}
	}
	return values[key]
}

// accumulateBreakdownTokenTotals adds token and cost buckets to a breakdown.
func accumulateBreakdownTokenTotals(item *UsageBreakdown, row tokenUsageRow) {
	total := tokenUsageTotal(row)
	item.TotalTokens += total
	if isEmbeddingTokenUsage(row.Type, row.Metadata) {
		item.embeddingTokens += total
		return
	}
	item.completionInputTokens += completionInputTokens(row)
	item.completionOutputTokens += row.OutputTokens
}

// sortedBreakdowns returns the highest-volume usage breakdowns.
func sortedBreakdowns(values map[string]*UsageBreakdown, limit int) []UsageBreakdown {
	items := make([]UsageBreakdown, 0, len(values))
	for _, value := range values {
		if value.Requests == 0 && value.TotalTokens == 0 {
			continue
		}
		items = append(items, *value)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalTokens == items[j].TotalTokens {
			if items[i].Requests == items[j].Requests {
				return items[i].Name < items[j].Name
			}
			return items[i].Requests > items[j].Requests
		}
		return items[i].TotalTokens > items[j].TotalTokens
	})
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

// dateKey formats a timestamp as a UTC daily bucket key.
func dateKey(value time.Time) string {
	return value.UTC().Format("2006-01-02")
}

// dayStart returns the UTC midnight for a timestamp.
func dayStart(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// daysBetween returns the whole-day distance between two UTC-normalized dates.
func daysBetween(start, end time.Time) int {
	return int(dayStart(end).Sub(dayStart(start)).Hours() / 24)
}

// promptTokenKey builds a collision-safe key for prompt token totals.
func promptTokenKey(interceptionID, responseID string) string {
	return interceptionID + "\x00" + responseID
}
