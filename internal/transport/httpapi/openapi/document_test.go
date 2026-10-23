package openapi

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const expectedOperationCount = 110

var expectedTagOrder = []string{
	"Current account",
	"Dashboard",
	"Tokens",
	"Users",
	"Service accounts",
	"Groups",
	"Subscriptions",
	"Providers",
	"Pricing",
	"Monitoring",
	"FAQ",
	"Guides",
	"Documentation",
}

func loadSourceDocument(t *testing.T) ([]byte, *openapi3.T) {
	t.Helper()

	source, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI source: %v", err)
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	document, err := loader.LoadFromData(source)
	if err != nil {
		t.Fatalf("load OpenAPI source: %v", err)
	}
	return source, document
}

func TestSourceDocumentIsValidOpenAPI303(t *testing.T) {
	_, document := loadSourceDocument(t)

	if document.OpenAPI != "3.0.3" {
		t.Fatalf("expected OpenAPI 3.0.3, got %q", document.OpenAPI)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI source: %v", err)
	}
}

func TestSourceDocumentUsesOnlyBrowserSessionSecurity(t *testing.T) {
	_, document := loadSourceDocument(t)

	if got := len(document.Components.SecuritySchemes); got != 1 {
		t.Fatalf("expected exactly one security scheme, got %d", got)
	}
	schemeRef, ok := document.Components.SecuritySchemes[sessionSecurityScheme]
	if !ok || schemeRef == nil || schemeRef.Value == nil {
		t.Fatalf("missing %q security scheme", sessionSecurityScheme)
	}
	scheme := schemeRef.Value
	if scheme.Type != "apiKey" || scheme.In != "cookie" || scheme.Name != "__PROMPTGATE_SESSION_COOKIE_NAME__" {
		t.Fatalf("unexpected session security scheme: type=%q in=%q name=%q", scheme.Type, scheme.In, scheme.Name)
	}

	checkRequirements := func(location string, requirements openapi3.SecurityRequirements) {
		t.Helper()
		if len(requirements) != 1 {
			t.Errorf("%s: expected one security requirement, got %d", location, len(requirements))
			return
		}
		if len(requirements[0]) != 1 {
			t.Errorf("%s: expected session cookie only, got %v", location, requirements[0])
			return
		}
		if _, ok := requirements[0][sessionSecurityScheme]; !ok {
			t.Errorf("%s: expected %q security, got %v", location, sessionSecurityScheme, requirements[0])
		}
	}
	checkRequirements("document", document.Security)
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation.Security != nil {
				checkRequirements(strings.ToUpper(method)+" "+path, *operation.Security)
			}
		}
	}
}

func TestSourceDocumentOperationMetadataAndContracts(t *testing.T) {
	_, document := loadSourceDocument(t)

	actualTags := make([]string, 0, len(document.Tags))
	declaredTags := make(map[string]struct{}, len(document.Tags))
	for _, tag := range document.Tags {
		actualTags = append(actualTags, tag.Name)
		declaredTags[tag.Name] = struct{}{}
	}
	if !slices.Equal(actualTags, expectedTagOrder) {
		t.Fatalf("unexpected tag order:\nwant %v\n got %v", expectedTagOrder, actualTags)
	}

	noBodyMutations := map[string]struct{}{
		"POST /api/v1/admin/monitoring/services/{id}/check": {},
		"POST /api/v1/admin/users/{id}/quota-resets":        {},
		"POST /api/v1/me/quota-resets/use":                  {},
		"PUT /api/v1/admin/subscriptions/{id}/default":      {},
		"PUT /api/v1/admin/groups/{id}/members/{userId}":    {},
	}
	operationIDs := make(map[string]string, expectedOperationCount)
	operationCount := 0
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			operationCount++
			method = strings.ToUpper(method)
			location := method + " " + path

			if operation.OperationID == "" {
				t.Errorf("%s: operationId is required", location)
			} else if previous, exists := operationIDs[operation.OperationID]; exists {
				t.Errorf("%s: duplicate operationId %q already used by %s", location, operation.OperationID, previous)
			} else {
				operationIDs[operation.OperationID] = location
			}
			if len(operation.Tags) != 1 {
				t.Errorf("%s: expected exactly one tag, got %v", location, operation.Tags)
			} else if _, ok := declaredTags[operation.Tags[0]]; !ok {
				t.Errorf("%s: undeclared tag %q", location, operation.Tags[0])
			}
			if operation.Responses == nil || operation.Responses.Len() == 0 {
				t.Errorf("%s: at least one response is required", location)
				continue
			}
			assertStructuredSuccessResponse(t, location, operation.Responses)

			switch method {
			case "POST", "PUT", "PATCH":
				if _, exempt := noBodyMutations[location]; exempt {
					break
				}
				if operation.RequestBody == nil || operation.RequestBody.Value == nil {
					t.Errorf("%s: requestBody is required", location)
					break
				}
				mediaType := operation.RequestBody.Value.Content.Get("application/json")
				if mediaType == nil || mediaType.Schema == nil {
					t.Errorf("%s: requestBody must define an application/json schema", location)
				}
			}
		}
	}
	if operationCount != expectedOperationCount {
		t.Fatalf("expected %d operations, got %d", expectedOperationCount, operationCount)
	}
}

func assertStructuredSuccessResponse(t *testing.T, location string, responses *openapi3.Responses) {
	t.Helper()

	codes := make([]string, 0, responses.Len())
	for code, responseRef := range responses.Map() {
		codes = append(codes, code)
		if !strings.HasPrefix(code, "2") {
			continue
		}
		if responseRef == nil || responseRef.Value == nil {
			t.Errorf("%s: success response %s does not resolve", location, code)
			return
		}
		if code == "204" {
			return
		}
		mediaType := responseRef.Value.Content.Get("application/json")
		if mediaType == nil || mediaType.Schema == nil {
			t.Errorf("%s: success response %s must define an application/json schema", location, code)
		}
		return
	}
	sort.Strings(codes)
	t.Errorf("%s: missing successful response (responses: %s)", location, strings.Join(codes, ", "))
}

func TestSourceDocumentPathParametersAreRequired(t *testing.T) {
	_, document := loadSourceDocument(t)
	placeholderPattern := regexp.MustCompile(`\{([^{}]+)\}`)

	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			location := strings.ToUpper(method) + " " + path
			parameters := append(openapi3.Parameters{}, item.Parameters...)
			parameters = append(parameters, operation.Parameters...)
			for _, match := range placeholderPattern.FindAllStringSubmatch(path, -1) {
				name := match[1]
				found := false
				for _, parameterRef := range parameters {
					if parameterRef == nil || parameterRef.Value == nil {
						continue
					}
					parameter := parameterRef.Value
					if parameter.In == openapi3.ParameterInPath && parameter.Name == name {
						found = true
						if !parameter.Required {
							t.Errorf("%s: path parameter %q must be required", location, name)
						}
					}
				}
				if !found {
					t.Errorf("%s: missing path parameter %q", location, name)
				}
			}
		}
	}
}

func TestSourceDocumentReferencesAreInternal(t *testing.T) {
	source, _ := loadSourceDocument(t)
	referencePattern := regexp.MustCompile(`(?m)\$ref:\s*['\"]?([^'\"\s},]+)`)
	matches := referencePattern.FindAllSubmatch(source, -1)
	if len(matches) == 0 {
		t.Fatal("expected at least one OpenAPI reference")
	}
	for _, match := range matches {
		reference := string(match[1])
		if !strings.HasPrefix(reference, "#/components/") {
			t.Errorf("external or non-component reference is forbidden: %s", reference)
		}
	}

	if t.Failed() {
		t.Logf("checked %d references", len(matches))
	}
}

func TestResponseSchemasAcceptRepresentativePayloads(t *testing.T) {
	_, document := loadSourceDocument(t)
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI source: %v", err)
	}

	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "UsageWindowMeta",
			payload: `{"window":"7d","startsAt":"2026-10-01T00:00:00Z","endsAt":"2026-10-08T00:00:00Z"}`,
		},
		{
			name: "DashboardOverviewPeriod",
			payload: `{
				"window":"7d","startsAt":"2026-10-01T00:00:00Z","endsAt":"2026-10-08T00:00:00Z",
				"totals":{"requests":0,"toolCalls":0,"inputTokens":0,"outputTokens":0,"cacheReadInputTokens":0,"cacheWriteInputTokens":0,"completionInputTokens":0,"completionOutputTokens":0,"completionTokens":0,"embeddingTokens":0,"totalTokens":0},
				"totalDurationMs":0,"averageDurationMs":null,"daily":[]
			}`,
		},
		{
			name: "DashboardOverview",
			payload: `{
				"window":"30d","startsAt":"2026-09-01T00:00:00Z","endsAt":"2026-10-01T00:00:00Z",
				"totals":{"requests":1,"toolCalls":0,"inputTokens":2,"outputTokens":3,"cacheReadInputTokens":0,"cacheWriteInputTokens":0,"completionInputTokens":2,"completionOutputTokens":3,"completionTokens":5,"embeddingTokens":0,"totalTokens":5},
				"totalDurationMs":10,"averageDurationMs":10,"daily":[]
			}`,
		},
		{
			name: "DashboardTokens",
			payload: `{
				"window":"all","startsAt":"2025-01-01T00:00:00Z","endsAt":"2026-10-01T00:00:00Z",
				"inputTokens":1,"outputTokens":2,"cacheReadInputTokens":0,"cacheWriteInputTokens":0,"completionInputTokens":1,"completionOutputTokens":2,"completionTokens":3,"embeddingTokens":0,"totalTokens":3
			}`,
		},
		{
			name:    "DashboardMessages",
			payload: `{"window":"7d","startsAt":"2026-10-01T00:00:00Z","endsAt":"2026-10-08T00:00:00Z","messages":2}`,
		},
		{
			name:    "DashboardDuration",
			payload: `{"window":"7d","startsAt":"2026-10-01T00:00:00Z","endsAt":"2026-10-08T00:00:00Z","totalDurationMs":42}`,
		},
		{
			name:    "DashboardActivity",
			payload: `{"window":"365d","startsAt":"2025-10-01T00:00:00Z","endsAt":"2026-10-01T00:00:00Z","daily":[]}`,
		},
		{
			name:    "DashboardBreakdown",
			payload: `{"window":"30d","startsAt":"2026-09-01T00:00:00Z","endsAt":"2026-10-01T00:00:00Z","items":[]}`,
		},
		{
			name:    "DashboardAdoption",
			payload: `{"window":"7d","startsAt":"2026-10-01T00:00:00Z","endsAt":"2026-10-08T00:00:00Z","activeUsers":1,"activeServiceAccounts":2,"activeVirtualKeys":3}`,
		},
		{
			name: "GroupListItem",
			payload: `{
				"id":"11111111-1111-4111-8111-111111111111","name":"platform","displayName":"Platform","description":"Platform team",
				"providers":[],"modelPatterns":[],"excludedModelPatterns":[],"members":[],
				"providerCount":0,"modelPatternCount":0,"memberCount":0,
				"createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-01T00:00:00Z","inputTokens":10,"outputTokens":20
			}`,
		},
		{
			name: "CreateTokenResponse",
			payload: `{
				"token":"pg_secret","tokenInfo":{
					"id":"11111111-1111-4111-8111-111111111111","userId":"22222222-2222-4222-8222-222222222222","name":"automation","description":"CI token",
					"expiresAt":"2026-11-01T00:00:00Z","createdAt":"2026-10-01T00:00:00Z"
				}
			}`,
		},
		{
			name: "AdminUser",
			payload: `{
				"id":"11111111-1111-4111-8111-111111111111","sub":null,"preferredUsername":"pending-user","email":"","name":"",
				"type":"user","role":"limited","subscriptionPlanId":null,"quotaResetCredits":0,"note":"","isActive":true,"activeInLast30Days":false,
				"inputTokens":0,"outputTokens":0,"expiresAt":null,"lastLoginAt":null,
				"createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-01T00:00:00Z"
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schemaRef, ok := document.Components.Schemas[test.name]
			if !ok || schemaRef == nil || schemaRef.Value == nil {
				t.Fatalf("missing schema %q", test.name)
			}

			var payload any
			if err := json.Unmarshal([]byte(test.payload), &payload); err != nil {
				t.Fatalf("decode representative payload: %v", err)
			}
			if err := schemaRef.Value.VisitJSON(payload, openapi3.VisitAsResponse()); err != nil {
				t.Fatalf("representative response is rejected: %v", err)
			}
		})
	}
}

func TestCreateTokenResponseMarksRawTokenReadOnly(t *testing.T) {
	_, document := loadSourceDocument(t)
	schemaRef := document.Components.Schemas["CreateTokenResponse"]
	if schemaRef == nil || schemaRef.Value == nil {
		t.Fatal("missing CreateTokenResponse schema")
	}
	tokenRef := schemaRef.Value.Properties["token"]
	if tokenRef == nil || tokenRef.Value == nil {
		t.Fatal("missing CreateTokenResponse.token schema")
	}
	if !tokenRef.Value.ReadOnly || tokenRef.Value.WriteOnly {
		t.Fatalf("expected response token to be readOnly and not writeOnly, got readOnly=%t writeOnly=%t", tokenRef.Value.ReadOnly, tokenRef.Value.WriteOnly)
	}
}
