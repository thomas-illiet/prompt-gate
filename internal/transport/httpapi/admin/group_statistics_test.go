package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/users"

	"gorm.io/gorm"
)

const statisticsOutsideGroupUserID = "33333333-3333-3333-3333-333333333333"

// newGroupStatisticsTestHandler creates an admin handler with group and usage dependencies.
func newGroupStatisticsTestHandler(t *testing.T) (*Handler, *gorm.DB, *groups.Service) {
	t.Helper()
	baseHandler, db := newDashboardTestHandler(t)
	if err := db.AutoMigrate(&provider.Provider{}); err != nil {
		t.Fatalf("auto-migrate provider table: %v", err)
	}
	groupService := groups.NewService(db)
	if err := groupService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate groups: %v", err)
	}
	return NewHandler(Dependencies{Groups: groupService, Proxy: baseHandler.proxy}), db, groupService
}

// createStatisticsGroup creates one group and assigns its current members.
func createStatisticsGroup(t *testing.T, db *gorm.DB, service *groups.Service, memberIDs ...string) groups.Group {
	t.Helper()
	group := groups.Group{Name: "statistics", DisplayName: "Statistics"}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	for _, memberID := range memberIDs {
		if err := service.AddMember(context.Background(), group.ID.String(), memberID); err != nil {
			t.Fatalf("add group member %s: %v", memberID, err)
		}
	}
	return group
}

// createStatisticsOutsideUser creates an identity that is not a group member.
func createStatisticsOutsideUser(t *testing.T, db *gorm.DB) {
	t.Helper()
	user := users.User{
		ID:                statisticsOutsideGroupUserID,
		ExternalSub:       "outside-group",
		Email:             "outside@example.com",
		PreferredUsername: "outside",
		Name:              "Outside",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create outside user: %v", err)
	}
}

// TestHandleAdminGroupStatisticsAggregatesCurrentMembers verifies human and
// service-account members are combined while non-members remain isolated.
func TestHandleAdminGroupStatisticsAggregatesCurrentMembers(t *testing.T) {
	handler, db, groupService := newGroupStatisticsTestHandler(t)
	createStatisticsOutsideUser(t, db)
	at := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, statisticsHumanUserID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", at, 10, 20)
	seedDashboardUsage(t, db, statisticsServiceUserID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", at, 30, 40)
	seedDashboardUsage(t, db, statisticsOutsideGroupUserID, "cccccccc-cccc-cccc-cccc-cccccccccccc", at, 50, 60)
	if err := db.Model(&proxy.ProxyDailyUsageKPI{}).
		Where("initiator_id = ?", statisticsHumanUserID).
		Update("total_duration_ms", 250).Error; err != nil {
		t.Fatalf("seed human duration: %v", err)
	}
	if err := db.Model(&proxy.ProxyDailyUsageKPI{}).
		Where("initiator_id = ?", statisticsServiceUserID).
		Update("total_duration_ms", 750).Error; err != nil {
		t.Fatalf("seed service duration: %v", err)
	}
	group := createStatisticsGroup(t, db, groupService, statisticsHumanUserID, statisticsServiceUserID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/"+group.ID.String()+"/statistics?window=7d", nil)
	req.SetPathValue("id", group.ID.String())
	recorder := httptest.NewRecorder()
	handler.HandleAdminGroupStatistics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Window != proxy.UsageWindow7Days || len(body.Daily) != 7 {
		t.Fatalf("unexpected group window: %#v", body.UsageWindowMeta)
	}
	if body.Totals.Requests != 2 || body.Totals.TotalTokens != 100 {
		t.Fatalf("expected only current member usage, got %#v", body.Totals)
	}
	if body.TotalDurationMs != 1000 {
		t.Fatalf("expected current member duration 1000, got %d", body.TotalDurationMs)
	}
}

// TestHandleAdminGroupStatisticsUsesMembershipAtRequestTime verifies removing a
// member also removes that identity's earlier usage from the group cohort.
func TestHandleAdminGroupStatisticsUsesMembershipAtRequestTime(t *testing.T) {
	handler, db, groupService := newGroupStatisticsTestHandler(t)
	at := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, statisticsHumanUserID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", at, 10, 20)
	seedDashboardUsage(t, db, statisticsServiceUserID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", at, 30, 40)
	group := createStatisticsGroup(t, db, groupService, statisticsHumanUserID, statisticsServiceUserID)
	if err := groupService.RemoveMember(context.Background(), group.ID.String(), statisticsServiceUserID); err != nil {
		t.Fatalf("remove service-account member: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/"+group.ID.String()+"/statistics?window=7d", nil)
	req.SetPathValue("id", group.ID.String())
	recorder := httptest.NewRecorder()
	handler.HandleAdminGroupStatistics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Totals.Requests != 1 || body.Totals.TotalTokens != 30 {
		t.Fatalf("expected only the remaining current member's usage, got %#v", body.Totals)
	}
}

// TestHandleAdminGroupStatisticsEmptyGroupDoesNotReturnGlobalUsage verifies an
// empty current-member cohort produces zero totals rather than global totals.
func TestHandleAdminGroupStatisticsEmptyGroupDoesNotReturnGlobalUsage(t *testing.T) {
	handler, db, groupService := newGroupStatisticsTestHandler(t)
	seedDashboardUsage(
		t,
		db,
		statisticsHumanUserID,
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		time.Now().UTC().Add(-time.Hour),
		10,
		20,
	)
	group := createStatisticsGroup(t, db, groupService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/"+group.ID.String()+"/statistics?window=7d", nil)
	req.SetPathValue("id", group.ID.String())
	recorder := httptest.NewRecorder()
	handler.HandleAdminGroupStatistics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Totals.Requests != 0 || body.Totals.TotalTokens != 0 || body.TotalDurationMs != 0 {
		t.Fatalf("empty group leaked global usage: %#v", body)
	}
}

// TestHandleAdminGroupStatisticsDefaultsToThirtyDays verifies the omitted-window API default.
func TestHandleAdminGroupStatisticsDefaultsToThirtyDays(t *testing.T) {
	handler, db, groupService := newGroupStatisticsTestHandler(t)
	group := createStatisticsGroup(t, db, groupService, statisticsHumanUserID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/"+group.ID.String()+"/statistics", nil)
	req.SetPathValue("id", group.ID.String())
	recorder := httptest.NewRecorder()

	handler.HandleAdminGroupStatistics(recorder, req)

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

// TestHandleAdminGroupStatisticsSupportsAllTime verifies the earliest current-member activity sets the range.
func TestHandleAdminGroupStatisticsSupportsAllTime(t *testing.T) {
	handler, db, groupService := newGroupStatisticsTestHandler(t)
	at := time.Now().UTC().Add(-48 * time.Hour)
	seedDashboardUsage(t, db, statisticsHumanUserID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", at, 12, 18)
	group := createStatisticsGroup(t, db, groupService, statisticsHumanUserID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/"+group.ID.String()+"/statistics?window=all", nil)
	req.SetPathValue("id", group.ID.String())
	recorder := httptest.NewRecorder()

	handler.HandleAdminGroupStatistics(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Window != proxy.UsageWindowAll || body.Totals.TotalTokens != 30 || len(body.Daily) < 2 {
		t.Fatalf("unexpected all-time group usage: %#v", body)
	}
	if body.Daily[0].Date != at.UTC().Format("2006-01-02") {
		t.Fatalf("expected first member activity on %s, got %#v", at.UTC().Format("2006-01-02"), body.Daily)
	}
}

// TestHandleAdminGroupStatisticsRejectsInvalidWindow verifies invalid windows use the dashboard API error.
func TestHandleAdminGroupStatisticsRejectsInvalidWindow(t *testing.T) {
	handler, db, groupService := newGroupStatisticsTestHandler(t)
	group := createStatisticsGroup(t, db, groupService, statisticsHumanUserID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/"+group.ID.String()+"/statistics?window=14d", nil)
	req.SetPathValue("id", group.ID.String())
	recorder := httptest.NewRecorder()

	handler.HandleAdminGroupStatistics(recorder, req)

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

// TestHandleAdminGroupStatisticsRejectsMissingGroup verifies unknown group IDs stay hidden behind the group error contract.
func TestHandleAdminGroupStatisticsRejectsMissingGroup(t *testing.T) {
	handler, _, _ := newGroupStatisticsTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/missing/statistics?window=7d", nil)
	req.SetPathValue("id", "missing")
	recorder := httptest.NewRecorder()

	handler.HandleAdminGroupStatistics(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "group_not_found" {
		t.Fatalf("expected group_not_found response, got %#v", body)
	}
}

// TestHandleAdminGroupStatisticsRejectsMissingDependencies verifies unavailable services return server errors.
func TestHandleAdminGroupStatisticsRejectsMissingDependencies(t *testing.T) {
	completeHandler, _, _ := newGroupStatisticsTestHandler(t)
	tests := []struct {
		name    string
		handler *Handler
		error   string
	}{
		{name: "groups", handler: NewHandler(Dependencies{Proxy: completeHandler.proxy}), error: "group service unavailable"},
		{name: "proxy", handler: NewHandler(Dependencies{Groups: completeHandler.groups}), error: "proxy usage service unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/missing/statistics?window=7d", nil)
			req.SetPathValue("id", "missing")
			recorder := httptest.NewRecorder()

			tt.handler.HandleAdminGroupStatistics(recorder, req)

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
