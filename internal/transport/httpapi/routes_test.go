package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/transport/httpapi/admin"
	"promptgate/backend/internal/transport/httpmiddleware"
)

func TestRouteInventoryIsCompleteAndUnique(t *testing.T) {
	srv := &server{}
	adminHandler := admin.NewHandler(admin.Dependencies{})
	openAPIHandler := func(http.ResponseWriter, *http.Request) {}
	groups := []struct {
		name   string
		routes []routeDefinition
		want   int
	}{
		{name: "public", routes: publicRoutes(srv), want: 5},
		{name: "session", routes: sessionRoutes(openAPIHandler), want: 2},
		{name: "limited", routes: limitedRoutes(srv), want: 16},
		{name: "user", routes: userRoutes(srv), want: 5},
		{name: "model-lab", routes: userModelLabRoutes(srv), want: 2},
		{name: "admin", routes: adminRoutes(adminHandler, openAPIHandler), want: 93},
	}

	seen := make(map[string]string, 123)
	for _, group := range groups {
		if len(group.routes) != group.want {
			t.Errorf("%s routes: got %d, want %d", group.name, len(group.routes), group.want)
		}
		for _, route := range group.routes {
			if route.handler == nil {
				t.Errorf("%s route %q has no handler", group.name, route.pattern)
			}
			method, path, ok := strings.Cut(route.pattern, " ")
			if !ok || path == "" || !strings.HasPrefix(path, "/") || !isSupportedRouteMethod(method) {
				t.Errorf("%s route has invalid pattern %q", group.name, route.pattern)
			}
			if previousGroup, ok := seen[route.pattern]; ok {
				t.Errorf("route %q is declared in both %s and %s", route.pattern, previousGroup, group.name)
			}
			seen[route.pattern] = group.name
		}
	}

	if len(seen) != 123 {
		t.Errorf("route inventory: got %d routes, want 123", len(seen))
	}
	for _, removed := range []string{
		"GET /api/v1/me/prompts",
		"GET /api/v1/admin/prompts",
		"GET /api/v1/admin/mcp/servers",
		"POST /api/v1/admin/mcp/servers",
		"GET /api/v1/admin/mcp/servers/{id}",
		"PATCH /api/v1/admin/mcp/servers/{id}",
		"DELETE /api/v1/admin/mcp/servers/{id}",
	} {
		if _, exists := seen[removed]; exists {
			t.Errorf("removed route %q is still registered", removed)
		}
	}
}

func TestRemovedMCPRoutesReturnNotFound(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux, adminRoutes(admin.NewHandler(admin.Dependencies{}), func(http.ResponseWriter, *http.Request) {}))

	for _, request := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/admin/mcp/servers"},
		{method: http.MethodPost, path: "/api/v1/admin/mcp/servers"},
		{method: http.MethodGet, path: "/api/v1/admin/mcp/servers/server-id"},
		{method: http.MethodPatch, path: "/api/v1/admin/mcp/servers/server-id"},
		{method: http.MethodDelete, path: "/api/v1/admin/mcp/servers/server-id"},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s %s: got %d, want %d", request.method, request.path, recorder.Code, http.StatusNotFound)
		}
	}
}

func TestQuotaResetTrackingRoutesRequireAdminAccess(t *testing.T) {
	const (
		cookieName = "quota_reset_session"
		adminKey   = "quota-reset-admin-key"
	)
	sessions := auth.NewSessionStore(nil, time.Hour)
	limitedSession := createOpenAPITestSession(t, sessions, auth.RoleLimited)
	adminSession := createOpenAPITestSession(t, sessions, auth.RoleAdmin)
	cfg := config.APIConfig{
		SessionConfig: config.SessionConfig{SessionCookieName: cookieName},
		APIHTTPConfig: config.APIHTTPConfig{AdminAPIKey: adminKey},
	}
	mux := http.NewServeMux()
	registerRoutes(
		mux,
		adminQuotaResetRoutes(admin.NewHandler(admin.Dependencies{})),
		adminAccessMiddlewares(sessions, cfg)...,
	)

	for _, path := range []string{
		"/api/v1/admin/quota-resets/quota-states",
		"/api/v1/admin/quota-resets/events",
	} {
		for _, test := range []struct {
			name       string
			sessionID  string
			adminKey   string
			wantStatus int
		}{
			{name: "missing credentials", wantStatus: http.StatusUnauthorized},
			{name: "limited session", sessionID: limitedSession.ID, wantStatus: http.StatusForbidden},
			{name: "admin session", sessionID: adminSession.ID, wantStatus: http.StatusInternalServerError},
			{name: "admin API key", adminKey: adminKey, wantStatus: http.StatusInternalServerError},
		} {
			t.Run(path+" "+test.name, func(t *testing.T) {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				if test.sessionID != "" {
					request.AddCookie(&http.Cookie{Name: cookieName, Value: test.sessionID})
				}
				if test.adminKey != "" {
					request.Header.Set(middleware.AdminAPIKeyHeader, test.adminKey)
				}
				recorder := httptest.NewRecorder()
				mux.ServeHTTP(recorder, request)
				if recorder.Code != test.wantStatus {
					t.Fatalf("expected %d, got %d: %s", test.wantStatus, recorder.Code, recorder.Body.String())
				}
			})
		}
	}
}

func TestLimitedRouteBoundary(t *testing.T) {
	srv := &server{}
	limited := make(map[string]bool)
	for _, route := range limitedRoutes(srv) {
		limited[route.pattern] = true
	}
	standard := make(map[string]bool)
	for _, route := range userRoutes(srv) {
		standard[route.pattern] = true
	}
	modelLab := make(map[string]bool)
	for _, route := range userModelLabRoutes(srv) {
		modelLab[route.pattern] = true
	}

	for _, allowed := range []string{
		"GET /api/v1/me/usage",
		"GET /api/v1/me/quota",
		"GET /api/v1/me/quota-resets",
		"POST /api/v1/me/quota-resets/use",
		"GET /api/v1/me/quota-resets/history",
		"GET /api/v1/me/groups",
		"GET /api/v1/me/dashboard/overview",
		"GET /api/v1/me/dashboard/tokens",
		"GET /api/v1/me/dashboard/top-tools",
		"GET /api/v1/faq",
	} {
		if !limited[allowed] {
			t.Errorf("expected limited route %q", allowed)
		}
	}
	for _, forbidden := range []string{
		"GET /api/v1/me/help/setup",
		"GET /api/v1/monitoring/status",
		"GET /api/v1/tokens",
		"POST /api/v1/tokens",
		"DELETE /api/v1/tokens/{id}",
	} {
		if limited[forbidden] || !standard[forbidden] {
			t.Errorf("expected standard-only route %q", forbidden)
		}
	}
	for _, restricted := range []string{
		"GET /api/v1/me/models",
		"POST /api/v1/me/model-lab/tickets",
	} {
		if limited[restricted] || standard[restricted] || !modelLab[restricted] {
			t.Errorf("expected metadata-only Model Lab route %q", restricted)
		}
	}
}

func TestRegisterRoutesPreservesMiddlewareOrder(t *testing.T) {
	var calls []string
	marker := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, name+":before")
				next.ServeHTTP(w, r)
				calls = append(calls, name+":after")
			})
		}
	}
	mux := http.NewServeMux()
	registerRoutes(mux, []routeDefinition{{
		pattern: "GET /probe",
		handler: func(w http.ResponseWriter, _ *http.Request) {
			calls = append(calls, "handler")
			w.WriteHeader(http.StatusNoContent)
		},
	}}, marker("first"), marker("second"))

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/probe", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusNoContent)
	}
	want := []string{"first:before", "second:before", "handler", "second:after", "first:after"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls: got %v, want %v", calls, want)
	}
}

func isSupportedRouteMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
