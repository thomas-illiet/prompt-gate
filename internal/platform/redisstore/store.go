package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	EventsChannel = "promptgate:config:events"
)

type Event struct {
	Domain    string    `json:"domain"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	client *redis.Client
	ttl    time.Duration
	logger *slog.Logger
}

// NewRequired creates a Redis-backed store and returns an error when the configured Redis cannot be used.
func NewRequired(ctx context.Context, rawURL string, ttl time.Duration, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("redis url is required")
	}
	opt, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Store{client: client, ttl: ttl, logger: logger}, nil
}

// Enabled reports whether Redis is available for this store.
func (s *Store) Enabled() bool {
	return s != nil && s.client != nil
}

// TTL returns the default cache TTL for this store.
func (s *Store) TTL() time.Duration {
	if s == nil || s.ttl <= 0 {
		return 5 * time.Minute
	}
	return s.ttl
}

// Client returns the underlying Redis client for features that need Redis commands
// outside the small JSON/cache helpers.
func (s *Store) Client() *redis.Client {
	if !s.Enabled() {
		return nil
	}
	return s.client
}

// Close closes the Redis client when one is configured.
func (s *Store) Close() error {
	if !s.Enabled() {
		return nil
	}
	return s.client.Close()
}

// Notify bumps a config version and publishes a reload event.
func (s *Store) Notify(ctx context.Context, domain string) {
	if !s.Enabled() {
		return
	}
	version, err := s.client.Incr(ctx, VersionKey(domain)).Result()
	if err != nil {
		s.logger.Warn("redis config version increment failed", "domain", domain, "error", err)
		return
	}
	if domain != "auth" {
		if err := s.client.Del(ctx, SnapshotKey(domain)).Err(); err != nil {
			s.logger.Warn("redis config snapshot invalidation failed", "domain", domain, "error", err)
		}
	}
	event := Event{Domain: domain, Version: version, CreatedAt: time.Now().UTC()}
	payload, err := json.Marshal(event)
	if err != nil {
		s.logger.Warn("redis config event marshal failed", "domain", domain, "error", err)
		return
	}
	if err := s.client.Publish(ctx, EventsChannel, payload).Err(); err != nil {
		s.logger.Warn("redis config publish failed", "domain", domain, "error", err)
		return
	}
	s.logger.Info("config reload event published", "domain", domain, "version", version)
}

// GetJSON loads and unmarshals a JSON value from Redis.
func (s *Store) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	raw, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, dst)
}

// GetDelJSON atomically loads, deletes, and unmarshals a JSON value from Redis.
func (s *Store) GetDelJSON(ctx context.Context, key string, dst any) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	raw, err := s.client.GetDel(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(raw, dst)
}

// SetJSON marshals and stores a JSON value in Redis.
func (s *Store) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = s.TTL()
	}
	return s.client.Set(ctx, key, payload, ttl).Err()
}

// Del removes keys from Redis when the store is enabled.
func (s *Store) Del(ctx context.Context, keys ...string) error {
	if !s.Enabled() || len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}

// Version returns the current version counter for a config domain.
func (s *Store) Version(ctx context.Context, domain string) (int64, error) {
	if !s.Enabled() {
		return 0, nil
	}
	value, err := s.client.Get(ctx, VersionKey(domain)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return value, err
}

// Subscribe streams config events from Redis until the context is done.
func (s *Store) Subscribe(ctx context.Context) <-chan Event {
	out := make(chan Event)
	if !s.Enabled() {
		close(out)
		return out
	}
	pubsub := s.client.Subscribe(ctx, EventsChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		if ctx.Err() == nil {
			s.logger.Warn("redis config event subscription failed", "error", err)
		}
		close(out)
		return out
	}
	go func() {
		defer close(out)
		defer pubsub.Close()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					s.logger.Warn("redis config event decode failed", "error", err)
					continue
				}
				select {
				case out <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

// VersionKey returns the Redis key for a config domain version.
func VersionKey(domain string) string {
	return fmt.Sprintf("promptgate:config:version:%s", domain)
}

// SnapshotKey returns the Redis key for a config domain snapshot.
func SnapshotKey(domain string) string {
	return fmt.Sprintf("promptgate:config:snapshot:%s", domain)
}

// AuthCacheKey returns the Redis key for a cached auth profile.
func AuthCacheKey(version int64, tokenHash string) string {
	return fmt.Sprintf("promptgate:proxy:auth:v%d:%s", version, tokenHash)
}

// SubscriptionAssignmentKey returns the Redis key for one cached subscription assignment.
func SubscriptionAssignmentKey(version int64, userID string) string {
	return fmt.Sprintf("promptgate:subscriptions:assignment:v%d:%s", version, userID)
}

// SubscriptionUsageZSetKey returns the Redis sorted-set key for usage buckets.
func SubscriptionUsageZSetKey(userID, window string) string {
	return fmt.Sprintf("promptgate:subscriptions:usage:%s:%s:z", userID, window)
}

// SubscriptionUsageHashKey returns the Redis hash key for usage bucket counters.
func SubscriptionUsageHashKey(userID, window string) string {
	return fmt.Sprintf("promptgate:subscriptions:usage:%s:%s:h", userID, window)
}

// SubscriptionActiveUsersKey returns the Redis set key for identities with quota usage.
func SubscriptionActiveUsersKey() string {
	return "promptgate:subscriptions:active_users"
}

// AuthSessionKey returns the Redis key for a browser session.
func AuthSessionKey(sessionID string) string {
	return fmt.Sprintf("promptgate:auth:session:%s", sessionID)
}

// AuthRequestKey returns the Redis key for an OIDC authorization request.
func AuthRequestKey(state string) string {
	return fmt.Sprintf("promptgate:auth:request:%s", state)
}
