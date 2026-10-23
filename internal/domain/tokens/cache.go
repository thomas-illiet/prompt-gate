package tokens

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

type AuthCache interface {
	Get(ctx context.Context, tokenHash string) (auth.Principal, int64, bool, error)
	Set(ctx context.Context, generation int64, tokenHash string, principal auth.Principal, ttl time.Duration)
	Version() int64
	SetVersion(version int64)
}

type NoopAuthCache struct{}

// Get implements an always-miss auth cache.
func (NoopAuthCache) Get(context.Context, string) (auth.Principal, int64, bool, error) {
	return auth.Principal{}, 0, false, nil
}

// Set implements a no-op auth cache write.
func (NoopAuthCache) Set(context.Context, int64, string, auth.Principal, time.Duration) {}

// Version returns the no-op auth cache version.
func (NoopAuthCache) Version() int64 { return 0 }

// SetVersion ignores auth cache version updates.
func (NoopAuthCache) SetVersion(int64) {}

type RedisAuthCache struct {
	store   *redisstore.Store
	version atomic.Int64
	ttl     time.Duration
	logger  *slog.Logger
}

// NewRedisAuthCache creates a Redis-backed auth cache.
func NewRedisAuthCache(store *redisstore.Store, ttl time.Duration, logger *slog.Logger) *RedisAuthCache {
	if logger == nil {
		logger = slog.Default()
	}
	cache := &RedisAuthCache{store: store, ttl: ttl, logger: logger}
	return cache
}

// SyncVersion refreshes the local auth cache version from Redis.
func (c *RedisAuthCache) SyncVersion(ctx context.Context) {
	if c == nil || c.store == nil || !c.store.Enabled() {
		return
	}
	version, err := c.store.Version(ctx, configevents.DomainAuth)
	if err != nil {
		c.logger.Warn("load auth cache version failed", "error", err)
		return
	}
	c.version.Store(version)
}

// Get atomically loads the current Redis auth generation and its cached user
// profile. The returned generation must be reused by Set after a cache miss.
func (c *RedisAuthCache) Get(ctx context.Context, tokenHash string) (auth.Principal, int64, bool, error) {
	if c == nil || c.store == nil || !c.store.Enabled() {
		return auth.Principal{}, 0, false, nil
	}
	var principal auth.Principal
	generation, ok, err := c.store.GetAuthCacheJSON(ctx, tokenHash, &principal)
	if err != nil {
		c.logger.Warn("auth cache get failed", "error", err)
		return auth.Principal{}, 0, false, err
	}
	c.SetVersion(generation)
	if ok && (strings.TrimSpace(principal.User.ID) == "" || strings.TrimSpace(principal.CredentialID) == "") {
		return auth.Principal{}, generation, false, nil
	}
	return principal, generation, ok, nil
}

// Set stores a cached user profile in the generation captured by Get. Using an
// explicit generation prevents a revocation between validation and this write
// from placing a stale principal in the newly active generation.
func (c *RedisAuthCache) Set(ctx context.Context, generation int64, tokenHash string, principal auth.Principal, ttl time.Duration) {
	if c == nil || c.store == nil || !c.store.Enabled() || generation < 0 || ttl <= 0 {
		return
	}
	if c.ttl > 0 && ttl > c.ttl {
		ttl = c.ttl
	}
	if err := c.store.SetJSON(ctx, redisstore.AuthCacheKey(generation, tokenHash), principal, ttl); err != nil {
		c.logger.Warn("auth cache set failed", "error", err)
	}
}

// Version returns the local auth cache version.
func (c *RedisAuthCache) Version() int64 {
	if c == nil {
		return 0
	}
	return c.version.Load()
}

// SetVersion advances the local auth cache version monotonically so delayed
// Pub/Sub events cannot roll a process back to an older generation.
func (c *RedisAuthCache) SetVersion(version int64) {
	if c == nil {
		return
	}
	for {
		current := c.version.Load()
		if version <= current {
			return
		}
		if c.version.CompareAndSwap(current, version) {
			return
		}
	}
}
