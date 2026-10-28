package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/transport/httpapi/admin"
	openapidoc "promptgate/backend/internal/transport/httpapi/openapi"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestRuntimeOpenAPIDocumentMatchesAssembledAPIRoutesBidirectionally(t *testing.T) {
	documented := runtimeOpenAPIRoutes(t)
	registered := assembledAPIRoutes(t)

	missingFromDocument := routeSetDifference(registered, documented)
	missingFromRouter := routeSetDifference(documented, registered)
	if len(missingFromDocument) != 0 || len(missingFromRouter) != 0 {
		t.Fatalf(
			"OpenAPI/router parity mismatch:\nmissing from document: %v\nmissing from router: %v",
			missingFromDocument,
			missingFromRouter,
		)
	}
}

// assembledAPIRoutes uses the same route assembly functions consumed by
// NewHandler. Unlike source scanning, it ignores dead routeDefinition literals
// that are not returned by one of the registered route groups.
func assembledAPIRoutes(t *testing.T) map[string]struct{} {
	t.Helper()

	srv := &server{}
	adminHandler := admin.NewHandler(admin.Dependencies{})
	openAPIHandler := func(http.ResponseWriter, *http.Request) {}
	routeGroups := [][]routeDefinition{
		publicRoutes(srv),
		sessionRoutes(openAPIHandler),
		limitedRoutes(srv),
		userRoutes(srv),
		userModelLabRoutes(srv),
		adminRoutes(adminHandler, openAPIHandler),
	}

	routes := make(map[string]struct{})
	for _, group := range routeGroups {
		for _, route := range group {
			_, path, ok := strings.Cut(route.pattern, " ")
			if !ok || !strings.HasPrefix(path, "/api/v1/") {
				continue
			}
			if _, duplicate := routes[route.pattern]; duplicate {
				t.Errorf("duplicate assembled route %s", route.pattern)
			}
			routes[route.pattern] = struct{}{}
		}
	}
	return routes
}

// runtimeOpenAPIRoutes loads the document through its production handler so
// the comparison covers the embedded source plus runtime transformation.
func runtimeOpenAPIRoutes(t *testing.T) map[string]struct{} {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	openapidoc.NewHandler(config.APIConfig{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("serve runtime OpenAPI document: status %d: %s", recorder.Code, recorder.Body.String())
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	document, err := loader.LoadFromData(recorder.Body.Bytes())
	if err != nil {
		t.Fatalf("load runtime OpenAPI document: %v", err)
	}

	routes := make(map[string]struct{})
	for path, item := range document.Paths.Map() {
		for method := range item.Operations() {
			pattern := strings.ToUpper(method) + " " + path
			if _, duplicate := routes[pattern]; duplicate {
				t.Errorf("duplicate documented route %s", pattern)
			}
			routes[pattern] = struct{}{}
		}
	}
	return routes
}

func routeSetDifference(left, right map[string]struct{}) []string {
	difference := make([]string, 0)
	for value := range left {
		if _, ok := right[value]; !ok {
			difference = append(difference, value)
		}
	}
	sort.Strings(difference)
	return difference
}
