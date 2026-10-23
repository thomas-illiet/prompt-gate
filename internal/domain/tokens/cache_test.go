package tokens

import (
	"context"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
)

func newRedisAuthCacheTestStore(t *testing.T) (*miniredis.Miniredis, *redisstore.Store, *RedisAuthCache) {
	t.Helper()

	server := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+server.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return server, store, NewRedisAuthCache(store, time.Minute, nil)
}

func testCachedPrincipal(id string) auth.Principal {
	return auth.Principal{
		User: auth.UserProfile{
			ID:                id,
			PreferredUsername: "cached-user",
			Type:              auth.UserTypeUser,
			Role:              auth.RoleUser,
			IsActive:          true,
		},
		CredentialID:   "credential-" + id,
		CredentialName: "test-token",
	}
}

// TestRedisAuthCacheReadsAuthoritativeGeneration verifies a missed Pub/Sub
// notification cannot leave a process reading an entry from an old namespace.
func TestRedisAuthCacheReadsAuthoritativeGeneration(t *testing.T) {
	_, store, cache := newRedisAuthCacheTestStore(t)
	ctx := context.Background()
	oldPrincipal := testCachedPrincipal("old-user")
	newPrincipal := testCachedPrincipal("new-user")

	cache.Set(ctx, 0, "token-hash", oldPrincipal, time.Minute)
	if err := store.Client().Set(ctx, redisstore.VersionKey(configevents.DomainAuth), 1, 0).Err(); err != nil {
		t.Fatalf("bump auth version without publish: %v", err)
	}
	if err := store.SetJSON(ctx, redisstore.AuthCacheKey(1, "token-hash"), newPrincipal, time.Minute); err != nil {
		t.Fatalf("seed current auth cache entry: %v", err)
	}

	got, generation, hit, err := cache.Get(ctx, "token-hash")
	if err != nil {
		t.Fatalf("get auth cache: %v", err)
	}
	if !hit || generation != 1 || got.User.ID != newPrincipal.User.ID {
		t.Fatalf("expected generation 1 principal, hit=%v generation=%d principal=%#v", hit, generation, got)
	}
	if cache.Version() != 1 {
		t.Fatalf("expected local version to advance to 1, got %d", cache.Version())
	}
}

// TestRedisAuthCacheSetUsesCapturedGeneration verifies validation completing
// after a revocation cannot populate the newly active cache generation.
func TestRedisAuthCacheSetUsesCapturedGeneration(t *testing.T) {
	_, store, cache := newRedisAuthCacheTestStore(t)
	ctx := context.Background()

	_, generation, hit, err := cache.Get(ctx, "racing-token")
	if err != nil {
		t.Fatalf("initial cache get: %v", err)
	}
	if hit || generation != 0 {
		t.Fatalf("expected generation 0 miss, hit=%v generation=%d", hit, generation)
	}
	if _, err := store.Client().Incr(ctx, redisstore.VersionKey(configevents.DomainAuth)).Result(); err != nil {
		t.Fatalf("bump auth version during validation: %v", err)
	}

	principal := testCachedPrincipal("validated-before-revocation")
	cache.Set(ctx, generation, "racing-token", principal, time.Minute)

	var stored auth.Principal
	ok, err := store.GetJSON(ctx, redisstore.AuthCacheKey(0, "racing-token"), &stored)
	if err != nil || !ok {
		t.Fatalf("expected result under captured generation: ok=%v err=%v", ok, err)
	}
	if ok, err := store.GetJSON(ctx, redisstore.AuthCacheKey(1, "racing-token"), &stored); err != nil || ok {
		t.Fatalf("stale result reached active generation: ok=%v err=%v", ok, err)
	}

	_, currentGeneration, currentHit, err := cache.Get(ctx, "racing-token")
	if err != nil {
		t.Fatalf("read current generation: %v", err)
	}
	if currentHit || currentGeneration != 1 {
		t.Fatalf("expected generation 1 miss, hit=%v generation=%d", currentHit, currentGeneration)
	}
}

// TestRedisAuthCacheSetVersionIsMonotone verifies delayed events cannot roll
// the process-local auth generation backwards.
func TestRedisAuthCacheSetVersionIsMonotone(t *testing.T) {
	cache := &RedisAuthCache{}
	cache.SetVersion(2)
	cache.SetVersion(1)
	if cache.Version() != 2 {
		t.Fatalf("expected delayed version 1 to be ignored, got %d", cache.Version())
	}
	cache.SetVersion(3)
	if cache.Version() != 3 {
		t.Fatalf("expected version to advance to 3, got %d", cache.Version())
	}
}

// TestRedisAuthCacheSyncVersionLoadsRemoteValue verifies startup initializes
// the local generation from the authoritative Redis counter.
func TestRedisAuthCacheSyncVersionLoadsRemoteValue(t *testing.T) {
	_, store, cache := newRedisAuthCacheTestStore(t)
	ctx := context.Background()
	if err := store.Client().Set(ctx, redisstore.VersionKey(configevents.DomainAuth), 7, 0).Err(); err != nil {
		t.Fatalf("seed auth version: %v", err)
	}

	cache.SyncVersion(ctx)
	if cache.Version() != 7 {
		t.Fatalf("expected synced version 7, got %d", cache.Version())
	}
}

// TestRedisAuthCacheCapsEntryTTL verifies the configured auth-cache TTL bounds
// the stale-entry window even when the token itself remains valid longer.
func TestRedisAuthCacheCapsEntryTTL(t *testing.T) {
	server, store, _ := newRedisAuthCacheTestStore(t)
	cache := NewRedisAuthCache(store, 30*time.Second, nil)
	ctx := context.Background()

	cache.Set(ctx, 0, "ttl-token", testCachedPrincipal("ttl-user"), time.Hour)
	key := redisstore.AuthCacheKey(0, "ttl-token")
	if ttl := server.TTL(key); ttl != 30*time.Second {
		t.Fatalf("expected 30s cache TTL, got %s", ttl)
	}
	server.FastForward(31 * time.Second)
	if _, _, hit, err := cache.Get(ctx, "ttl-token"); err != nil || hit {
		t.Fatalf("expected expired cache miss, hit=%v err=%v", hit, err)
	}
}

// TestRedisAuthCacheReturnsMissOnRedisError verifies callers can distinguish a
// Redis failure from an authoritative miss and fall back to database validation
// without writing under an unknown generation.
func TestRedisAuthCacheReturnsMissOnRedisError(t *testing.T) {
	server, _, cache := newRedisAuthCacheTestStore(t)
	server.Close()

	_, _, hit, err := cache.Get(context.Background(), "unavailable-token")
	if err == nil || hit {
		t.Fatalf("expected Redis error and cache miss, hit=%v err=%v", hit, err)
	}
}
