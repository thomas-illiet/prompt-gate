package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/proxy"
)

// handleCurrentUserUsage returns the authenticated user's usage summary.
func (s server) handleCurrentUserUsage(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	days, err := parseUsageDays(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_usage_window"})
		return
	}
	if s.proxyService == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "proxy usage service unavailable"})
		return
	}

	summary, err := s.proxyService.UsageSummary(r.Context(), user.ID, days, time.Now().UTC())
	if err != nil {
		if errors.Is(err, proxy.ErrInvalidUsageWindow) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_usage_window"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// handleCurrentUserDashboardTokens returns token totals for one dashboard window.
func (s server) handleCurrentUserDashboardTokens(w http.ResponseWriter, r *http.Request) {
	user, window, ok := s.currentUserDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := s.proxyService.DashboardTokens(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCurrentUserDashboardMessages returns request/message totals for one dashboard window.
func (s server) handleCurrentUserDashboardMessages(w http.ResponseWriter, r *http.Request) {
	user, window, ok := s.currentUserDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := s.proxyService.DashboardMessages(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCurrentUserDashboardDuration returns completed request duration totals for one dashboard window.
func (s server) handleCurrentUserDashboardDuration(w http.ResponseWriter, r *http.Request) {
	user, window, ok := s.currentUserDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := s.proxyService.DashboardDuration(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCurrentUserDashboardActivity returns daily dashboard activity for one window.
func (s server) handleCurrentUserDashboardActivity(w http.ResponseWriter, r *http.Request) {
	user, window, ok := s.currentUserDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := s.proxyService.DashboardActivity(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCurrentUserDashboardTopModels returns top model usage for one dashboard window.
func (s server) handleCurrentUserDashboardTopModels(w http.ResponseWriter, r *http.Request) {
	user, window, ok := s.currentUserDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := s.proxyService.DashboardTopModels(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCurrentUserDashboardTopProviderNames returns top provider-name usage for one dashboard window.
func (s server) handleCurrentUserDashboardTopProviderNames(w http.ResponseWriter, r *http.Request) {
	user, window, ok := s.currentUserDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := s.proxyService.DashboardTopProviderNames(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCurrentUserDashboardTopProviderTypes returns top provider-type usage for one dashboard window.
func (s server) handleCurrentUserDashboardTopProviderTypes(w http.ResponseWriter, r *http.Request) {
	user, window, ok := s.currentUserDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := s.proxyService.DashboardTopProviderTypes(r.Context(), user.ID, window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCurrentUserPrompts returns paged prompt history for the authenticated user.
func (s server) handleCurrentUserPrompts(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	params, err := parsePromptListParams(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_pagination"})
		return
	}
	if s.proxyService == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "proxy usage service unavailable"})
		return
	}

	result, err := s.proxyService.ListPrompts(r.Context(), user.ID, params)
	if err != nil {
		if errors.Is(err, proxy.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		if errors.Is(err, proxy.ErrInvalidPagination) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_pagination"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleCurrentUserGroups returns the authenticated user's access groups.
func (s server) handleCurrentUserGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	if s.groups == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "groups service unavailable"})
		return
	}

	result, err := s.groups.ListUserGroupSummaries(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load_user_groups_failed"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleHelpSetup returns provider setup metadata for authenticated users.
func (s server) handleHelpSetup(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return
	}
	if s.providers == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "provider service unavailable"})
		return
	}
	if s.groups == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "groups service unavailable"})
		return
	}

	access, err := s.groups.UserAccess(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load_user_access_failed"})
		return
	}

	setup, err := s.providers.HelpSetupForProviderNames(
		r.Context(),
		s.config.ProxyBaseURL,
		access.Providers,
		access.Allows,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, setup)
}

// currentUserDashboardRequest validates the authenticated dashboard request and returns its user and window.
func (s server) currentUserDashboardRequest(w http.ResponseWriter, r *http.Request) (auth.UserProfile, proxy.UsageWindow, bool) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing user in context"})
		return auth.UserProfile{}, "", false
	}
	window, err := parseUsageWindow(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_usage_window"})
		return auth.UserProfile{}, "", false
	}
	if s.proxyService == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "proxy usage service unavailable"})
		return auth.UserProfile{}, "", false
	}

	return user, window, true
}

// writeDashboardError writes a current-user dashboard error response and reports whether it handled an error.
func writeDashboardError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, proxy.ErrInvalidUsageWindow) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_usage_window"})
		return true
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	return true
}

// parseUsageWindow reads the dashboard usage window from the query string.
func parseUsageWindow(r *http.Request) (proxy.UsageWindow, error) {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("window")))
	if value == "" {
		if days := strings.TrimSpace(r.URL.Query().Get("days")); days != "" {
			parsed, err := strconv.Atoi(days)
			if err != nil {
				return "", err
			}
			switch parsed {
			case 7:
				return proxy.UsageWindow7Days, nil
			case 30:
				return proxy.UsageWindow30Days, nil
			default:
				return "", proxy.ErrInvalidUsageWindow
			}
		}
		return proxy.UsageWindow30Days, nil
	}

	switch proxy.UsageWindow(value) {
	case proxy.UsageWindow7Days, proxy.UsageWindow30Days, proxy.UsageWindowAll:
		return proxy.UsageWindow(value), nil
	default:
		return "", proxy.ErrInvalidUsageWindow
	}
}

// parseUsageDays reads the allowed usage window from the query string.
func parseUsageDays(r *http.Request) (int, error) {
	value := r.URL.Query().Get("days")
	if value == "" {
		return 30, nil
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if days != 7 && days != 30 {
		return 0, proxy.ErrInvalidUsageWindow
	}
	return days, nil
}

// parsePromptListParams converts prompt history query parameters into service filters.
func parsePromptListParams(r *http.Request) (proxy.PromptListParams, error) {
	query := r.URL.Query()
	page := 1
	pageSize := 10

	if value := query.Get("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return proxy.PromptListParams{}, err
		}
		page = parsed
	}
	if value := query.Get("pageSize"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return proxy.PromptListParams{}, err
		}
		pageSize = parsed
	}
	if page <= 0 || pageSize <= 0 || pageSize > 100 {
		return proxy.PromptListParams{}, proxy.ErrInvalidPagination
	}

	return proxy.PromptListParams{
		Page:     page,
		PageSize: pageSize,
		Search:   query.Get("search"),
		SortBy:   query.Get("sortBy"),
		SortDir:  query.Get("sortDir"),
	}, nil
}
