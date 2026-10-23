package tokens

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

type memoryAuthCache struct {
	principal  auth.Principal
	generation int64
	ok         bool
	err        error
	set        bool
	setAt      int64
}

// Get returns a cached profile from the in-memory test auth cache.
func (c *memoryAuthCache) Get(context.Context, string) (auth.Principal, int64, bool, error) {
	return c.principal, c.generation, c.ok, c.err
}

// Set stores a profile in the in-memory test auth cache.
func (c *memoryAuthCache) Set(_ context.Context, generation int64, _ string, _ auth.Principal, _ time.Duration) {
	c.set = true
	c.setAt = generation
}

// Version returns the in-memory test auth cache version.
func (c *memoryAuthCache) Version() int64 { return 0 }

// SetVersion updates the in-memory test auth cache version.
func (c *memoryAuthCache) SetVersion(int64) {}

type blockingUserResolver struct {
	delegate UserResolver
	started  chan struct{}
	release  chan struct{}
}

func (r *blockingUserResolver) UserByID(ctx context.Context, id string) (auth.UserProfile, error) {
	close(r.started)
	select {
	case <-r.release:
	case <-ctx.Done():
		return auth.UserProfile{}, ctx.Err()
	}
	return r.delegate.UserByID(ctx, id)
}

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
	cache := &memoryAuthCache{principal: auth.Principal{User: user, CredentialID: "key-id", CredentialName: "key-name"}, ok: true}
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        cache,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.UserFromContext(r.Context()); !ok {
			t.Fatal("expected user in context")
		}
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.CredentialID != "key-id" || principal.CredentialName != "key-name" {
			t.Fatalf("unexpected principal: %#v", principal)
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

// TestMiddlewareCachesValidatedPrincipalInCapturedGeneration verifies a
// validation result is written to the generation observed before the database
// lookup, rather than a potentially newer process-local generation.
func TestMiddlewareCachesValidatedPrincipalInCapturedGeneration(t *testing.T) {
	tokenService, userService, _, user := newTokenTestServices(t)
	created, err := tokenService.CreateToken(context.Background(), user, "middleware_generation", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	cache := &memoryAuthCache{generation: 41}
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        cache,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if !cache.set || cache.setAt != 41 {
		t.Fatalf("expected cache write in captured generation 41, set=%v generation=%d", cache.set, cache.setAt)
	}
}

// TestMiddlewareRevocationDuringValidationCannotPopulateCurrentGeneration
// exercises the complete cache-miss path while user resolution is blocked. A
// concurrent auth version bump must leave the validated result only in the old
// generation captured before validation began.
func TestMiddlewareRevocationDuringValidationCannotPopulateCurrentGeneration(t *testing.T) {
	_, store, cache := newRedisAuthCacheTestStore(t)
	tokenService, userService, _, user := newTokenTestServices(t)
	created, err := tokenService.CreateToken(context.Background(), user, "middleware_revocation_race", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	resolver := &blockingUserResolver{
		delegate: userService,
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: resolver,
		Cache:        cache,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(rec, req)
	}()

	select {
	case <-resolver.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for database validation")
	}
	if _, err := store.Client().Incr(context.Background(), redisstore.VersionKey(configevents.DomainAuth)).Result(); err != nil {
		t.Fatalf("bump auth version: %v", err)
	}
	close(resolver.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for middleware")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	tokenHash := sha256hex(created.Token)
	var principal auth.Principal
	if ok, err := store.GetJSON(context.Background(), redisstore.AuthCacheKey(0, tokenHash), &principal); err != nil || !ok {
		t.Fatalf("expected cache write in captured generation: ok=%v err=%v", ok, err)
	}
	if ok, err := store.GetJSON(context.Background(), redisstore.AuthCacheKey(1, tokenHash), &principal); err != nil || ok {
		t.Fatalf("stale principal reached current generation: ok=%v err=%v", ok, err)
	}
}

// TestMiddlewareValidatesAgainstDatabaseWhenCacheFails verifies Redis cache
// errors fail open only for caching and never bypass token validation.
func TestMiddlewareValidatesAgainstDatabaseWhenCacheFails(t *testing.T) {
	tokenService, userService, _, user := newTokenTestServices(t)
	created, err := tokenService.CreateToken(context.Background(), user, "middleware_cache_error", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	cache := &memoryAuthCache{
		principal: auth.Principal{User: auth.UserProfile{ID: "stale-cache-user"}, CredentialID: "stale-key"},
		ok:        true,
		err:       errors.New("redis unavailable"),
	}
	handler := MiddlewareWithOptions(MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        cache,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.User.ID != user.ID {
			t.Fatalf("expected database-validated principal, got %#v", principal)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected database validation to succeed with 204, got %d", rec.Code)
	}
	if cache.set {
		t.Fatal("cache read errors must not be followed by an unversioned cache write")
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
