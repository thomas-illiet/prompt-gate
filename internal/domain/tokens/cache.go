package tokens

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

type AuthCache interface {
	Get(ctx context.Context, tokenHash string) (auth.UserProfile, bool)
	Set(ctx context.Context, tokenHash string, user auth.UserProfile, ttl time.Duration)
	Version() int64
	SetVersion(version int64)
}

type NoopAuthCache struct{}

// Get implements an always-miss auth cache.
func (NoopAuthCache) Get(context.Context, string) (auth.UserProfile, bool) {
	return auth.UserProfile{}, false
}

// Set implements a no-op auth cache write.
func (NoopAuthCache) Set(context.Context, string, auth.UserProfile, time.Duration) {}

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

// Get loads a cached user profile by token hash.
func (c *RedisAuthCache) Get(ctx context.Context, tokenHash string) (auth.UserProfile, bool) {
	if c == nil || c.store == nil || !c.store.Enabled() {
		return auth.UserProfile{}, false
	}
	var user auth.UserProfile
	ok, err := c.store.GetJSON(ctx, redisstore.AuthCacheKey(c.Version(), tokenHash), &user)
	if err != nil {
		c.logger.Warn("auth cache get failed", "error", err)
		return auth.UserProfile{}, false
	}
	return user, ok
}

// Set stores a cached user profile by token hash.
func (c *RedisAuthCache) Set(ctx context.Context, tokenHash string, user auth.UserProfile, ttl time.Duration) {
	if c == nil || c.store == nil || !c.store.Enabled() || ttl <= 0 {
		return
	}
	if c.ttl > 0 && ttl > c.ttl {
		ttl = c.ttl
	}
	if err := c.store.SetJSON(ctx, redisstore.AuthCacheKey(c.Version(), tokenHash), user, ttl); err != nil {
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

// SetVersion updates the local auth cache version.
func (c *RedisAuthCache) SetVersion(version int64) {
	if c == nil {
		return
	}
	c.version.Store(version)
}
