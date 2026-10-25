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

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openai/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
		case "/ollama/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[]}`))
		case "/anthropic/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))

	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
	}{
		{name: "openai models exact", method: http.MethodGet, path: "/openai/v1/models", statusCode: http.StatusOK},
		{name: "ollama models exact", method: http.MethodGet, path: "/ollama/v1/models", statusCode: http.StatusOK},
		{name: "anthropic models exact", method: http.MethodGet, path: "/anthropic/v1/models", statusCode: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req = req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
				ID:       "user-id",
				Role:     auth.RoleUser,
				IsActive: true,
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.statusCode {
				t.Fatalf("expected %d, got %d: %s", tc.statusCode, rec.Code, rec.Body.String())
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
		w.Header().Set("Content-Length", "4096")
		w.Header().Set("Content-Md5", "stale-content-md5")
		w.Header().Set("Content-Digest", "sha-256=:stale:")
		w.Header().Set("Digest", "sha-256=stale")
		w.Header().Set("Etag", `"upstream-catalog"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		w.Header().Set("Repr-Digest", "sha-256=:stale:")
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
	for _, name := range []string{"Content-Length", "Content-Md5", "Content-Digest", "Digest", "Etag", "Last-Modified", "Repr-Digest"} {
		if value := rec.Header().Get(name); value != "" {
			t.Fatalf("expected stale %s to be removed, got %q", name, value)
		}
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

// TestMiddlewareFiltersAnthropicModelList verifies Anthropic's exact model
// catalog route is filtered with the same fail-closed policy as other providers.
func TestMiddlewareFiltersAnthropicModelList(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"anthropic"},
		ProviderTypes:  testProviderTypes("anthropic"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"anthropic"},
					ModelPatterns: []string{`^claude-sonnet`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-4"},{"id":"claude-opus-4"}],"first_id":"claude-sonnet-4","last_id":"claude-opus-4","has_more":true,"next_cursor":"opaque-secret-cursor"}`))
	}))
	req := httptest.NewRequest(http.MethodGet, "/anthropic/v1/models", nil)
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
		FirstID    string  `json:"first_id"`
		LastID     string  `json:"last_id"`
		HasMore    bool    `json:"has_more"`
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := modelIDs(response.Data); strings.Join(got, ",") != "claude-sonnet-4" {
		t.Fatalf("unexpected filtered models: %#v", got)
	}
	if response.FirstID != "claude-sonnet-4" || response.LastID != "claude-sonnet-4" || response.HasMore || response.NextCursor != nil {
		t.Fatalf("filtered pagination leaked excluded catalog state: %#v", response)
	}
}

func TestMiddlewareSanitizesPaginationWithoutCurrentPageFiltering(t *testing.T) {
	store := modelListTestSnapshot(t)
	for name, upstreamBody := range map[string]string{
		"allowed page": `{"data":[{"id":"gpt-5-mini"}],"first_id":"gpt-5-mini","last_id":"gpt-5-mini","has_more":true,"next_cursor":"secret-model"}`,
		"empty page":   `{"data":[],"first_id":"secret-model","last_id":"secret-model","has_more":true,"next_cursor":"secret-model"}`,
	} {
		t.Run(name, func(t *testing.T) {
			handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(upstreamBody))
			}))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, modelListTestRequest())

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "secret-model") || strings.Contains(recorder.Body.String(), "next_cursor") || strings.Contains(recorder.Body.String(), `"has_more":true`) {
				t.Fatalf("pagination leaked excluded catalog state: %s", recorder.Body.String())
			}
		})
	}
}

// TestMiddlewareFiltersModelListAfterEarlyHints verifies a provisional 103
// cannot make the final successful catalog bypass authorization filtering.
func TestMiddlewareFiltersModelListAfterEarlyHints(t *testing.T) {
	store := modelListTestSnapshot(t)
	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", "</models>; rel=preload")
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Del("Link")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"},{"id":"secret-model"}]}`))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, modelListTestRequest())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected final 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret-model") || !strings.Contains(rec.Body.String(), "gpt-5-mini") {
		t.Fatalf("early hints bypassed model filtering: %s", rec.Body.String())
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

// TestMiddlewareRejectsOversizedModelList verifies filtered responses are bounded.
func TestMiddlewareRejectsOversizedModelList(t *testing.T) {
	store := modelListTestSnapshot(t)
	handler := MiddlewareWithOptions(store, nil, MiddlewareOptions{
		MaxBufferedResponseBytes: 32,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-5-mini"},{"id":"gpt-5-large"}]}`))
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, modelListTestRequest())

	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "model_response_too_large") {
		t.Fatalf("expected bounded model response 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMiddlewareNeutralizesAndRejectsCompressedModelList verifies compressed bytes cannot bypass filtering.
func TestMiddlewareNeutralizesAndRejectsCompressedModelList(t *testing.T) {
	store := modelListTestSnapshot(t)
	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept-Encoding"); got != "identity" {
			t.Fatalf("expected identity encoding upstream, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header()["Content-Encoding"] = []string{"identity", "gzip"}
		_, _ = w.Write([]byte("compressed-secret-catalog"))
	}))

	req := modelListTestRequest()
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "invalid_model_response") {
		t.Fatalf("expected compressed model response 502, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("compressed upstream body leaked: %s", rec.Body.String())
	}
}

// TestMiddlewareRejectsInvalidSuccessfulModelList verifies malformed success responses fail closed.
func TestMiddlewareRejectsInvalidSuccessfulModelList(t *testing.T) {
	store := modelListTestSnapshot(t)
	for name, body := range map[string]string{
		"malformed JSON":            `{"data":[`,
		"missing list":              `{"object":"list","unexpected":[{"id":"bge-secret"}]}`,
		"non-array list":            `{"object":"list","data":{"id":"bge-secret"}}`,
		"null list":                 `{"object":"list","data":null}`,
		"duplicate list":            `{"data":[{"id":"bge-secret"}],"data":[{"id":"gpt-5-mini"}]}`,
		"case-folded duplicate":     `{"data":[{"id":"gpt-5-mini"}],"DATA":[{"id":"bge-secret"}]}`,
		"unicode-folded duplicate":  `{"models":[{"name":"gpt-5-mini"}],"modelſ":[{"name":"bge-secret"}]}`,
		"nested duplicate model ID": `{"data":[{"id":"gpt-5-mini","ID":"bge-secret"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, modelListTestRequest())

			if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "invalid_model_response") {
				t.Fatalf("expected invalid model response 502, got %d: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "bge-secret") {
				t.Fatalf("invalid upstream model body leaked: %s", rec.Body.String())
			}
		})
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

func modelListTestSnapshot(t *testing.T) *SnapshotStore {
	t.Helper()
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes:  testProviderTypes("openai"),
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^gpt-`},
				}},
			},
		},
	}); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}
	return store
}

func modelListTestRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
	return req.WithContext(auth.ContextWithUser(req.Context(), auth.UserProfile{
		ID:       "user-id",
		Role:     auth.RoleUser,
		IsActive: true,
	}))
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
