package proxy

import (
	"context"
	"fmt"
	"time"

	"promptgate/backend/internal/domain/auth"

	"gorm.io/gorm"
)

// UsageSummary builds usage totals and daily buckets for a user.
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
	if _, err := s.loadAggregatedUsage(ctx, scope, resolved.StartsAt, resolved.EndsAt, &summary.Totals, dailyByDate); err != nil {
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

	return summary, nil
}

// DashboardOverview returns usage totals, duration, and daily activity for one identity.
func (s *Service) DashboardOverview(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	return s.dashboardOverview(ctx, currentUserDashboardScope(userID), window, now)
}

// DashboardOverviewForIdentities returns usage totals, duration, and daily
// activity aggregated across a selected set of users and service accounts.
func (s *Service) DashboardOverviewForIdentities(ctx context.Context, initiatorIDs []string, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	return s.dashboardOverview(ctx, identitiesDashboardScope(initiatorIDs), window, now)
}

// DashboardOverviewForGroup returns usage totals, duration, and daily activity
// for the access group's current membership without loading member IDs in Go.
func (s *Service) DashboardOverviewForGroup(ctx context.Context, groupID string, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	return s.dashboardOverview(ctx, groupDashboardScope(groupID), window, now)
}

// dashboardOverview builds an overview without comparison for one usage scope.
func (s *Service) dashboardOverview(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}
	period, err := s.loadDashboardOverviewPeriod(ctx, scope, resolved, true)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}
	return dashboardOverviewResponse(period), nil
}

// CurrentUserDashboardOverview returns the current and comparable previous
// usage periods for the authenticated user's dashboard.
func (s *Service) CurrentUserDashboardOverview(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	return s.dashboardOverviewWithComparison(ctx, currentUserDashboardScope(userID), window, now)
}

// AdminDashboardOverview returns global usage, comparison, and adoption KPIs.
func (s *Service) AdminDashboardOverview(ctx context.Context, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	response, err := s.dashboardOverviewWithComparison(ctx, globalDashboardScope(), window, now)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}
	adoption, err := s.AdminDashboardAdoption(ctx, window, now)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}
	response.Adoption = &adoption
	return response, nil
}

// dashboardOverviewWithComparison builds the cockpit overview for one scope.
func (s *Service) dashboardOverviewWithComparison(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}
	current, err := s.loadDashboardOverviewPeriod(ctx, scope, resolved, true)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}
	response := dashboardOverviewResponse(current)

	if previous := previousDashboardPeriod(resolved); previous != nil {
		comparison, err := s.loadDashboardOverviewPeriod(ctx, scope, *previous, true)
		if err != nil {
			return DashboardOverviewResponse{}, err
		}
		response.Previous = &comparison
	}
	return response, nil
}

// loadDashboardOverviewPeriod loads a single concrete dashboard period.
func (s *Service) loadDashboardOverviewPeriod(ctx context.Context, scope dashboardUsageScope, resolved usageRange, includeCostCoverage bool) (DashboardOverviewPeriod, error) {
	period := DashboardOverviewPeriod{
		UsageWindowMeta: resolved.UsageWindowMeta,
		Daily:           []DailyUsage{},
	}
	if resolved.Days > 0 {
		period.Daily = buildDailyBuckets(resolved.StartsAt, resolved.Days)
	}
	dailyByDate := make(map[string]*DailyUsage, len(period.Daily))
	for i := range period.Daily {
		dailyByDate[period.Daily[i].Date] = &period.Daily[i]
	}

	var err error
	period.TotalDurationMs, err = s.loadAggregatedUsage(ctx, scope, resolved.StartsAt, resolved.EndsAt, &period.Totals, dailyByDate)
	if err != nil {
		return DashboardOverviewPeriod{}, err
	}
	period.AverageDurationMs, err = s.averageCompletedDuration(
		ctx,
		scope,
		resolved.StartsAt,
		resolved.EndsAt,
		period.TotalDurationMs,
		period.Totals.Requests,
	)
	if err != nil {
		return DashboardOverviewPeriod{}, err
	}
	if err := s.attachEstimatedCosts(ctx, scope, &period.Totals, period.Daily); err != nil {
		return DashboardOverviewPeriod{}, err
	}

	if includeCostCoverage {
		coverage, err := s.dashboardCostCoverage(ctx, scope, resolved.StartsAt, resolved.EndsAt, period.Totals)
		if err != nil {
			return DashboardOverviewPeriod{}, err
		}
		period.CostCoverage = &coverage
		if coverage.Status == CostCoverageUnavailable {
			period.Totals.EstimatedCost = nil
			for i := range period.Daily {
				period.Daily[i].EstimatedCost = nil
			}
		}
	}

	return period, nil
}

// dashboardOverviewResponse preserves the existing top-level overview shape.
func dashboardOverviewResponse(period DashboardOverviewPeriod) DashboardOverviewResponse {
	return DashboardOverviewResponse{
		UsageWindowMeta:   period.UsageWindowMeta,
		Totals:            period.Totals,
		TotalDurationMs:   period.TotalDurationMs,
		AverageDurationMs: period.AverageDurationMs,
		Daily:             period.Daily,
		CostCoverage:      period.CostCoverage,
	}
}

// previousDashboardPeriod returns the previous equal-length calendar period.
func previousDashboardPeriod(current usageRange) *usageRange {
	if current.Window != UsageWindow7Days && current.Window != UsageWindow30Days {
		return nil
	}
	startsAt := dayStart(current.StartsAt).AddDate(0, 0, -current.Days)
	endsAt := dayStart(current.StartsAt).Add(-time.Nanosecond)
	return &usageRange{
		UsageWindowMeta: UsageWindowMeta{
			Window:   current.Window,
			StartsAt: startsAt,
			EndsAt:   endsAt,
		},
		Days: current.Days,
	}
}

// averageCompletedDuration divides durable duration by completed requests only.
// When raw interceptions have been retained for only part of the requested
// window, it returns nil rather than mixing incompatible coverage periods.
func (s *Service) averageCompletedDuration(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time, totalDurationMs, durableRequests int64) (*float64, error) {
	if durableRequests == 0 {
		return nil, nil
	}
	type durationCoverageRow struct {
		Requests          int64
		CompletedRequests int64
	}
	var row durationCoverageRow
	query := s.db.WithContext(ctx).
		Table("interceptions").
		Select(`COUNT(*) AS requests,
			COALESCE(SUM(CASE WHEN ended_at IS NOT NULL AND ended_at >= started_at THEN 1 ELSE 0 END), 0) AS completed_requests`).
		Where("started_at >= ? AND started_at <= ?", startsAt, endsAt)
	query = scope.applyInitiatorFilter(query, "initiator_id")
	if err := query.Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("load completed request count: %w", err)
	}
	if row.Requests != durableRequests || row.CompletedRequests == 0 {
		return nil, nil
	}
	average := float64(totalDurationMs) / float64(row.CompletedRequests)
	return &average, nil
}

// dashboardCostCoverage compares durable token totals with the retained raw
// token rows used by the current-price estimator.
func (s *Service) dashboardCostCoverage(ctx context.Context, scope dashboardUsageScope, startsAt, endsAt time.Time, totals UsageTotals) (CostCoverage, error) {
	if !s.usageCost.Enabled {
		return CostCoverage{Status: CostCoverageUnavailable}, nil
	}

	type coverageRow struct {
		Requests    int64
		TotalTokens int64
	}
	var row coverageRow
	coverageQuery := func() *gorm.DB {
		query := s.db.WithContext(ctx).
			Table("token_usages").
			Joins("JOIN interceptions ON interceptions.id = token_usages.interception_id")
		return scope.applyInitiatorFilter(query, "interceptions.initiator_id").
			Where("token_usages.created_at >= ? AND token_usages.created_at <= ?", startsAt, endsAt)
	}
	if err := coverageQuery().
		Select(`COUNT(DISTINCT token_usages.interception_id) AS requests,
			COALESCE(SUM(token_usages.input_tokens + token_usages.output_tokens + token_usages.cache_read_input_tokens + token_usages.cache_write_input_tokens), 0) AS total_tokens`).
		Scan(&row).Error; err != nil {
		return CostCoverage{}, fmt.Errorf("load dashboard cost coverage: %w", err)
	}

	if totals.Requests == 0 && totals.TotalTokens == 0 && row.Requests == 0 {
		start, end := startsAt, endsAt
		return CostCoverage{Status: CostCoverageComplete, StartsAt: &start, EndsAt: &end}, nil
	}
	if row.Requests == 0 {
		return CostCoverage{Status: CostCoverageUnavailable}, nil
	}

	var firstUsage TokenUsage
	if err := coverageQuery().
		Select("token_usages.created_at").
		Order("token_usages.created_at ASC").
		Take(&firstUsage).Error; err != nil {
		return CostCoverage{}, fmt.Errorf("load dashboard cost coverage start: %w", err)
	}
	var lastUsage TokenUsage
	if err := coverageQuery().
		Select("token_usages.created_at").
		Order("token_usages.created_at DESC").
		Take(&lastUsage).Error; err != nil {
		return CostCoverage{}, fmt.Errorf("load dashboard cost coverage end: %w", err)
	}
	if row.Requests != totals.Requests || row.TotalTokens != totals.TotalTokens {
		first, last := firstUsage.CreatedAt, lastUsage.CreatedAt
		return CostCoverage{Status: CostCoveragePartial, StartsAt: &first, EndsAt: &last}, nil
	}

	start, end := startsAt, endsAt
	return CostCoverage{Status: CostCoverageComplete, StartsAt: &start, EndsAt: &end}, nil
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

	if _, err := s.loadAggregatedUsage(ctx, scope, resolved.StartsAt, resolved.EndsAt, &summary.Totals, dailyByDate); err != nil {
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
func (s *Service) DashboardTopModels(ctx context.Context, userID string, window UsageWindow, now time.Time, metrics ...DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
	metric, err := resolveDashboardBreakdownMetric(metrics)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	return s.dashboardBreakdown(ctx, currentUserDashboardScope(userID), window, now, dashboardBreakdownModels, metric)
}

// AdminDashboardTopModels returns top model usage across all identities for one dashboard window.
func (s *Service) AdminDashboardTopModels(ctx context.Context, window UsageWindow, now time.Time, metrics ...DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
	metric, err := resolveDashboardBreakdownMetric(metrics)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	return s.dashboardBreakdown(ctx, globalDashboardScope(), window, now, dashboardBreakdownModels, metric)
}

// DashboardTopProviderNames returns top provider-name usage for one dashboard window.
func (s *Service) DashboardTopProviderNames(ctx context.Context, userID string, window UsageWindow, now time.Time, metrics ...DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
	metric, err := resolveDashboardBreakdownMetric(metrics)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	return s.dashboardBreakdown(ctx, currentUserDashboardScope(userID), window, now, dashboardBreakdownProviderNames, metric)
}

// AdminDashboardTopProviderNames returns top provider-name usage across all identities for one dashboard window.
func (s *Service) AdminDashboardTopProviderNames(ctx context.Context, window UsageWindow, now time.Time, metrics ...DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
	metric, err := resolveDashboardBreakdownMetric(metrics)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	return s.dashboardBreakdown(ctx, globalDashboardScope(), window, now, dashboardBreakdownProviderNames, metric)
}

// DashboardTopProviderTypes returns top provider-type usage for one dashboard window.
func (s *Service) DashboardTopProviderTypes(ctx context.Context, userID string, window UsageWindow, now time.Time, metrics ...DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
	metric, err := resolveDashboardBreakdownMetric(metrics)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	return s.dashboardBreakdown(ctx, currentUserDashboardScope(userID), window, now, dashboardBreakdownProviderTypes, metric)
}

// AdminDashboardTopProviderTypes returns top provider-type usage across all identities for one dashboard window.
func (s *Service) AdminDashboardTopProviderTypes(ctx context.Context, window UsageWindow, now time.Time, metrics ...DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
	metric, err := resolveDashboardBreakdownMetric(metrics)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	return s.dashboardBreakdown(ctx, globalDashboardScope(), window, now, dashboardBreakdownProviderTypes, metric)
}

// DashboardTopTools returns the authenticated identity's most frequently
// observed model tool calls for one dashboard window.
func (s *Service) DashboardTopTools(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardTopToolsResponse, error) {
	return s.dashboardTopTools(ctx, currentUserDashboardScope(userID), window, now)
}

// AdminDashboardTopTools returns the most frequently observed model tool calls
// across all identities for one dashboard window.
func (s *Service) AdminDashboardTopTools(ctx context.Context, window UsageWindow, now time.Time) (DashboardTopToolsResponse, error) {
	return s.dashboardTopTools(ctx, globalDashboardScope(), window, now)
}

func (s *Service) dashboardTopTools(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time) (DashboardTopToolsResponse, error) {
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardTopToolsResponse{}, err
	}

	totals, err := s.aggregatedUsageTotals(ctx, scope, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardTopToolsResponse{}, err
	}
	values, err := s.aggregatedBreakdowns(ctx, scope, resolved.StartsAt, resolved.EndsAt, dashboardBreakdownTools)
	if err != nil {
		return DashboardTopToolsResponse{}, err
	}

	attributedCalls := int64(0)
	for _, value := range values {
		attributedCalls += value.Requests
	}
	ranked := sortedBreakdownsByMetric(values, DashboardBreakdownRequests, 5)
	items := make([]ToolUsageBreakdown, 0, len(ranked))
	for _, item := range ranked {
		items = append(items, ToolUsageBreakdown{Name: item.Name, Calls: item.Requests})
	}

	return DashboardTopToolsResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		TotalCalls:      totals.ToolCalls,
		AttributedCalls: attributedCalls,
		Items:           items,
	}, nil
}

// dashboardBreakdown returns top models, provider names, or provider types for a dashboard scope.
func (s *Service) dashboardBreakdown(ctx context.Context, scope dashboardUsageScope, window UsageWindow, now time.Time, target dashboardBreakdownTarget, metric DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
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
	limit := 5
	if metric == DashboardBreakdownCost {
		limit = 0
	}
	items := sortedBreakdownsByMetric(values, metric, limit)
	if s.usageCost.Enabled {
		rates, err := s.usageCostRateBook(ctx)
		if err != nil {
			return DashboardBreakdownResponse{}, err
		}
		if err := s.attachBreakdownEstimatedCosts(ctx, rates, scope, target, items, resolved.StartsAt, resolved.EndsAt); err != nil {
			return DashboardBreakdownResponse{}, err
		}
	}
	if metric == DashboardBreakdownCost {
		items = sortBreakdownItems(items, metric, 5)
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

// AdminDashboardTopIdentities returns top users and service accounts by the selected metric.
func (s *Service) AdminDashboardTopIdentities(ctx context.Context, window UsageWindow, now time.Time, metrics ...DashboardBreakdownMetric) (DashboardBreakdownResponse, error) {
	metric, err := resolveDashboardBreakdownMetric(metrics)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	resolved, err := s.resolveDashboardWindow(ctx, globalDashboardScope(), window, now)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}

	itemsByID, err := s.aggregatedTopIdentities(ctx, resolved.StartsAt, resolved.EndsAt)
	if err != nil {
		return DashboardBreakdownResponse{}, err
	}
	limit := 5
	if metric == DashboardBreakdownCost {
		limit = 0
	}
	items := sortedBreakdownsByMetric(itemsByID, metric, limit)
	if s.usageCost.Enabled {
		rates, err := s.usageCostRateBook(ctx)
		if err != nil {
			return DashboardBreakdownResponse{}, err
		}
		if err := s.attachIdentityEstimatedCosts(ctx, rates, items, resolved.StartsAt, resolved.EndsAt); err != nil {
			return DashboardBreakdownResponse{}, err
		}
	}
	if metric == DashboardBreakdownCost {
		items = sortBreakdownItems(items, metric, 5)
	}

	return DashboardBreakdownResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		Items:           items,
	}, nil
}

func resolveDashboardBreakdownMetric(metrics []DashboardBreakdownMetric) (DashboardBreakdownMetric, error) {
	if len(metrics) == 0 {
		return DashboardBreakdownTokens, nil
	}
	if len(metrics) != 1 {
		return "", ErrInvalidDashboardMetric
	}
	if metrics[0] == "" {
		return DashboardBreakdownTokens, nil
	}
	switch metrics[0] {
	case DashboardBreakdownRequests, DashboardBreakdownTokens, DashboardBreakdownCost:
		return metrics[0], nil
	default:
		return "", ErrInvalidDashboardMetric
	}
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
