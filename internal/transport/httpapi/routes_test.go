package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/transport/httpapi/admin"
	"promptgate/backend/internal/transport/httpmiddleware"
)

func TestRouteInventoryIsCompleteAndUnique(t *testing.T) {
	srv := &server{}
	adminHandler := admin.NewHandler(admin.Dependencies{})
	groups := []struct {
		name   string
		routes []routeDefinition
		want   int
	}{
		{name: "public", routes: publicRoutes(srv), want: 5},
		{name: "session", routes: sessionRoutes(), want: 1},
		{name: "user", routes: userRoutes(srv), want: 17},
		{name: "admin", routes: adminRoutes(adminHandler), want: 105},
	}

	seen := make(map[string]string, 128)
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

	if len(seen) != 128 {
		t.Errorf("route inventory: got %d routes, want 128", len(seen))
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
