package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/proxy"
)

func newCurrentUserDashboardTestServer(t *testing.T) (server, auth.UserProfile) {
	t.Helper()
	_, db := newHTTPGroupService(t)
	if err := proxy.AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy usage: %v", err)
	}
	profile := testUserProfile()
	createHTTPUser(t, db, profile)
	return server{proxyService: proxy.NewService(db)}, profile
}

func currentUserDashboardRequest(method, target string, profile auth.UserProfile) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	return request.WithContext(auth.ContextWithUser(context.Background(), profile))
}

func TestCurrentUserDashboardHandlersReturnEmptyResults(t *testing.T) {
	srv, profile := newCurrentUserDashboardTestServer(t)
	tests := []struct {
		name   string
		target string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "usage", target: "/api/v1/me/usage?days=7", call: srv.handleCurrentUserUsage},
		{name: "tokens", target: "/api/v1/me/dashboard/tokens?window=7d", call: srv.handleCurrentUserDashboardTokens},
		{name: "messages", target: "/api/v1/me/dashboard/messages?window=30d", call: srv.handleCurrentUserDashboardMessages},
		{name: "duration", target: "/api/v1/me/dashboard/duration?window=all", call: srv.handleCurrentUserDashboardDuration},
		{name: "activity", target: "/api/v1/me/dashboard/activity?window=365d", call: srv.handleCurrentUserDashboardActivity},
		{name: "top models", target: "/api/v1/me/dashboard/top-models?window=7d&metric=requests", call: srv.handleCurrentUserDashboardTopModels},
		{name: "top provider names", target: "/api/v1/me/dashboard/top-provider-names?window=7d&metric=tokens", call: srv.handleCurrentUserDashboardTopProviderNames},
		{name: "top provider types", target: "/api/v1/me/dashboard/top-provider-types?window=7d", call: srv.handleCurrentUserDashboardTopProviderTypes},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, currentUserDashboardRequest(http.MethodGet, tc.target, profile))
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s: %d %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestCurrentUserDashboardHandlersValidateContextAndQueries(t *testing.T) {
	srv, profile := newCurrentUserDashboardTestServer(t)
	tests := []struct {
		name       string
		call       func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantError  string
	}{
		{
			name:       "usage missing user",
			call:       srv.handleCurrentUserUsage,
			request:    httptest.NewRequest(http.MethodGet, "/api/v1/me/usage", nil),
			wantStatus: http.StatusInternalServerError,
			wantError:  "missing user in context",
		},
		{
			name:       "usage service unavailable",
			call:       server{}.handleCurrentUserUsage,
			request:    currentUserDashboardRequest(http.MethodGet, "/api/v1/me/usage", profile),
			wantStatus: http.StatusInternalServerError,
			wantError:  "proxy usage service unavailable",
		},
		{
			name:       "dashboard missing user",
			call:       srv.handleCurrentUserDashboardTokens,
			request:    httptest.NewRequest(http.MethodGet, "/api/v1/me/dashboard/tokens", nil),
			wantStatus: http.StatusInternalServerError,
			wantError:  "missing user in context",
		},
		{
			name:       "dashboard invalid window",
			call:       srv.handleCurrentUserDashboardMessages,
			request:    currentUserDashboardRequest(http.MethodGet, "/api/v1/me/dashboard/messages?window=invalid", profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_usage_window",
		},
		{
			name:       "dashboard service unavailable",
			call:       server{}.handleCurrentUserDashboardDuration,
			request:    currentUserDashboardRequest(http.MethodGet, "/api/v1/me/dashboard/duration", profile),
			wantStatus: http.StatusInternalServerError,
			wantError:  "proxy usage service unavailable",
		},
		{
			name:       "activity missing user",
			call:       srv.handleCurrentUserDashboardActivity,
			request:    httptest.NewRequest(http.MethodGet, "/api/v1/me/dashboard/activity", nil),
			wantStatus: http.StatusInternalServerError,
			wantError:  "missing user in context",
		},
		{
			name:       "activity invalid window",
			call:       srv.handleCurrentUserDashboardActivity,
			request:    currentUserDashboardRequest(http.MethodGet, "/api/v1/me/dashboard/activity?window=invalid", profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_usage_window",
		},
		{
			name:       "activity service unavailable",
			call:       server{}.handleCurrentUserDashboardActivity,
			request:    currentUserDashboardRequest(http.MethodGet, "/api/v1/me/dashboard/activity?window=365d", profile),
			wantStatus: http.StatusInternalServerError,
			wantError:  "proxy usage service unavailable",
		},
		{
			name:       "invalid breakdown metric",
			call:       srv.handleCurrentUserDashboardTopModels,
			request:    currentUserDashboardRequest(http.MethodGet, "/api/v1/me/dashboard/top-models?metric=latency", profile),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_dashboard_metric",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, tc.request)
			if recorder.Code != tc.wantStatus || !strings.Contains(recorder.Body.String(), tc.wantError) {
				t.Fatalf("status/body = %d %s, want %d containing %q", recorder.Code, recorder.Body.String(), tc.wantStatus, tc.wantError)
			}
		})
	}
}

func TestDashboardErrorAndUsageParsers(t *testing.T) {
	for name, err := range map[string]error{
		"invalid window": proxy.ErrInvalidUsageWindow,
		"invalid metric": proxy.ErrInvalidDashboardMetric,
		"internal":       errors.New("dashboard unavailable"),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			if !writeDashboardError(recorder, err) {
				t.Fatal("expected dashboard error to be handled")
			}
		})
	}
	if writeDashboardError(httptest.NewRecorder(), nil) {
		t.Fatal("nil dashboard error should not be handled")
	}

	for _, tc := range []struct {
		target string
		want   int
	}{
		{target: "/api/v1/me/usage", want: 30},
		{target: "/api/v1/me/usage?days=7", want: 7},
		{target: "/api/v1/me/usage?days=30", want: 30},
	} {
		got, err := parseUsageDays(httptest.NewRequest(http.MethodGet, tc.target, nil))
		if err != nil || got != tc.want {
			t.Fatalf("parse %s: got %d want %d err=%v", tc.target, got, tc.want, err)
		}
	}
	if _, err := parseUsageDays(httptest.NewRequest(http.MethodGet, "/api/v1/me/usage?days=abc", nil)); err == nil {
		t.Fatal("expected non-numeric usage days to fail")
	}
}
