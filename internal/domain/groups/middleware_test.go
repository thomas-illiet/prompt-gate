package groups

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
)

func TestMiddlewareAllowsModelRegexAndRestoresBody(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
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
	if !strings.Contains(rec.Body.String(), "group_model_required") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestMiddlewareDeniesProviderGrantWithoutModelRegex(t *testing.T) {
	store := NewSnapshotStore(nil)
	if err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
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
