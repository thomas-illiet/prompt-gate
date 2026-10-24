package modellab

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var acquireLeaseScript = redis.NewScript(`
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
if redis.call("ZSCORE", KEYS[1], ARGV[3]) then
  return 1
end
if redis.call("ZCARD", KEYS[1]) >= tonumber(ARGV[4]) then
  return 0
end
redis.call("ZADD", KEYS[1], ARGV[2], ARGV[3])
redis.call("PEXPIRE", KEYS[1], ARGV[5])
return 1
`)

// AcquireLease enforces the server-side per-account parallel stream limit.
// The caller must release the lease when the upstream request terminates.
func (s *Service) AcquireLease(ctx context.Context, claims TicketClaims) error {
	if s.store == nil || !s.store.Enabled() || s.store.Client() == nil {
		return ErrTicketUnavailable
	}
	now := s.now().UTC()
	expiresAt := now.Add(s.leaseTTL)
	result, err := acquireLeaseScript.Run(
		ctx,
		s.store.Client(),
		[]string{leaseStoreKey(claims.Subject)},
		strconv.FormatInt(now.UnixMilli(), 10),
		strconv.FormatInt(expiresAt.UnixMilli(), 10),
		claims.ID,
		strconv.Itoa(s.maxConcurrent),
		strconv.FormatInt(s.leaseTTL.Milliseconds(), 10),
	).Int64()
	if err != nil {
		return fmt.Errorf("%w: acquire lease", ErrTicketUnavailable)
	}
	if result != 1 {
		return ErrParallelLimit
	}
	return nil
}

func (s *Service) ReleaseLease(ctx context.Context, claims TicketClaims) error {
	if s.store == nil || !s.store.Enabled() || s.store.Client() == nil {
		return ErrTicketUnavailable
	}
	if err := s.store.Client().ZRem(ctx, leaseStoreKey(claims.Subject), claims.ID).Err(); err != nil {
		return fmt.Errorf("%w: release lease", ErrTicketUnavailable)
	}
	return nil
}

func leaseStoreKey(subject string) string {
	return "promptgate:model-lab:active:v1:" + subject
}

// LeaseTTL exposes the configured recovery bound for integration tests and
// proxy shutdown handling without leaking any ticket material.
func (s *Service) LeaseTTL() time.Duration {
	return s.leaseTTL
}
