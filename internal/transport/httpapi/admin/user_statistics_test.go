package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/proxy"
)

const (
	statisticsHumanUserID   = "11111111-1111-1111-1111-111111111111"
	statisticsServiceUserID = "22222222-2222-2222-2222-222222222222"
)

// TestHandleAdminUserStatisticsReturnsOnlyTargetUserUsage verifies the response shape and user isolation.
func TestHandleAdminUserStatisticsReturnsOnlyTargetUserUsage(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	at := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, statisticsHumanUserID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", at, 10, 20)
	seedDashboardUsage(t, db, statisticsServiceUserID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", at, 30, 40)
	if err := db.Model(&proxy.ProxyDailyUsageKPI{}).
		Where("initiator_id = ?", statisticsHumanUserID).
		Update("total_duration_ms", 250).Error; err != nil {
		t.Fatalf("seed duration: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+statisticsHumanUserID+"/statistics?window=7d", nil)
	req.SetPathValue("id", statisticsHumanUserID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminUserStatistics(recorder, req)

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
	if body.Totals.Requests != 1 || body.Totals.TotalTokens != 30 {
		t.Fatalf("expected only target-user usage, got %#v", body.Totals)
	}
	if body.TotalDurationMs != 250 {
		t.Fatalf("expected duration 250, got %d", body.TotalDurationMs)
	}
	if len(body.Daily) != 7 {
		t.Fatalf("expected seven daily buckets, got %d", len(body.Daily))
	}
}

// TestHandleAdminUserStatisticsDefaultsToThirtyDays verifies the omitted-window API default.
func TestHandleAdminUserStatisticsDefaultsToThirtyDays(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+statisticsHumanUserID+"/statistics", nil)
	req.SetPathValue("id", statisticsHumanUserID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminUserStatistics(recorder, req)

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

// TestHandleAdminUserStatisticsRejectsInvalidWindow verifies invalid windows use the dashboard API error.
func TestHandleAdminUserStatisticsRejectsInvalidWindow(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+statisticsHumanUserID+"/statistics?window=14d", nil)
	req.SetPathValue("id", statisticsHumanUserID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminUserStatistics(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "invalid_usage_window") {
		t.Fatalf("expected invalid_usage_window response, got %s", recorder.Body.String())
	}
}

// TestHandleAdminUserStatisticsHidesUnknownAndServiceUsers verifies only human users are addressable.
func TestHandleAdminUserStatisticsHidesUnknownAndServiceUsers(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	for _, id := range []string{"missing-user", statisticsServiceUserID} {
		t.Run(id, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+id+"/statistics?window=7d", nil)
			req.SetPathValue("id", id)
			recorder := httptest.NewRecorder()

			handler.HandleAdminUserStatistics(recorder, req)

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "user_not_found") {
				t.Fatalf("expected user_not_found response, got %s", recorder.Body.String())
			}
		})
	}
}

// TestHandleAdminUserStatisticsRejectsMissingDependencies verifies unavailable services return server errors.
func TestHandleAdminUserStatisticsRejectsMissingDependencies(t *testing.T) {
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
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+statisticsHumanUserID+"/statistics?window=7d", nil)
			req.SetPathValue("id", statisticsHumanUserID)
			recorder := httptest.NewRecorder()

			tt.handler.HandleAdminUserStatistics(recorder, req)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d: %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.error) {
				t.Fatalf("expected %q response, got %s", tt.error, recorder.Body.String())
			}
		})
	}
}
