package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/domain/proxy"
)

// TestHandleAdminServiceAccountStatisticsReturnsOnlyTargetUsage verifies the response shape and service-account isolation.
func TestHandleAdminServiceAccountStatisticsReturnsOnlyTargetUsage(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	at := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, statisticsHumanUserID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", at, 10, 20)
	seedDashboardUsage(t, db, statisticsServiceUserID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", at, 30, 40)
	if err := db.Model(&proxy.ProxyDailyUsageKPI{}).
		Where("initiator_id = ?", statisticsServiceUserID).
		Update("total_duration_ms", 750).Error; err != nil {
		t.Fatalf("seed duration: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-accounts/"+statisticsServiceUserID+"/statistics?window=7d", nil)
	req.SetPathValue("id", statisticsServiceUserID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminServiceAccountStatistics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Window != proxy.UsageWindow7Days {
		t.Fatalf("expected 7d window, got %q", body.Window)
	}
	if body.Totals.Requests != 1 || body.Totals.TotalTokens != 70 {
		t.Fatalf("expected only target-service-account usage, got %#v", body.Totals)
	}
	if body.TotalDurationMs != 750 {
		t.Fatalf("expected duration 750, got %d", body.TotalDurationMs)
	}
	if len(body.Daily) != 7 {
		t.Fatalf("expected seven daily buckets, got %d", len(body.Daily))
	}
}

// TestHandleAdminServiceAccountStatisticsDefaultsToThirtyDays verifies the omitted-window API default.
func TestHandleAdminServiceAccountStatisticsDefaultsToThirtyDays(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-accounts/"+statisticsServiceUserID+"/statistics", nil)
	req.SetPathValue("id", statisticsServiceUserID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminServiceAccountStatistics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Window != proxy.UsageWindow30Days || len(body.Daily) != 30 {
		t.Fatalf("expected default 30d response, got window %q with %d buckets", body.Window, len(body.Daily))
	}
}

// TestHandleAdminServiceAccountStatisticsSupportsAllTime verifies the all-time window is forwarded.
func TestHandleAdminServiceAccountStatisticsSupportsAllTime(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	seedDashboardUsage(
		t,
		db,
		statisticsServiceUserID,
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		time.Now().UTC().Add(-48*time.Hour),
		12,
		18,
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-accounts/"+statisticsServiceUserID+"/statistics?window=all", nil)
	req.SetPathValue("id", statisticsServiceUserID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminServiceAccountStatistics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Window != proxy.UsageWindowAll || body.Totals.TotalTokens != 30 {
		t.Fatalf("expected all-time service-account usage, got %#v", body)
	}
}

// TestHandleAdminServiceAccountStatisticsRejectsInvalidWindow verifies invalid windows use the dashboard API error.
func TestHandleAdminServiceAccountStatisticsRejectsInvalidWindow(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-accounts/"+statisticsServiceUserID+"/statistics?window=14d", nil)
	req.SetPathValue("id", statisticsServiceUserID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminServiceAccountStatistics(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "invalid_usage_window" {
		t.Fatalf("expected invalid_usage_window response, got %#v", body)
	}
}

// TestHandleAdminServiceAccountStatisticsHidesUnknownAndHumanUsers verifies only service accounts are addressable.
func TestHandleAdminServiceAccountStatisticsHidesUnknownAndHumanUsers(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	for _, id := range []string{"missing-service-account", statisticsHumanUserID} {
		t.Run(id, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-accounts/"+id+"/statistics?window=7d", nil)
			req.SetPathValue("id", id)
			recorder := httptest.NewRecorder()

			handler.HandleAdminServiceAccountStatistics(recorder, req)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]string
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != "service_account_not_found" {
				t.Fatalf("expected service_account_not_found response, got %#v", body)
			}
		})
	}
}

// TestHandleAdminServiceAccountStatisticsRejectsMissingDependencies verifies unavailable services return server errors.
func TestHandleAdminServiceAccountStatisticsRejectsMissingDependencies(t *testing.T) {
	completeHandler, _ := newDashboardTestHandler(t)
	tests := []struct {
		name    string
		handler *Handler
		error   string
	}{
		{name: "users", handler: NewHandler(Dependencies{Proxy: completeHandler.proxy}), error: "user service unavailable"},
		{name: "proxy", handler: NewHandler(Dependencies{Users: completeHandler.users}), error: "proxy usage service unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-accounts/"+statisticsServiceUserID+"/statistics?window=7d", nil)
			req.SetPathValue("id", statisticsServiceUserID)
			recorder := httptest.NewRecorder()

			tt.handler.HandleAdminServiceAccountStatistics(recorder, req)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d: %s", recorder.Code, recorder.Body.String())
			}
			var body map[string]string
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != tt.error {
				t.Fatalf("expected %q response, got %#v", tt.error, body)
			}
		})
	}
}
