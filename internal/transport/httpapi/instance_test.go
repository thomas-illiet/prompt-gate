package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"promptgate/backend/internal/platform/config"
)

func TestHandleInstanceReturnsOnlyEnvironment(t *testing.T) {
	recorder := httptest.NewRecorder()
	srv := server{config: config.APIConfig{
		EnvironmentConfig: config.EnvironmentConfig{Environment: config.EnvironmentStaging},
		PublicURLConfig: config.PublicURLConfig{
			FrontendBaseURL: "https://frontend.internal.example",
			BackendBaseURL:  "https://backend.internal.example",
		},
	}}

	srv.handleInstance(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/instance", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 1 || response["environment"] != "staging" {
		t.Fatalf("unexpected instance response: %#v", response)
	}
}
