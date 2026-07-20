package proxy

import (
	"context"
	"fmt"
	"time"

	"promptgate/backend/internal/domain/auth"
)

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

	recent, err := s.ListPrompts(ctx, userID, PromptListParams{Page: 1, PageSize: 5})
	if err != nil {
		return UsageSummary{}, err
	}
	summary.RecentPrompts = recent.Items

	return summary, nil
}

// DashboardOverview returns usage totals, duration, and daily activity for one user.
func (s *Service) DashboardOverview(ctx context.Context, userID string, window UsageWindow, now time.Time) (DashboardOverviewResponse, error) {
	scope := currentUserDashboardScope(userID)
	resolved, err := s.resolveDashboardWindow(ctx, scope, window, now)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}

	response := DashboardOverviewResponse{
		UsageWindowMeta: resolved.UsageWindowMeta,
		Daily:           []DailyUsage{},
	}
	if resolved.Days > 0 {
		response.Daily = buildDailyBuckets(resolved.StartsAt, resolved.Days)
	}
	dailyByDate := make(map[string]*DailyUsage, len(response.Daily))
	for i := range response.Daily {
		dailyByDate[response.Daily[i].Date] = &response.Daily[i]
	}

	response.TotalDurationMs, err = s.loadAggregatedUsage(ctx, scope, resolved.StartsAt, resolved.EndsAt, &response.Totals, dailyByDate)
	if err != nil {
		return DashboardOverviewResponse{}, err
	}
	if err := s.attachEstimatedCosts(ctx, scope, &response.Totals, response.Daily); err != nil {
		return DashboardOverviewResponse{}, err
	}

	return response, nil
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
