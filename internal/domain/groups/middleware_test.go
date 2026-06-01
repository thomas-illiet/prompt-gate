package groups

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/provider"
)

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
