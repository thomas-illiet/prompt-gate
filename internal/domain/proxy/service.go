package proxy

import (
	"errors"
	"strings"
	"time"

	"promptgate/backend/internal/domain/pricing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidUsageWindow     = errors.New("usage window must be 7 days, 30 days, or all time")
	ErrInvalidSort            = errors.New("invalid_sort")
	ErrInvalidDashboardMetric = errors.New("dashboard metric must be requests, tokens, or cost")
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

type usageRange struct {
	UsageWindowMeta
	Days int
}

type dashboardBreakdownTarget string

const (
	dashboardBreakdownModels        dashboardBreakdownTarget = "models"
	dashboardBreakdownProviderNames dashboardBreakdownTarget = "provider-names"
	dashboardBreakdownProviderTypes dashboardBreakdownTarget = "provider-types"
	dashboardBreakdownTools         dashboardBreakdownTarget = "tools"
)

type dashboardUsageScope struct {
	global        bool
	initiatorIDs  []string
	memberGroupID string
}

// currentUserDashboardScope creates a dashboard scope limited to one user.
func currentUserDashboardScope(userID string) dashboardUsageScope {
	return identitiesDashboardScope([]string{userID})
}

// identitiesDashboardScope creates a dashboard scope limited to a normalized,
// de-duplicated set of identities. An empty set intentionally matches no usage.
func identitiesDashboardScope(initiatorIDs []string) dashboardUsageScope {
	normalized := make([]string, 0, len(initiatorIDs))
	seen := make(map[string]struct{}, len(initiatorIDs))
	for _, initiatorID := range initiatorIDs {
		initiatorID = strings.TrimSpace(initiatorID)
		if initiatorID == "" {
			continue
		}
		if _, exists := seen[initiatorID]; exists {
			continue
		}
		seen[initiatorID] = struct{}{}
		normalized = append(normalized, initiatorID)
	}
	return dashboardUsageScope{initiatorIDs: normalized}
}

// groupDashboardScope creates a dashboard scope that resolves the current
// access-group membership in SQL. Keeping the group identifier in the scope
// avoids materializing an arbitrarily large member list in the API process.
func groupDashboardScope(groupID string) dashboardUsageScope {
	normalized := strings.TrimSpace(groupID)
	if parsed, err := uuid.Parse(normalized); err == nil {
		normalized = parsed.String()
	}
	return dashboardUsageScope{memberGroupID: normalized}
}

// globalDashboardScope creates a dashboard scope across all identities.
func globalDashboardScope() dashboardUsageScope {
	return dashboardUsageScope{global: true}
}

// applyInitiatorFilter restricts a query to the scoped identities when needed.
func (scope dashboardUsageScope) applyInitiatorFilter(query *gorm.DB, column string) *gorm.DB {
	if scope.global {
		return query
	}
	if scope.memberGroupID != "" {
		return query.Where(
			column+" IN (SELECT user_id FROM access_group_members WHERE group_id = ?)",
			scope.memberGroupID,
		)
	}
	if len(scope.initiatorIDs) == 0 {
		return query.Where("1 = 0")
	}
	if len(scope.initiatorIDs) == 1 {
		return query.Where(column+" = ?", scope.initiatorIDs[0])
	}
	return query.Where(column+" IN ?", scope.initiatorIDs)
}

// NewService creates a proxy usage service.
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
