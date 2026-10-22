package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
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
local event_at_micros = tonumber(ARGV[11])
local reset_cutoff_micros = tonumber(redis.call("HGET", KEYS[6], "cutoff") or "0")
if event_at_micros <= reset_cutoff_micros then
 return 0
end

local function update_active_user(zset, user_id, score)
 local current = redis.call("ZSCORE", zset, user_id)
 if not current or tonumber(score) > tonumber(current) then
  redis.call("ZADD", zset, score, user_id)
 end
end

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
update_active_user(KEYS[3], ARGV[9], ARGV[10])
return 1
`)

var applyQuotaResetScript = redis.NewScript(`
local function update_active_user(zset, user_id, score)
 local current = redis.call("ZSCORE", zset, user_id)
 if not current or tonumber(score) > tonumber(current) then
  redis.call("ZADD", zset, score, user_id)
 end
end

local requested_sequence = tonumber(ARGV[1])
local current_sequence = tonumber(redis.call("HGET", KEYS[6], "sequence") or "0")
local current_cutoff = tonumber(redis.call("HGET", KEYS[6], "cutoff") or "0")

if requested_sequence <= current_sequence then
 return {0, current_sequence, current_cutoff}
end

local redis_time = redis.call("TIME")
local effective_cutoff = (tonumber(redis_time[1]) * 1000000) + tonumber(redis_time[2])
if current_cutoff > effective_cutoff then
 effective_cutoff = current_cutoff
end

redis.call("DEL", KEYS[1], KEYS[2], KEYS[3], KEYS[4])
redis.call("HSET", KEYS[6], "sequence", requested_sequence, "cutoff", effective_cutoff)
update_active_user(KEYS[5], ARGV[2], math.floor(effective_cutoff / 1000000))
redis.call("SREM", KEYS[7], ARGV[2])
return {1, requested_sequence, effective_cutoff}
`)

type RedisStore struct {
	store     *redisstore.Store
	service   *Service
	ttl       time.Duration
	logger    *slog.Logger
	version   atomic.Int64
	snapshot  atomic.Value
	refreshMu sync.Mutex
}

type quotaWindow struct {
	name          string
	windowSeconds int64
	bucketSeconds int64
}

// QuotaResetApplication reports the durable Redis barrier observed for a reset.
type QuotaResetApplication struct {
	Advanced bool
	Sequence int64
	Cutoff   time.Time
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
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.version.Store(version)
}

func (s *RedisStore) SetVersion(version int64) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.version.Store(version)
}

// RefreshIfStale synchronizes the process-local plan snapshot and assignment
// version when Redis reports a different authoritative version. A mutex keeps
// concurrent API reads and watcher events from completing refreshes out of
// order. The version is checked again after warming so the method only returns
// once the snapshot corresponds to a stable Redis version.
func (s *RedisStore) RefreshIfStale(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}

	version, err := s.store.Version(ctx, configevents.DomainSubscriptions)
	if err != nil {
		return fmt.Errorf("load subscription config version: %w", err)
	}
	if version == s.version.Load() {
		return nil
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	version, err = s.store.Version(ctx, configevents.DomainSubscriptions)
	if err != nil {
		return fmt.Errorf("reload subscription config version: %w", err)
	}
	if version == s.version.Load() {
		return nil
	}
	previousVersion := s.version.Load()

	for {
		if version < previousVersion {
			if err := s.clearAssignmentVersion(ctx, version); err != nil {
				return fmt.Errorf("clear reset subscription assignments: %w", err)
			}
		}
		if err := s.warmSnapshotLocked(ctx); err != nil {
			return fmt.Errorf("refresh subscription snapshot: %w", err)
		}

		latest, err := s.store.Version(ctx, configevents.DomainSubscriptions)
		if err != nil {
			return fmt.Errorf("verify subscription config version: %w", err)
		}
		if latest != version {
			version = latest
			continue
		}
		s.version.Store(version)
		return nil
	}
}

func (s *RedisStore) clearAssignmentVersion(ctx context.Context, version int64) error {
	client := s.store.Client()
	pattern := redisstore.SubscriptionAssignmentKey(version, "*")
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (s *RedisStore) Watch(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	events := s.store.Subscribe(ctx)
	if ctx.Err() != nil {
		return
	}

	// Subscribe is established synchronously. Refresh after that point so a
	// mutation published between process startup and watcher startup cannot
	// leave this process on a stale assignment version or plan snapshot.
	if err := s.RefreshIfStale(ctx); err != nil {
		s.logger.Warn("subscription snapshot catch-up failed", "error", err)
	}
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
			if err := s.RefreshIfStale(ctx); err != nil {
				s.logger.Warn("subscription snapshot refresh failed", "version", event.Version, "error", err)
			}
		}
	}
}

func (s *RedisStore) WarmSnapshot(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.warmSnapshotLocked(ctx)
}

func (s *RedisStore) warmSnapshotLocked(ctx context.Context) error {
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

// ApplyQuotaReset atomically clears both quota windows. Redis chooses the cutoff
// at the script's linearization point; the sequence makes retries idempotent and
// prevents an older pending event from clearing usage recorded after a reset.
func (s *RedisStore) ApplyQuotaReset(ctx context.Context, userID string, sequence int64) (QuotaResetApplication, error) {
	if !s.Enabled() {
		return QuotaResetApplication{}, errors.New("subscription quota store unavailable")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" || sequence <= 0 {
		return QuotaResetApplication{}, errors.New("invalid quota reset application")
	}
	values, err := applyQuotaResetScript.Run(
		ctx,
		s.store.Client(),
		[]string{
			redisstore.SubscriptionUsageZSetKey(userID, Window5H),
			redisstore.SubscriptionUsageHashKey(userID, Window5H),
			redisstore.SubscriptionUsageZSetKey(userID, Window7D),
			redisstore.SubscriptionUsageHashKey(userID, Window7D),
			redisstore.SubscriptionActiveUsersZSetKey(),
			redisstore.SubscriptionQuotaResetStateKey(userID),
			redisstore.SubscriptionActiveUsersKey(),
		},
		sequence,
		userID,
	).Slice()
	if err != nil {
		return QuotaResetApplication{}, fmt.Errorf("apply quota reset: %w", err)
	}
	if len(values) != 3 {
		return QuotaResetApplication{}, fmt.Errorf("apply quota reset: unexpected result length %d", len(values))
	}
	advanced, err := quotaResetScriptInt64(values[0])
	if err != nil {
		return QuotaResetApplication{}, fmt.Errorf("decode quota reset applied flag: %w", err)
	}
	appliedSequence, err := quotaResetScriptInt64(values[1])
	if err != nil {
		return QuotaResetApplication{}, fmt.Errorf("decode quota reset sequence: %w", err)
	}
	appliedCutoff, err := quotaResetScriptInt64(values[2])
	if err != nil {
		return QuotaResetApplication{}, fmt.Errorf("decode quota reset cutoff: %w", err)
	}
	return QuotaResetApplication{
		Advanced: advanced == 1,
		Sequence: appliedSequence,
		Cutoff:   time.UnixMicro(appliedCutoff).UTC(),
	}, nil
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
	var count int64
	for _, userID := range ids {
		// Timestamp each observation before reading Redis. A concurrent reset
		// obtains a later timestamp after its Lua barrier and wins monotonically.
		observedAt := time.Now().UTC()
		status, err := s.CurrentQuota(ctx, userID, observedAt)
		if err != nil {
			return count, err
		}
		if err := service.UpsertQuotaState(ctx, userID, status, observedAt); err != nil {
			return count, err
		}
		count++
	}
	if err := service.PruneQuotaStates(ctx, ids); err != nil {
		return count, err
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
		redisstore.SubscriptionQuotaResetStateKey(userID),
	)
	otherBucket := bucketStart(now, other.bucketSeconds)
	otherCutoff := bucketStart(now.Add(-time.Duration(other.windowSeconds+other.bucketSeconds)*time.Second), other.bucketSeconds)
	return incrementUsageScript.Run(ctx, client, keys, bucket, tokens, ttl, cutoff, otherBucket, tokens, other.windowSeconds+(2*other.bucketSeconds), otherCutoff, userID, now.Unix(), now.UnixMicro()).Err()
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

func quotaResetScriptInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
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
