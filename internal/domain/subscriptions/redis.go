package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/redis/go-redis/v9"
)

const (
	quotaWindow5HSeconds = int64((5 * time.Hour) / time.Second)
	quotaWindow7DSeconds = int64((7 * 24 * time.Hour) / time.Second)
	quotaBucket5HSeconds = int64(time.Minute / time.Second)
	quotaBucket7DSeconds = int64(time.Hour / time.Second)
)

var incrementUsageScript = redis.NewScript(`
local function increment(zset, hash, bucket, tokens, ttl, cutoff)
 local expired = redis.call("ZRANGEBYSCORE", zset, "-inf", cutoff)
 if #expired > 0 then
  redis.call("ZREM", zset, unpack(expired))
  redis.call("HDEL", hash, unpack(expired))
 end
 redis.call("ZADD", zset, bucket, bucket)
 redis.call("HINCRBY", hash, bucket, tokens)
 redis.call("EXPIRE", zset, ttl)
 redis.call("EXPIRE", hash, ttl)
end
increment(KEYS[1], KEYS[2], ARGV[1], ARGV[2], ARGV[3], ARGV[4])
increment(KEYS[4], KEYS[5], ARGV[5], ARGV[6], ARGV[7], ARGV[8])
redis.call("ZADD", KEYS[3], ARGV[10], ARGV[9])
return 1
`)

type RedisStore struct {
	store    *redisstore.Store
	service  *Service
	ttl      time.Duration
	logger   *slog.Logger
	version  atomic.Int64
	snapshot atomic.Value
}

type quotaWindow struct {
	name          string
	windowSeconds int64
	bucketSeconds int64
}

func NewRedisStore(store *redisstore.Store, service *Service, ttl time.Duration, logger *slog.Logger) *RedisStore {
	if logger == nil {
		logger = slog.Default()
	}
	if ttl <= 0 && store != nil {
		ttl = store.TTL()
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &RedisStore{store: store, service: service, ttl: ttl, logger: logger}
}

func (s *RedisStore) Enabled() bool {
	return s != nil && s.store != nil && s.store.Enabled()
}

func (s *RedisStore) SyncVersion(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	version, err := s.store.Version(ctx, configevents.DomainSubscriptions)
	if err != nil {
		s.logger.Warn("subscription config version load failed", "error", err)
		return
	}
	s.version.Store(version)
}

func (s *RedisStore) SetVersion(version int64) {
	s.version.Store(version)
}

func (s *RedisStore) Watch(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	events := s.store.Subscribe(ctx)
	s.logger.Info("subscription quota watcher started")
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Domain != configevents.DomainSubscriptions {
				continue
			}
			s.SetVersion(event.Version)
			if err := s.WarmSnapshot(ctx); err != nil {
				s.logger.Warn("subscription snapshot refresh failed", "version", event.Version, "error", err)
			}
		}
	}
}

func (s *RedisStore) WarmSnapshot(ctx context.Context) error {
	if !s.Enabled() || s.service == nil {
		return nil
	}
	snapshot, err := s.service.Snapshot(ctx)
	if err != nil {
		return err
	}
	if snapshot.Plans == nil {
		snapshot.Plans = map[string]PlanSnapshot{}
	}
	s.snapshot.Store(snapshot)
	return s.store.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainSubscriptions), snapshot, s.ttl)
}

func (s *RedisStore) CurrentQuota(ctx context.Context, userID string, now time.Time) (QuotaStatus, error) {
	now = now.UTC()
	plan, ok, err := s.ResolveEffectivePlan(ctx, userID)
	if err != nil {
		return QuotaStatus{}, err
	}
	if !ok {
		return QuotaStatus{HasSubscription: false}, nil
	}

	used5h, reset5h, err := s.windowUsage(ctx, userID, quota5H(), now)
	if err != nil {
		return QuotaStatus{}, err
	}
	used7d, reset7d, err := s.windowUsage(ctx, userID, quota7D(), now)
	if err != nil {
		return QuotaStatus{}, err
	}
	return quotaStatusFromPlan(plan, used5h, reset5h, used7d, reset7d), nil
}

func (s *RedisStore) ResolveEffectivePlan(ctx context.Context, userID string) (PlanSnapshot, bool, error) {
	snapshot, err := s.loadSnapshot(ctx)
	if err != nil {
		return PlanSnapshot{}, false, err
	}
	assignment, err := s.loadAssignment(ctx, userID)
	if err != nil {
		return PlanSnapshot{}, false, err
	}
	if !assignment.HasUser {
		return PlanSnapshot{}, false, nil
	}

	planID := assignment.PlanID
	if planID == nil {
		planID = snapshot.DefaultPlanID
	}
	if planID == nil {
		return PlanSnapshot{}, false, nil
	}
	plan, ok := snapshot.Plans[*planID]
	if ok {
		return plan, true, nil
	}
	if s.service == nil {
		return PlanSnapshot{}, false, nil
	}
	if err := s.WarmSnapshot(ctx); err != nil {
		return PlanSnapshot{}, false, err
	}
	snapshot, err = s.loadSnapshot(ctx)
	if err != nil {
		return PlanSnapshot{}, false, err
	}
	plan, ok = snapshot.Plans[*planID]
	return plan, ok, nil
}

func (s *RedisStore) IncrementUsage(ctx context.Context, userID string, tokens int64, now time.Time) error {
	if !s.Enabled() || tokens <= 0 {
		return nil
	}
	now = now.UTC()
	client := s.store.Client()
	return s.incrementWindow(ctx, client, userID, tokens, now, quota5H())
}

func (s *RedisStore) ActiveUserIDs(ctx context.Context) ([]string, error) {
	if !s.Enabled() {
		return nil, nil
	}
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour).Unix()
	ids, err := s.store.Client().ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     redisstore.SubscriptionActiveUsersZSetKey(),
		ByScore: true,
		Start:   strconv.FormatInt(cutoff, 10),
		Stop:    "+inf",
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil || len(ids) > 0 {
		return ids, err
	}
	// Read the legacy set during the rolling migration so existing usage is not skipped.
	legacy, legacyErr := s.store.Client().SMembers(ctx, redisstore.SubscriptionActiveUsersKey()).Result()
	if legacyErr != nil && !errors.Is(legacyErr, redis.Nil) {
		return nil, legacyErr
	}
	return legacy, nil
}

func (s *RedisStore) SyncQuotaStates(ctx context.Context, service *Service) (int64, error) {
	if !s.Enabled() || service == nil {
		return 0, nil
	}
	ids, err := s.ActiveUserIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("load active subscription users: %w", err)
	}
	now := time.Now().UTC()
	var count int64
	for _, userID := range ids {
		status, err := s.CurrentQuota(ctx, userID, now)
		if err != nil {
			return count, err
		}
		if err := service.UpsertQuotaState(ctx, userID, status, now); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *RedisStore) loadSnapshot(ctx context.Context) (Snapshot, error) {
	var snapshot Snapshot
	if cached := s.snapshot.Load(); cached != nil {
		return cached.(Snapshot), nil
	}
	if s.Enabled() {
		if ok, err := s.store.GetJSON(ctx, redisstore.SnapshotKey(configevents.DomainSubscriptions), &snapshot); err != nil {
			s.logger.Warn("subscription snapshot load failed", "error", err)
		} else if ok {
			if snapshot.Plans == nil {
				snapshot.Plans = map[string]PlanSnapshot{}
			}
			s.snapshot.Store(snapshot)
			return snapshot, nil
		}
	}
	if s.service == nil {
		return Snapshot{Plans: map[string]PlanSnapshot{}}, nil
	}
	loaded, err := s.service.Snapshot(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if s.Enabled() {
		_ = s.store.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainSubscriptions), loaded, s.ttl)
	}
	if loaded.Plans == nil {
		loaded.Plans = map[string]PlanSnapshot{}
	}
	s.snapshot.Store(loaded)
	return loaded, nil
}

func (s *RedisStore) loadAssignment(ctx context.Context, userID string) (AssignmentSnapshot, error) {
	key := redisstore.SubscriptionAssignmentKey(s.version.Load(), userID)
	var assignment AssignmentSnapshot
	if s.Enabled() {
		if ok, err := s.store.GetJSON(ctx, key, &assignment); err != nil {
			s.logger.Warn("subscription assignment load failed", "user_id", userID, "error", err)
		} else if ok {
			return assignment, nil
		}
	}
	if s.service == nil {
		return AssignmentSnapshot{}, nil
	}
	planID, hasUser, err := s.service.UserPlanID(ctx, userID)
	if err != nil {
		return AssignmentSnapshot{}, err
	}
	assignment = AssignmentSnapshot{HasUser: hasUser, PlanID: planID}
	if s.Enabled() {
		_ = s.store.SetJSON(ctx, key, assignment, s.ttl)
	}
	return assignment, nil
}

func (s *RedisStore) incrementWindow(ctx context.Context, client *redis.Client, userID string, tokens int64, now time.Time, window quotaWindow) error {
	bucket := bucketStart(now, window.bucketSeconds)
	cutoff := bucketStart(now.Add(-time.Duration(window.windowSeconds+window.bucketSeconds)*time.Second), window.bucketSeconds)
	ttl := window.windowSeconds + (2 * window.bucketSeconds)
	keys := []string{
		redisstore.SubscriptionUsageZSetKey(userID, window.name),
		redisstore.SubscriptionUsageHashKey(userID, window.name),
		redisstore.SubscriptionActiveUsersZSetKey(),
	}
	other := quota7D()
	keys = append(keys,
		redisstore.SubscriptionUsageZSetKey(userID, other.name),
		redisstore.SubscriptionUsageHashKey(userID, other.name),
	)
	otherBucket := bucketStart(now, other.bucketSeconds)
	otherCutoff := bucketStart(now.Add(-time.Duration(other.windowSeconds+other.bucketSeconds)*time.Second), other.bucketSeconds)
	return incrementUsageScript.Run(ctx, client, keys, bucket, tokens, ttl, cutoff, otherBucket, tokens, other.windowSeconds+(2*other.bucketSeconds), otherCutoff, userID, now.Unix()).Err()
}

func (s *RedisStore) windowUsage(ctx context.Context, userID string, window quotaWindow, now time.Time) (int64, *time.Time, error) {
	if !s.Enabled() {
		return 0, nil, nil
	}
	client := s.store.Client()
	zsetKey := redisstore.SubscriptionUsageZSetKey(userID, window.name)
	hashKey := redisstore.SubscriptionUsageHashKey(userID, window.name)
	cutoff := bucketStart(now.Add(-time.Duration(window.windowSeconds+window.bucketSeconds)*time.Second), window.bucketSeconds)

	expired, err := client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     zsetKey,
		ByScore: true,
		Start:   "-inf",
		Stop:    strconv.FormatInt(cutoff, 10),
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, nil, err
	}
	if len(expired) > 0 {
		pipe := client.Pipeline()
		pipe.ZRem(ctx, zsetKey, stringMembers(expired)...)
		pipe.HDel(ctx, hashKey, expired...)
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return 0, nil, fmt.Errorf("remove expired quota buckets: %w", err)
		}
	}

	members, err := client.ZRange(ctx, zsetKey, 0, -1).Result()
	if errors.Is(err, redis.Nil) || len(members) == 0 {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	values, err := client.HMGet(ctx, hashKey, members...).Result()
	if err != nil {
		return 0, nil, err
	}

	var used int64
	for _, raw := range values {
		switch value := raw.(type) {
		case string:
			next, _ := strconv.ParseInt(value, 10, 64)
			used += next
		case []byte:
			next, _ := strconv.ParseInt(string(value), 10, 64)
			used += next
		case int64:
			used += value
		}
	}
	if used == 0 {
		return 0, nil, nil
	}
	oldest, err := strconv.ParseInt(members[0], 10, 64)
	if err != nil {
		return used, nil, nil
	}
	resetAt := time.Unix(oldest+window.windowSeconds+window.bucketSeconds, 0).UTC()
	return used, &resetAt, nil
}

func bucketStart(now time.Time, bucketSeconds int64) int64 {
	unix := now.UTC().Unix()
	return unix - (unix % bucketSeconds)
}

func stringMembers(values []string) []any {
	members := make([]any, 0, len(values))
	for _, value := range values {
		members = append(members, value)
	}
	return members
}

func quota5H() quotaWindow {
	return quotaWindow{
		name:          Window5H,
		windowSeconds: quotaWindow5HSeconds,
		bucketSeconds: quotaBucket5HSeconds,
	}
}

func quota7D() quotaWindow {
	return quotaWindow{
		name:          Window7D,
		windowSeconds: quotaWindow7DSeconds,
		bucketSeconds: quotaBucket7DSeconds,
	}
}
