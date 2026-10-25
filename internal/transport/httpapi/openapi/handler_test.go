package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"promptgate/backend/internal/platform/config"
)

func TestNewHandlerServesConfiguredRuntimeDocument(t *testing.T) {
	cfg := config.APIConfig{
		PublicURLConfig: config.PublicURLConfig{
			BackendBaseURL: "https://api.example.test/",
		},
		SessionConfig: config.SessionConfig{
			SessionCookieName: "custom_session",
		},
	}
	if _, err := buildDocument(cfg); err != nil {
		t.Fatalf("build runtime OpenAPI document: %v", err)
	}
	handler := NewHandler(cfg)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("unexpected cache control %q", got)
	}

	var document struct {
		Components struct {
			SecuritySchemes map[string]struct {
				Type string `json:"type"`
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode OpenAPI response: %v", err)
	}
	var rawDocument map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &rawDocument); err != nil {
		t.Fatalf("decode raw OpenAPI response: %v", err)
	}
	if _, ok := rawDocument["servers"]; ok {
		t.Fatal("OpenAPI response unexpectedly contains servers")
	}
	securityScheme, ok := document.Components.SecuritySchemes[sessionSecurityScheme]
	if !ok {
		t.Fatalf("missing %q security scheme", sessionSecurityScheme)
	}
	if securityScheme.Type != "apiKey" || securityScheme.In != "cookie" || securityScheme.Name != "custom_session" {
		t.Fatalf("unexpected session security scheme %#v", securityScheme)
	}
}

func TestNewHandlerUsesSafeRuntimeDefaults(t *testing.T) {
	if _, err := buildDocument(config.APIConfig{}); err != nil {
		t.Fatalf("build default OpenAPI document: %v", err)
	}
	handler := NewHandler(config.APIConfig{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var document struct {
		Components struct {
			SecuritySchemes map[string]struct {
				Name string `json:"name"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode OpenAPI response: %v", err)
	}
	var rawDocument map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &rawDocument); err != nil {
		t.Fatalf("decode raw OpenAPI response: %v", err)
	}
	if _, ok := rawDocument["servers"]; ok {
		t.Fatal("OpenAPI response unexpectedly contains servers")
	}
	if got := document.Components.SecuritySchemes[sessionSecurityScheme].Name; got != defaultSessionCookieName {
		t.Fatalf("unexpected default session cookie name %q", got)
	}
}
