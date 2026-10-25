package groups

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/provider"
)

func TestProviderBypassAllowsOnlyExplicitSafeRoutes(t *testing.T) {
	tests := []struct {
		name         string
		providerType provider.ProviderType
		path         string
		want         bool
	}{
		{name: "openai exact models", providerType: provider.ProviderTypeOpenAI, path: "/openai/v1/models", want: true},
		{name: "openai models trailing slash", providerType: provider.ProviderTypeOpenAI, path: "/openai/v1/models/", want: false},
		{name: "openai model resource", providerType: provider.ProviderTypeOpenAI, path: "/openai/v1/models/gpt-5", want: false},
		{name: "openai conversations", providerType: provider.ProviderTypeOpenAI, path: "/openai/v1/conversations/conv_123", want: false},
		{name: "openai response resource", providerType: provider.ProviderTypeOpenAI, path: "/openai/v1/responses/resp_123", want: false},
		{name: "ollama exact models", providerType: provider.ProviderTypeOllama, path: "/ollama/v1/models", want: true},
		{name: "ollama model resource", providerType: provider.ProviderTypeOllama, path: "/ollama/v1/models/llama3.2", want: false},
		{name: "anthropic exact models", providerType: provider.ProviderTypeAnthropic, path: "/anthropic/v1/models", want: true},
		{name: "anthropic model resource", providerType: provider.ProviderTypeAnthropic, path: "/anthropic/v1/models/claude-sonnet-4", want: false},
		{name: "anthropic count tokens requires model authorization", providerType: provider.ProviderTypeAnthropic, path: "/anthropic/v1/messages/count_tokens", want: false},
		{name: "anthropic telemetry", providerType: provider.ProviderTypeAnthropic, path: "/anthropic/api/event_logging/events", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isModelBypassRoute(test.providerType, test.path); got != test.want {
				t.Fatalf("isModelBypassRoute(%q): got %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestAnthropicCountTokensRequiresAllowedModel(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"anthropic"},
		ProviderTypes:  testProviderTypes("anthropic"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"anthropic"},
					ModelPatterns: []string{`^claude-allowed$`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	for _, test := range []struct {
		name       string
		body       string
		wantStatus int
		wantCalled bool
	}{
		{name: "allowed model", body: `{"model":"claude-allowed","messages":[]}`, wantStatus: http.StatusNoContent, wantCalled: true},
		{name: "denied model", body: `{"model":"claude-denied","messages":[]}`, wantStatus: http.StatusForbidden},
		{name: "missing model", body: `{"messages":[]}`, wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
			request := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages/count_tokens", strings.NewReader(test.body))
			request = request.WithContext(auth.ContextWithUser(request.Context(), auth.UserProfile{
				ID:       "user-id",
				Role:     auth.RoleUser,
				IsActive: true,
			}))
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status: got %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if called != test.wantCalled {
				t.Fatalf("downstream called: got %v, want %v", called, test.wantCalled)
			}
		})
	}
}
