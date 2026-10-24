package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/proxy"
	tokenDomain "promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newDashboardTestHandler creates dashboard test handler.
func newDashboardTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	userService := users.NewService(db)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate users table: %v", err)
	}
	if err := proxy.AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("auto-migrate proxy tables: %v", err)
	}
	if err := db.AutoMigrate(&tokenDomain.Token{}); err != nil {
		t.Fatalf("auto-migrate tokens table: %v", err)
	}

	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	for _, user := range []users.User{
		{
			ID:                "11111111-1111-1111-1111-111111111111",
			ExternalSub:       testPointer("sub-1"),
			Email:             "one@example.com",
			PreferredUsername: "one",
			Name:              "One",
			Type:              auth.UserTypeUser,
			Role:              auth.RoleUser,
			IsActive:          true,
			LastLoginAt:       testPointer(now),
		},
		{
			ID:                "22222222-2222-2222-2222-222222222222",
			ExternalSub:       testPointer("service:worker"),
			PreferredUsername: "worker",
			Name:              "Worker",
			Type:              auth.UserTypeService,
			Role:              auth.RoleUser,
			IsActive:          true,
			LastLoginAt:       testPointer(now),
		},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	return NewHandler(Dependencies{Users: userService, Proxy: proxy.NewService(db)}), db
}

// seedDashboardUsage seeds dashboard usage.
func seedDashboardUsage(t *testing.T, db *gorm.DB, userID string, id string, at time.Time, inputTokens int64, outputTokens int64) {
	t.Helper()
	createdAt := time.Now().UTC()
	day := time.Date(at.UTC().Year(), at.UTC().Month(), at.UTC().Day(), 0, 0, 0, 0, time.UTC)
	totalTokens := inputTokens + outputTokens
	if err := db.Create(&proxy.Interception{
		ID:           id,
		InitiatorID:  userID,
		Provider:     "openai",
		ProviderType: "openai",
		Model:        "gpt-5",
		StartedAt:    at,
		Metadata:     "{}",
	}).Error; err != nil {
		t.Fatalf("seed interception: %v", err)
	}
	if err := db.Create(&proxy.TokenUsage{
		InterceptionID:     id,
		ProviderResponseID: "response-" + id,
		InputTokens:        inputTokens,
		OutputTokens:       outputTokens,
		Metadata:           "{}",
		CreatedAt:          at.Add(time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}
	if err := db.Create(&proxy.ProxyDailyUsageKPI{
		Day:                    day,
		InitiatorID:            userID,
		Requests:               1,
		InputTokens:            inputTokens,
		OutputTokens:           outputTokens,
		CompletionInputTokens:  inputTokens,
		CompletionOutputTokens: outputTokens,
		CompletionTokens:       totalTokens,
		TotalTokens:            totalTokens,
		CreatedAt:              createdAt,
		UpdatedAt:              createdAt,
	}).Error; err != nil {
		t.Fatalf("seed dashboard kpi: %v", err)
	}
}

// TestHandleAdminDashboardTokensReturnsGlobalResult verifies handle admin dashboard tokens returns global result.
func TestHandleAdminDashboardTokensReturnsGlobalResult(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	now := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", now, 10, 20)
	seedDashboardUsage(t, db, "22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", now, 30, 40)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/tokens?window=7d", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminDashboardTokens(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardTokensResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TotalTokens != 100 {
		t.Fatalf("expected global token total, got %#v", body)
	}
}

// TestHandleAdminDashboardAdoptionReturnsResult verifies handle admin dashboard adoption returns result.
func TestHandleAdminDashboardAdoptionReturnsResult(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	now := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", now, 10, 20)
	seedDashboardUsage(t, db, "22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", now, 30, 40)
	if err := db.Create(&tokenDomain.Token{
		UserID:      "11111111-1111-1111-1111-111111111111",
		Name:        "active",
		TokenHash:   "active-hash",
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		Description: "",
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/adoption?window=7d", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminDashboardAdoption(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardAdoptionResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.ActiveUsers != 1 || body.ActiveServiceAccounts != 1 || body.ActiveVirtualKeys != 1 {
		t.Fatalf("unexpected adoption response: %#v", body)
	}
}

// TestHandleAdminDashboardOverviewReturnsUsageAndAdoption verifies the global
// cockpit endpoint combines period usage, comparison, and adoption.
func TestHandleAdminDashboardOverviewReturnsUsageAndAdoption(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	now := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", now, 10, 20)
	seedDashboardUsage(t, db, "22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", now, 30, 40)
	if err := db.Create(&tokenDomain.Token{
		UserID:      "11111111-1111-1111-1111-111111111111",
		Name:        "active",
		TokenHash:   "overview-active-hash",
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		Description: "",
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/overview?window=7d", nil)
	recorder := httptest.NewRecorder()
	handler.HandleAdminDashboardOverview(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardOverviewResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Totals.Requests != 2 || body.Totals.TotalTokens != 100 || body.Previous == nil {
		t.Fatalf("unexpected global usage response: %#v", body)
	}
	if body.Adoption == nil || body.Adoption.ActiveUsers != 1 || body.Adoption.ActiveServiceAccounts != 1 || body.Adoption.ActiveVirtualKeys != 1 {
		t.Fatalf("unexpected global adoption response: %#v", body.Adoption)
	}
	if body.CostCoverage == nil || body.CostCoverage.Status != proxy.CostCoverageComplete {
		t.Fatalf("unexpected global cost coverage: %#v", body.CostCoverage)
	}
}

// TestHandleAdminDashboardTopIdentitiesRanksByRequests verifies the query
// metric changes the actual backend top-five ordering.
func TestHandleAdminDashboardTopIdentitiesRanksByRequests(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	now := time.Now().UTC()
	userID := "11111111-1111-1111-1111-111111111111"
	serviceID := "22222222-2222-2222-2222-222222222222"
	seedDashboardUsage(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1", now.Add(-time.Hour), 1, 1)
	seedDashboardUsage(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2", now.Add(-25*time.Hour), 1, 1)
	seedDashboardUsage(t, db, serviceID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1", now.Add(-2*time.Hour), 100, 100)

	request := func(path string) proxy.DashboardBreakdownResponse {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler.HandleAdminDashboardTopIdentities(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var body proxy.DashboardBreakdownResponse
		if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		return body
	}

	byTokens := request("/api/v1/admin/dashboard/top-identities?window=7d")
	if len(byTokens.Items) != 2 || byTokens.Items[0].Name != "Worker" {
		t.Fatalf("expected default token ranking, got %#v", byTokens.Items)
	}
	byRequests := request("/api/v1/admin/dashboard/top-identities?window=7d&metric=requests")
	if len(byRequests.Items) != 2 || byRequests.Items[0].Name != "One" || byRequests.Items[0].Requests != 2 {
		t.Fatalf("expected request ranking, got %#v", byRequests.Items)
	}
}

func TestHandleAdminDashboardTopToolsReturnsGlobalCalls(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	now := time.Now().UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	serviceID := "22222222-2222-2222-2222-222222222222"
	for _, row := range []proxy.ProxyDailyUsageKPI{
		{Day: day, InitiatorID: userID, ToolCalls: 2, CreatedAt: now, UpdatedAt: now},
		{Day: day, InitiatorID: serviceID, ToolCalls: 5, CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed tool total: %v", err)
		}
	}
	for _, row := range []proxy.ProxyDailyUsageBreakdown{
		{Day: day, InitiatorID: userID, Dimension: "tools", Name: "github.search", Requests: 2, CreatedAt: now, UpdatedAt: now},
		{Day: day, InitiatorID: serviceID, Dimension: "tools", Name: "linear.list", Requests: 4, CreatedAt: now, UpdatedAt: now},
		{Day: day, InitiatorID: serviceID, Dimension: "tools", Name: "github.search", Requests: 1, CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed tool breakdown: %v", err)
		}
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/top-tools?window=7d", nil)
	handler.HandleAdminDashboardTopTools(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardTopToolsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TotalCalls != 7 || body.AttributedCalls != 7 || len(body.Items) != 2 {
		t.Fatalf("unexpected global tool response: %#v", body)
	}
	if body.Items[0].Name != "linear.list" || body.Items[0].Calls != 4 || body.Items[1].Name != "github.search" || body.Items[1].Calls != 3 {
		t.Fatalf("unexpected global tool ranking: %#v", body.Items)
	}
}

// TestHandleAdminDashboardBreakdownRejectsInvalidMetric verifies clients get a
// stable, explicit 400 rather than an arbitrary fallback.
func TestHandleAdminDashboardBreakdownRejectsInvalidMetric(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/top-models?metric=latency", nil)
	handler.HandleAdminDashboardTopModels(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "invalid_dashboard_metric" {
		t.Fatalf("unexpected error response: %#v", body)
	}
}

// TestHandleAdminDashboardRejectsInvalidWindow verifies handle admin dashboard rejects invalid window.
func TestHandleAdminDashboardRejectsInvalidWindow(t *testing.T) {
	for _, window := range []string{"14d", "365d"} {
		t.Run(window, func(t *testing.T) {
			handler, _ := newDashboardTestHandler(t)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/tokens?window="+window, nil)
			recorder := httptest.NewRecorder()

			handler.HandleAdminDashboardTokens(recorder, req)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
