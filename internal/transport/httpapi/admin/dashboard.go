package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"promptgate/backend/internal/domain/proxy"
)

type adminDashboardLoader func(context.Context, proxy.UsageWindow, time.Time) (any, error)

// HandleAdminDashboardOverview returns the organization cockpit overview.
func (h *Handler) HandleAdminDashboardOverview(w http.ResponseWriter, r *http.Request) {
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardOverview(ctx, window, now)
	})
}

// HandleAdminDashboardTokens returns global token totals for one dashboard window.
func (h *Handler) HandleAdminDashboardTokens(w http.ResponseWriter, r *http.Request) {
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardTokens(ctx, window, now)
	})
}

// HandleAdminDashboardMessages returns global request/message totals for one dashboard window.
func (h *Handler) HandleAdminDashboardMessages(w http.ResponseWriter, r *http.Request) {
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardMessages(ctx, window, now)
	})
}

// HandleAdminDashboardDuration returns global completed request duration totals for one dashboard window.
func (h *Handler) HandleAdminDashboardDuration(w http.ResponseWriter, r *http.Request) {
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardDuration(ctx, window, now)
	})
}

// HandleAdminDashboardActivity returns global daily usage for one dashboard window.
func (h *Handler) HandleAdminDashboardActivity(w http.ResponseWriter, r *http.Request) {
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardActivity(ctx, window, now)
	})
}

// HandleAdminDashboardTopModels returns global top model usage for one dashboard window.
func (h *Handler) HandleAdminDashboardTopModels(w http.ResponseWriter, r *http.Request) {
	metric, ok := dashboardBreakdownMetricRequest(w, r)
	if !ok {
		return
	}
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardTopModels(ctx, window, now, metric)
	})
}

// HandleAdminDashboardTopProviderNames returns global top provider-name usage for one dashboard window.
func (h *Handler) HandleAdminDashboardTopProviderNames(w http.ResponseWriter, r *http.Request) {
	metric, ok := dashboardBreakdownMetricRequest(w, r)
	if !ok {
		return
	}
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardTopProviderNames(ctx, window, now, metric)
	})
}

// HandleAdminDashboardTopProviderTypes returns global top provider-type usage for one dashboard window.
func (h *Handler) HandleAdminDashboardTopProviderTypes(w http.ResponseWriter, r *http.Request) {
	metric, ok := dashboardBreakdownMetricRequest(w, r)
	if !ok {
		return
	}
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardTopProviderTypes(ctx, window, now, metric)
	})
}

// HandleAdminDashboardAdoption returns global adoption KPIs for one dashboard window.
func (h *Handler) HandleAdminDashboardAdoption(w http.ResponseWriter, r *http.Request) {
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardAdoption(ctx, window, now)
	})
}

// HandleAdminDashboardTopIdentities returns top users and service accounts by the selected metric.
func (h *Handler) HandleAdminDashboardTopIdentities(w http.ResponseWriter, r *http.Request) {
	metric, ok := dashboardBreakdownMetricRequest(w, r)
	if !ok {
		return
	}
	h.writeDashboardResponse(w, r, func(ctx context.Context, window proxy.UsageWindow, now time.Time) (any, error) {
		return h.proxy.AdminDashboardTopIdentities(ctx, window, now, metric)
	})
}

// writeDashboardResponse validates the admin dashboard request and writes the loaded response.
func (h *Handler) writeDashboardResponse(w http.ResponseWriter, r *http.Request, load adminDashboardLoader) {
	window, ok := h.adminDashboardRequest(w, r)
	if !ok {
		return
	}

	response, err := load(r.Context(), window, time.Now().UTC())
	if writeDashboardError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// adminDashboardRequest validates dashboard dependencies and query parameters.
func (h *Handler) adminDashboardRequest(w http.ResponseWriter, r *http.Request) (proxy.UsageWindow, bool) {
	if h.proxy == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "proxy usage service unavailable"})
		return "", false
	}
	window, err := parseDashboardWindow(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_usage_window"})
		return "", false
	}
	return window, true
}

// parseDashboardWindow reads an admin dashboard usage window from query parameters.
func parseDashboardWindow(r *http.Request) (proxy.UsageWindow, error) {
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

// dashboardBreakdownMetricRequest validates the optional breakdown ranking metric.
func dashboardBreakdownMetricRequest(w http.ResponseWriter, r *http.Request) (proxy.DashboardBreakdownMetric, bool) {
	metric, err := parseDashboardBreakdownMetric(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_dashboard_metric"})
		return "", false
	}
	return metric, true
}

func parseDashboardBreakdownMetric(r *http.Request) (proxy.DashboardBreakdownMetric, error) {
	metric := proxy.DashboardBreakdownMetric(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("metric"))))
	if metric == "" {
		return proxy.DashboardBreakdownTokens, nil
	}
	switch metric {
	case proxy.DashboardBreakdownRequests, proxy.DashboardBreakdownTokens, proxy.DashboardBreakdownCost:
		return metric, nil
	default:
		return "", proxy.ErrInvalidDashboardMetric
	}
}

// writeDashboardError writes a dashboard error response and reports whether an error was handled.
func writeDashboardError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, proxy.ErrInvalidUsageWindow) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_usage_window"})
		return true
	}
	if errors.Is(err, proxy.ErrInvalidDashboardMetric) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_dashboard_metric"})
		return true
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	return true
}
