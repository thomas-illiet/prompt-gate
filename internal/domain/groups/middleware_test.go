package groups

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/provider"
)

// testProviderTypes returns provider types.
func testProviderTypes(names ...string) map[string]provider.ProviderType {
	all := map[string]provider.ProviderType{
		"openai":    provider.ProviderTypeOpenAI,
		"ollama":    provider.ProviderTypeOllama,
		"anthropic": provider.ProviderTypeAnthropic,
	}
	out := make(map[string]provider.ProviderType, len(names))
	for _, name := range names {
		out[name] = all[name]
	}
	return out
}

// TestMiddlewareAllowsModelRegexAndRestoresBody verifies middleware allows model regex and restores body.
func TestMiddlewareAllowsModelRegexAndRestoresBody(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^gpt-5`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(raw) != `{"model":"gpt-5-mini"}` {
			t.Fatalf("body was not restored: %q", raw)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-5-mini"}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:          "user-id",
		Sub:         "sub",
		Role:        auth.RoleUser,
		IsActive:    true,
		LastLoginAt: time.Now().UTC(),
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMiddlewareAllowsRequestBodyAtConfiguredLimit verifies configured body limits are inclusive.
func TestMiddlewareAllowsRequestBodyAtConfiguredLimit(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^gpt-5`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	body := `{"model":"gpt-5-mini"}`
	handler := MiddlewareWithOptions(store, nil, MiddlewareOptions{
		MaxBufferedRequestBytes: int64(len(body)),
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(raw) != body {
			t.Fatalf("body was not restored: %q", raw)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMiddlewareRejectsOversizedRequestBody verifies oversized requests fail before proxying.
func TestMiddlewareRejectsOversizedRequestBody(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^gpt-5`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	body := `{"model":"gpt-5-mini"}`
	handler := MiddlewareWithOptions(store, nil, MiddlewareOptions{
		MaxBufferedRequestBytes: int64(len(body) - 1),
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "request_body_too_large") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// TestMiddlewareDeniesUserWithoutGroup verifies middleware denies user without group.
func TestMiddlewareDeniesUserWithoutGroup(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users:          map[string]UserAccess{},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-5-mini"}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "group_access_denied") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// TestMiddlewareDeniesRequestWithoutModel verifies middleware denies request without model.
func TestMiddlewareDeniesRequestWithoutModel(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^gpt-5`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "group_model_required") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// TestMiddlewareAllowsWhitelistedProviderRoutesWithoutModel verifies middleware allows whitelisted provider routes without model.
func TestMiddlewareAllowsWhitelistedProviderRoutesWithoutModel(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai", "ollama", "anthropic"},
		ProviderTypes:  testProviderTypes("openai", "ollama", "anthropic"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai", "ollama", "anthropic"},
					ModelPatterns: []string{`^approved-model$`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name string
		path string
	}{
		{name: "openai models exact", path: "/openai/v1/models"},
		{name: "openai models subtree", path: "/openai/v1/models/gpt-5"},
		{name: "openai conversations exact", path: "/openai/v1/conversations"},
		{name: "openai conversations subtree", path: "/openai/v1/conversations/conv_123"},
		{name: "openai responses subtree", path: "/openai/v1/responses/resp_123"},
		{name: "ollama models exact", path: "/ollama/v1/models"},
		{name: "ollama models subtree", path: "/ollama/v1/models/llama3.2"},
		{name: "anthropic models exact", path: "/anthropic/v1/models"},
		{name: "anthropic models subtree", path: "/anthropic/v1/models/claude-sonnet-4"},
		{name: "anthropic count tokens", path: "/anthropic/v1/messages/count_tokens"},
		{name: "anthropic event logging", path: "/anthropic/api/event_logging/events"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
				ID:       "user-id",
				Role:     auth.RoleUser,
				IsActive: true,
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestMiddlewareDeniesWhitelistedProviderRouteWithoutProviderGrant verifies middleware denies whitelisted provider route without provider grant.
func TestMiddlewareDeniesWhitelistedProviderRouteWithoutProviderGrant(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai", "anthropic"},
		ProviderTypes:  testProviderTypes("openai", "anthropic"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"anthropic"},
					ModelPatterns: []string{`^claude-`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "group_access_denied") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// TestMiddlewareFiltersOpenAIModelListWithExcludedRegex verifies OpenAI model listings hide excluded models.
func TestMiddlewareFiltersOpenAIModelListWithExcludedRegex(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:             []string{"openai"},
					ModelPatterns:         []string{`.*`},
					ExcludedModelPatterns: []string{`^bge`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-5-mini"},{"id":"bge-large"},{"id":"text-embedding-3-small"}]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := modelIDs(response.Data); strings.Join(got, ",") != "gpt-5-mini,text-embedding-3-small" {
		t.Fatalf("unexpected filtered models: %#v", got)
	}
}

// TestMiddlewareFiltersOllamaModelListWithExcludedRegex verifies Ollama model listings hide excluded models.
func TestMiddlewareFiltersOllamaModelListWithExcludedRegex(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"ollama"},
		ProviderTypes:  testProviderTypes("ollama"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:             []string{"ollama"},
					ModelPatterns:         []string{`.*`},
					ExcludedModelPatterns: []string{`^bge`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2"},{"name":"bge-m3"},{"name":"nomic-embed-text"}]}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/ollama/v1/models", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := modelNames(response.Models); strings.Join(got, ",") != "llama3.2,nomic-embed-text" {
		t.Fatalf("unexpected filtered models: %#v", got)
	}
}

// TestMiddlewareLeavesModelListErrorsUnchanged verifies non-success model list responses are not rewritten.
func TestMiddlewareLeavesModelListErrorsUnchanged(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`.*`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"upstream"}`, http.StatusBadGateway)
	}))

	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "upstream") {
		t.Fatalf("expected upstream body to be preserved, got %s", rec.Body.String())
	}
}

// TestMiddlewareDeniesProviderGrantWithoutModelRegex verifies middleware denies provider grant without model regex.
func TestMiddlewareDeniesProviderGrantWithoutModelRegex(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers: []string{"openai"},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-5-mini"}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "group_access_denied") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// TestMiddlewareDeniesExcludedModelRequest verifies excluded model patterns apply to proxied model requests.
func TestMiddlewareDeniesExcludedModelRequest(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"ollama"},
		ProviderTypes:  testProviderTypes("ollama"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:             []string{"ollama"},
					ModelPatterns:         []string{`.*`},
					ExcludedModelPatterns: []string{`^bge`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/ollama/v1/chat/completions", strings.NewReader(`{"model":"bge-m3"}`))
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "group_access_denied") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

// TestMiddlewareAllowsMultipleProvidersFromOneGroupWithMatchingModelRegex verifies middleware allows multiple providers from one group with matching model regex.
func TestMiddlewareAllowsMultipleProvidersFromOneGroupWithMatchingModelRegex(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai", "anthropic"},
		ProviderTypes:  testProviderTypes("openai", "anthropic"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai", "anthropic"},
					ModelPatterns: []string{`^gpt-5`, `^claude-sonnet`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/openai/v1/chat/completions", body: `{"model":"gpt-5-mini"}`},
		{path: "/anthropic/v1/messages", body: `{"model":"claude-sonnet-4"}`},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
			ID:       "user-id",
			Role:     auth.RoleUser,
			IsActive: true,
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 for %s, got %d: %s", tc.path, rec.Code, rec.Body.String())
		}
	}
}

// TestMiddlewareDeniesUnknownProvider verifies middleware denies unknown provider.
func TestMiddlewareDeniesUnknownProvider(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^gpt-5`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/unknown/v1/models", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func modelIDs(models []struct {
	ID string `json:"id"`
}) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		out = append(out, model.ID)
	}
	return out
}

func modelNames(models []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		out = append(out, model.Name)
	}
	return out
}
