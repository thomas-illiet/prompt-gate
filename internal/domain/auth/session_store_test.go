package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
)

type testUserResolver struct {
	user UserProfile
	err  error
}

// UserByID returns a test user profile by id.
func (r testUserResolver) UserByID(context.Context, string) (UserProfile, error) {
	if r.err != nil {
		return UserProfile{}, r.err
	}
	return r.user, nil
}

// testUser returns user.
func testUser(id string) UserProfile {
	return UserProfile{
		ID:                id,
		Sub:               "sub-" + id,
		PreferredUsername: "user-" + id,
		Email:             id + "@example.com",
		Name:              "User " + id,
		Type:              UserTypeUser,
		Role:              RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
}

// newRedisSessionStorePair creates Redis session store pair.
func newRedisSessionStorePair(t *testing.T, ttl time.Duration) (*miniredis.Miniredis, *SessionStore, *SessionStore) {
	t.Helper()

	srv := miniredis.RunT(t)
	storeA, err := redisstore.NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("redis store A: %v", err)
	}
	t.Cleanup(func() { _ = storeA.Close() })

	storeB, err := redisstore.NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("redis store B: %v", err)
	}
	t.Cleanup(func() { _ = storeB.Close() })

	user := testUser("user-1")
	return srv,
		NewRedisSessionStore(testUserResolver{user: user}, ttl, storeA),
		NewRedisSessionStore(testUserResolver{user: user}, ttl, storeB)
}

// TestSessionStoreMemorySessionExpiresAndRefreshesUser verifies session store memory session expires and refreshes user.
func TestSessionStoreMemorySessionExpiresAndRefreshesUser(t *testing.T) {
	original := testUser("user-1")
	refreshed := original
	refreshed.Name = "Fresh Name"
	store := NewSessionStore(testUserResolver{user: refreshed}, time.Hour)

	session, err := store.CreateSession(original, "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ExpiresAt.Before(time.Now().Add(59 * time.Minute)) {
		t.Fatalf("expected session ttl near one hour, got %s", session.ExpiresAt)
	}

	loaded, ok := store.Session(context.Background(), session.ID)
	if !ok {
		t.Fatal("expected session to load")
	}
	if loaded.User.Name != refreshed.Name {
		t.Fatalf("expected refreshed user, got %#v", loaded.User)
	}

	store.mu.Lock()
	expired := store.sessions[session.ID]
	expired.ExpiresAt = time.Now().Add(-time.Second)
	store.sessions[session.ID] = expired
	store.mu.Unlock()

	if _, ok := store.Session(context.Background(), session.ID); ok {
		t.Fatal("expected expired session to be rejected")
	}
}

// TestRedisSessionStoreSharesSessionAcrossStores verifies Redis session store shares session across stores.
func TestRedisSessionStoreSharesSessionAcrossStores(t *testing.T) {
	_, storeA, storeB := newRedisSessionStorePair(t, time.Hour)

	session, err := storeA.CreateSession(testUser("user-1"), "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	loaded, ok := storeB.Session(context.Background(), session.ID)
	if !ok {
		t.Fatal("expected second store to load redis session")
	}
	if loaded.ID != session.ID || loaded.IDToken != "id-token" {
		t.Fatalf("unexpected redis session: %#v", loaded)
	}
}

// TestRedisSessionStoreDeletesSessionOnLogout verifies Redis session store deletes session on logout.
func TestRedisSessionStoreDeletesSessionOnLogout(t *testing.T) {
	_, storeA, storeB := newRedisSessionStorePair(t, time.Hour)

	session, err := storeA.CreateSession(testUser("user-1"), "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	storeB.DeleteSession(session.ID)
	if _, ok := storeA.Session(context.Background(), session.ID); ok {
		t.Fatal("expected deleted redis session to be unavailable")
	}
}

// TestRedisSessionStoreExpiresSession verifies Redis session store expires session.
func TestRedisSessionStoreExpiresSession(t *testing.T) {
	srv, storeA, storeB := newRedisSessionStorePair(t, time.Second)

	session, err := storeA.CreateSession(testUser("user-1"), "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	srv.FastForward(2 * time.Second)
	if _, ok := storeB.Session(context.Background(), session.ID); ok {
		t.Fatal("expected redis-expired session to be unavailable")
	}
}

// TestRedisSessionStoreDeletesSessionWhenUserMissing verifies Redis session store deletes session when user missing.
func TestRedisSessionStoreDeletesSessionWhenUserMissing(t *testing.T) {
	srv := miniredis.RunT(t)
	redisStore, err := redisstore.NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("redis store: %v", err)
	}
	defer redisStore.Close()

	storeA := NewRedisSessionStore(testUserResolver{user: testUser("user-1")}, time.Hour, redisStore)
	session, err := storeA.CreateSession(testUser("user-1"), "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	storeB := NewRedisSessionStore(testUserResolver{err: errors.New("missing user")}, time.Hour, redisStore)
	if _, ok := storeB.Session(context.Background(), session.ID); ok {
		t.Fatal("expected missing user to invalidate session")
	}
	if srv.Exists(redisstore.AuthSessionKey(session.ID)) {
		t.Fatal("expected missing user to delete redis session key")
	}
}

// TestRedisAuthorizationRequestSharedAndConsumedOnce verifies Redis authorization request shared and consumed once.
func TestRedisAuthorizationRequestSharedAndConsumedOnce(t *testing.T) {
	_, storeA, storeB := newRedisSessionStorePair(t, time.Hour)

	request, err := storeA.CreateAuthorizationRequest("/admin", "http://localhost:3000")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	loaded, ok := storeB.ConsumeAuthorizationRequest(request.State)
	if !ok {
		t.Fatal("expected second store to consume redis auth request")
	}
	if loaded.CodeVerifier != request.CodeVerifier || loaded.RedirectPath != "/admin" {
		t.Fatalf("unexpected auth request: %#v", loaded)
	}
	if _, ok := storeA.ConsumeAuthorizationRequest(request.State); ok {
		t.Fatal("expected auth request to be consumed once")
	}
}

// TestRedisAuthorizationRequestExpires verifies Redis authorization request expires.
func TestRedisAuthorizationRequestExpires(t *testing.T) {
	srv, storeA, storeB := newRedisSessionStorePair(t, time.Hour)

	request, err := storeA.CreateAuthorizationRequest("/admin", "http://localhost:3000")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	srv.FastForward(11 * time.Minute)
	if _, ok := storeB.ConsumeAuthorizationRequest(request.State); ok {
		t.Fatal("expected expired auth request to be unavailable")
	}
}
