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

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	quotaWindow5HSeconds = int64((5 * time.Hour) / time.Second)
	quotaWindow7DSeconds = int64((7 * 24 * time.Hour) / time.Second)
	quotaBucket5HSeconds = int64(time.Minute / time.Second)
	quotaBucket7DSeconds = int64(time.Hour / time.Second)
)

var incrementUsageScript = redis.NewScript(`
local redis_time = redis.call("TIME")
local recorded_at_micros = (tonumber(redis_time[1]) * 1000000) + tonumber(redis_time[2])
local reset_cutoff_micros = tonumber(redis.call("HGET", KEYS[6], "cutoff") or "0")
if recorded_at_micros <= reset_cutoff_micros then
 recorded_at_micros = reset_cutoff_micros + 1
end
local recorded_at_seconds = math.floor(recorded_at_micros / 1000000)

local function update_active_user(zset, user_id, score)
 local current = redis.call("ZSCORE", zset, user_id)
 if not current or tonumber(score) > tonumber(current) then
  redis.call("ZADD", zset, score, user_id)
 end
end

local function bucket_start(timestamp, bucket_seconds)
 return timestamp - (timestamp % bucket_seconds)
end

local function increment(zset, hash, tokens, window_seconds, bucket_seconds)
 local bucket = bucket_start(recorded_at_seconds, bucket_seconds)
 local cutoff = bucket_start(recorded_at_seconds - window_seconds - bucket_seconds, bucket_seconds)
 local ttl = window_seconds + (2 * bucket_seconds)
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
increment(KEYS[1], KEYS[2], ARGV[1], ARGV[2], ARGV[3])
increment(KEYS[4], KEYS[5], ARGV[1], ARGV[4], ARGV[5])
update_active_user(KEYS[3], ARGV[6], recorded_at_seconds)
return {1, recorded_at_micros}
`)

var currentQuotaScript = redis.NewScript(`
local redis_time = redis.call("TIME")
local wall_time_micros = (tonumber(redis_time[1]) * 1000000) + tonumber(redis_time[2])
local observed_at_micros = wall_time_micros
local previous_observation = tonumber(redis.call("GET", KEYS[5]) or "0")
if observed_at_micros <= previous_observation then
 observed_at_micros = previous_observation + 1
end
redis.call("SET", KEYS[5], observed_at_micros)
local wall_time_seconds = math.floor(wall_time_micros / 1000000)

local function snapshot_window(zset, hash, window_seconds, bucket_seconds)
 local cutoff_time = wall_time_seconds - window_seconds - bucket_seconds
 local cutoff = cutoff_time - (cutoff_time % bucket_seconds)
 local expired = redis.call("ZRANGEBYSCORE", zset, "-inf", cutoff)
 if #expired > 0 then
  redis.call("ZREM", zset, unpack(expired))
  redis.call("HDEL", hash, unpack(expired))
 end

 local members = redis.call("ZRANGE", zset, 0, -1)
 if #members == 0 then
  return 0, 0
 end
 local values = redis.call("HMGET", hash, unpack(members))
 local used = 0
 for _, value in ipairs(values) do
  used = used + tonumber(value or "0")
 end
 if used == 0 then
  return 0, 0
 end
 return used, tonumber(members[1]) or 0
end

local used_5h, oldest_5h = snapshot_window(KEYS[1], KEYS[2], ARGV[1], ARGV[2])
local used_7d, oldest_7d = snapshot_window(KEYS[3], KEYS[4], ARGV[3], ARGV[4])
return {observed_at_micros, used_5h, oldest_5h, used_7d, oldest_7d}
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

var deleteUserQuotaStateScript = redis.NewScript(`
redis.call("DEL", KEYS[1], KEYS[2], KEYS[3], KEYS[4], KEYS[5], KEYS[6])
redis.call("ZREM", KEYS[7], ARGV[1])
redis.call("SREM", KEYS[8], ARGV[1])
return 1
`)

var migrateLegacyActiveUsersScript = redis.NewScript(`
local members = redis.call("SRANDMEMBER", KEYS[2], tonumber(ARGV[2]))
if #members == 0 then
 return members
end
for _, user_id in ipairs(members) do
 local current = redis.call("ZSCORE", KEYS[1], user_id)
 if not current or tonumber(ARGV[1]) > tonumber(current) then
  redis.call("ZADD", KEYS[1], ARGV[1], user_id)
 end
end
redis.call("SREM", KEYS[2], unpack(members))
return members
`)

type RedisStore struct {
	store              *redisstore.Store
	service            *Service
	ttl                time.Duration
	logger             *slog.Logger
	version            atomic.Int64
	snapshot           atomic.Value
	refreshMu          sync.Mutex
	deletedUserScanMu  sync.Mutex
	deletedUserZCursor uint64
	deletedUserSCursor uint64
	deletedUserScanSet bool
}

type quotaUsageSnapshot struct {
	observedAt time.Time
	used5H     int64
	reset5HAt  *time.Time
	used7D     int64
	reset7DAt  *time.Time
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
	status, _, err := s.currentQuota(ctx, userID, now)
	return status, err
}

func (s *RedisStore) currentQuota(ctx context.Context, userID string, now time.Time) (QuotaStatus, time.Time, error) {
	now = now.UTC()
	plan, ok, err := s.ResolveEffectivePlan(ctx, userID)
	if err != nil {
		return QuotaStatus{}, time.Time{}, err
	}
	usage, err := s.loadQuotaUsageSnapshot(ctx, userID, now)
	if err != nil {
		return QuotaStatus{}, time.Time{}, err
	}
	if !ok {
		return QuotaStatus{HasSubscription: false}, usage.observedAt, nil
	}
	return quotaStatusFromPlan(
		plan,
		usage.used5H,
		usage.reset5HAt,
		usage.used7D,
		usage.reset7DAt,
	), usage.observedAt, nil
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

// IncrementUsage records tokens at Redis' linearization time. The timestamp
// argument is retained for API compatibility but is deliberately not trusted:
// proxy clock skew must never make usage recorded after a reset disappear.
func (s *RedisStore) IncrementUsage(ctx context.Context, userID string, tokens int64, _ time.Time) error {
	if !s.Enabled() || tokens <= 0 {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("subscription usage user is required")
	}
	values, err := incrementUsageScript.Run(
		ctx,
		s.store.Client(),
		[]string{
			redisstore.SubscriptionUsageZSetKey(userID, Window5H),
			redisstore.SubscriptionUsageHashKey(userID, Window5H),
			redisstore.SubscriptionActiveUsersZSetKey(),
			redisstore.SubscriptionUsageZSetKey(userID, Window7D),
			redisstore.SubscriptionUsageHashKey(userID, Window7D),
			redisstore.SubscriptionQuotaResetStateKey(userID),
		},
		tokens,
		quotaWindow5HSeconds,
		quotaBucket5HSeconds,
		quotaWindow7DSeconds,
		quotaBucket7DSeconds,
		userID,
	).Slice()
	if err != nil {
		return fmt.Errorf("increment subscription usage: %w", err)
	}
	if len(values) != 2 {
		return fmt.Errorf("increment subscription usage: unexpected result length %d", len(values))
	}
	recorded, err := quotaResetScriptInt64(values[0])
	if err != nil {
		return fmt.Errorf("decode subscription usage recorded flag: %w", err)
	}
	if recorded != 1 {
		return errors.New("increment subscription usage: Redis rejected usage")
	}
	if _, err := quotaResetScriptInt64(values[1]); err != nil {
		return fmt.Errorf("decode subscription usage timestamp: %w", err)
	}
	return nil
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
	redisNow, err := s.store.Client().Time(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("load Redis active-user clock: %w", err)
	}
	// Usage remains authoritative until the final seven-day bucket expires.
	// Retain its projection for the same conservative window so SQL cannot be
	// pruned while CurrentQuota still counts the bucket.
	cutoff := redisNow.UTC().Add(-time.Duration(quotaWindow7DSeconds+quotaBucket7DSeconds) * time.Second).Unix()
	ids, err := s.store.Client().ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     redisstore.SubscriptionActiveUsersZSetKey(),
		ByScore: true,
		Start:   strconv.FormatInt(cutoff, 10),
		Stop:    "+inf",
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	// Atomically migrate a bounded legacy batch after adding every selected
	// identity to the zset. Returning the migrated members preserves this run's
	// sync, while repeated runs drain the set without a full-history SMEMBERS.
	legacy, legacyErr := migrateLegacyActiveUsersScript.Run(
		ctx,
		s.store.Client(),
		[]string{
			redisstore.SubscriptionActiveUsersZSetKey(),
			redisstore.SubscriptionActiveUsersKey(),
		},
		redisNow.UTC().Unix(),
		100,
	).StringSlice()
	if legacyErr != nil && !errors.Is(legacyErr, redis.Nil) {
		return nil, legacyErr
	}
	seen := make(map[string]struct{}, len(ids)+len(legacy))
	combined := make([]string, 0, len(ids)+len(legacy))
	for _, userID := range append(ids, legacy...) {
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		combined = append(combined, userID)
	}
	return combined, nil
}

// DeleteUserQuotaState atomically removes all Redis quota and reset state for one identity.
func (s *RedisStore) DeleteUserQuotaState(ctx context.Context, userID string) error {
	if !s.Enabled() {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("subscription quota user is required")
	}
	result, err := deleteUserQuotaStateScript.Run(
		ctx,
		s.store.Client(),
		[]string{
			redisstore.SubscriptionUsageZSetKey(userID, Window5H),
			redisstore.SubscriptionUsageHashKey(userID, Window5H),
			redisstore.SubscriptionUsageZSetKey(userID, Window7D),
			redisstore.SubscriptionUsageHashKey(userID, Window7D),
			redisstore.SubscriptionQuotaResetStateKey(userID),
			redisstore.SubscriptionAssignmentKey(s.version.Load(), userID),
			redisstore.SubscriptionActiveUsersZSetKey(),
			redisstore.SubscriptionActiveUsersKey(),
		},
		userID,
	).Int64()
	if err != nil {
		return fmt.Errorf("delete subscription quota state: %w", err)
	}
	if result != 1 {
		return fmt.Errorf("delete subscription quota state: unexpected result %d", result)
	}
	return nil
}

func (s *RedisStore) existingTrackedUserIDs(ctx context.Context, service *Service, rawIDs []string) ([]string, error) {
	ids := make([]string, 0, len(rawIDs))
	seen := make(map[string]struct{}, len(rawIDs))
	for _, rawID := range rawIDs {
		userID := strings.TrimSpace(rawID)
		if userID == "" {
			continue
		}
		parsed, err := uuid.Parse(userID)
		if err != nil || parsed.String() != userID {
			if err := s.DeleteUserQuotaState(ctx, userID); err != nil {
				return nil, err
			}
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		ids = append(ids, userID)
	}

	existingIDs, err := service.existingUserIDSet(ctx, ids)
	if err != nil {
		return nil, err
	}
	existing := make([]string, 0, len(existingIDs))
	for _, userID := range ids {
		if _, exists := existingIDs[userID]; exists {
			existing = append(existing, userID)
			continue
		}
		if err := s.DeleteUserQuotaState(ctx, userID); err != nil {
			return nil, err
		}
	}
	return existing, nil
}

func (s *RedisStore) pruneDeletedUserQuotaStates(ctx context.Context, service *Service) error {
	// IncrementUsage and ApplyQuotaReset both add the owner to the active-user
	// zset. A cursorized batch over the full zset (not only the seven-day
	// projection window) therefore discovers reset-only state even after years
	// of inactivity without walking every historical owner every five minutes.
	// The legacy set is scanned on alternating runs so it cannot starve a large
	// current zset. Direct cleanup on user deletion is the fast path; this is a
	// bounded, retryable backstop for Redis outages and older deletion paths.
	const scanSize = 100

	s.deletedUserScanMu.Lock()
	defer s.deletedUserScanMu.Unlock()
	client := s.store.Client()

	if s.deletedUserScanSet {
		ids, next, err := client.SScan(
			ctx,
			redisstore.SubscriptionActiveUsersKey(),
			s.deletedUserSCursor,
			"*",
			scanSize,
		).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return fmt.Errorf("scan legacy subscription users: %w", err)
		}
		if _, err := s.existingTrackedUserIDs(ctx, service, ids); err != nil {
			return err
		}
		s.deletedUserSCursor = next
	} else {
		entries, next, err := client.ZScan(
			ctx,
			redisstore.SubscriptionActiveUsersZSetKey(),
			s.deletedUserZCursor,
			"*",
			scanSize,
		).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return fmt.Errorf("scan tracked subscription users: %w", err)
		}
		ids := make([]string, 0, (len(entries)+1)/2)
		for index := 0; index < len(entries); index += 2 {
			ids = append(ids, entries[index])
		}
		if _, err := s.existingTrackedUserIDs(ctx, service, ids); err != nil {
			return err
		}
		s.deletedUserZCursor = next
	}
	s.deletedUserScanSet = !s.deletedUserScanSet
	return nil
}

func (s *RedisStore) pruneQuotaStateProjections(ctx context.Context, service *Service, activeIDs []string) error {
	legacyRemaining, err := s.store.Client().SCard(ctx, redisstore.SubscriptionActiveUsersKey()).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("inspect legacy subscription users: %w", err)
	}
	if legacyRemaining > 0 {
		// Some legacy identities have not reached the bounded migration batch yet.
		// Pruning now would make their persisted quota projections flicker away.
		return nil
	}
	return service.PruneQuotaStates(ctx, activeIDs)
}

func (s *RedisStore) SyncQuotaStates(ctx context.Context, service *Service) (int64, error) {
	if !s.Enabled() || service == nil {
		return 0, nil
	}
	if err := s.pruneDeletedUserQuotaStates(ctx, service); err != nil {
		return 0, err
	}

	ids, err := s.ActiveUserIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("load active subscription users: %w", err)
	}
	ids, err = s.existingTrackedUserIDs(ctx, service, ids)
	if err != nil {
		return 0, fmt.Errorf("filter active subscription users: %w", err)
	}
	var count int64
	for _, userID := range ids {
		status, observedAt, err := s.currentQuota(ctx, userID, time.Now().UTC())
		if err != nil {
			return count, err
		}
		if err := service.UpsertQuotaState(ctx, userID, status, observedAt); err != nil {
			return count, err
		}
		count++
	}
	if err := s.pruneQuotaStateProjections(ctx, service, ids); err != nil {
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

func (s *RedisStore) loadQuotaUsageSnapshot(ctx context.Context, userID string, now time.Time) (quotaUsageSnapshot, error) {
	snapshot := quotaUsageSnapshot{observedAt: now.UTC()}
	if !s.Enabled() {
		return snapshot, nil
	}
	values, err := currentQuotaScript.Run(
		ctx,
		s.store.Client(),
		[]string{
			redisstore.SubscriptionUsageZSetKey(userID, Window5H),
			redisstore.SubscriptionUsageHashKey(userID, Window5H),
			redisstore.SubscriptionUsageZSetKey(userID, Window7D),
			redisstore.SubscriptionUsageHashKey(userID, Window7D),
			redisstore.SubscriptionQuotaObservationClockKey(),
		},
		quotaWindow5HSeconds,
		quotaBucket5HSeconds,
		quotaWindow7DSeconds,
		quotaBucket7DSeconds,
	).Slice()
	if err != nil {
		return quotaUsageSnapshot{}, fmt.Errorf("load subscription quota snapshot: %w", err)
	}
	if len(values) != 5 {
		return quotaUsageSnapshot{}, fmt.Errorf("load subscription quota snapshot: unexpected result length %d", len(values))
	}
	decoded := make([]int64, len(values))
	for index, value := range values {
		decoded[index], err = quotaResetScriptInt64(value)
		if err != nil {
			return quotaUsageSnapshot{}, fmt.Errorf("decode subscription quota snapshot field %d: %w", index, err)
		}
	}
	snapshot.observedAt = time.UnixMicro(decoded[0]).UTC()
	snapshot.used5H = decoded[1]
	snapshot.reset5HAt = quotaWindowResetAt(decoded[1], decoded[2], quotaWindow5HSeconds, quotaBucket5HSeconds)
	snapshot.used7D = decoded[3]
	snapshot.reset7DAt = quotaWindowResetAt(decoded[3], decoded[4], quotaWindow7DSeconds, quotaBucket7DSeconds)
	return snapshot, nil
}

func quotaWindowResetAt(used, oldestBucket, windowSeconds, bucketSeconds int64) *time.Time {
	if used == 0 || oldestBucket == 0 {
		return nil
	}
	resetAt := time.Unix(oldestBucket+windowSeconds+bucketSeconds, 0).UTC()
	return &resetAt
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
