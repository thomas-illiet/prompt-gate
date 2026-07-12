package proxy

import (
	"errors"
	"strings"
	"time"

	"promptgate/backend/internal/domain/pricing"

	"gorm.io/gorm"
)

var (
	ErrInvalidUsageWindow = errors.New("usage window must be 7 days, 30 days, or all time")
	ErrInvalidPagination  = errors.New("pagination must use page >= 1 and pageSize between 1 and 100")
	ErrInvalidSort        = errors.New("invalid_sort")
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
