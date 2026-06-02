package tokens

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
)

type memoryAuthCache struct {
	user auth.UserProfile
	ok   bool
	set  bool
}

// Get returns a cached profile from the in-memory test auth cache.
func (c *memoryAuthCache) Get(context.Context, string) (auth.UserProfile, bool) {
	return c.user, c.ok
}

// Set stores a profile in the in-memory test auth cache.
func (c *memoryAuthCache) Set(context.Context, string, auth.UserProfile, time.Duration) {
	c.set = true
}

// Version returns the in-memory test auth cache version.
func (c *memoryAuthCache) Version() int64 { return 0 }

// SetVersion updates the in-memory test auth cache version.
func (c *memoryAuthCache) SetVersion(int64) {}

// TestMiddlewareRejectsMissingBearer verifies missing bearer tokens are rejected.
func TestMiddlewareRejectsMissingBearer(t *testing.T) {
	tokenService, userService, _, _ := newTokenTestServices(t)
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        NoopAuthCache{},
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// TestMiddlewareUsesCacheAndStripsProviderCredentials verifies auth cache hits and upstream credential stripping.
func TestMiddlewareUsesCacheAndStripsProviderCredentials(t *testing.T) {
	tokenService, userService, _, user := newTokenTestServices(t)
	cache := &memoryAuthCache{user: user, ok: true}
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        cache,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.UserFromContext(r.Context()); !ok {
			t.Fatal("expected user in context")
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" {
			t.Fatal("expected provider credentials stripped")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer cached-token")
	req.Header.Set("X-Api-Key", "provider-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if cache.set {
		t.Fatal("cache hit should not write cache")
	}
}

// TestMiddlewareWithOptionsRejectsCookieSessionWithoutBearer verifies middleware with options rejects cookie session without bearer.
func TestMiddlewareWithOptionsRejectsCookieSessionWithoutBearer(t *testing.T) {
	tokenService, userService, _, user := newTokenTestServices(t)
	sessionStore := auth.NewSessionStore(userService, time.Hour)
	session, err := sessionStore.CreateSession(user, "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        NoopAuthCache{},
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "promptgate_session", Value: session.ID})
	req.Header.Set("X-Api-Key", "provider-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// TestMiddlewareWithOptionsRejectsMissingAuthCredentials verifies middleware with options rejects missing auth credentials.
func TestMiddlewareWithOptionsRejectsMissingAuthCredentials(t *testing.T) {
	tokenService, userService, _, _ := newTokenTestServices(t)
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        NoopAuthCache{},
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
