package modellab

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var reserveTicketsScript = redis.NewScript(`
redis.call("ZREMRANGEBYSCORE", KEYS[1], "-inf", ARGV[1])
local requested = #ARGV - 4
if redis.call("ZCARD", KEYS[1]) + requested > tonumber(ARGV[3]) then
  return 0
end
for index = 5, #ARGV do
  redis.call("ZADD", KEYS[1], ARGV[2], ARGV[index])
end
local current_ttl = redis.call("PTTL", KEYS[1])
if current_ttl < tonumber(ARGV[4]) then
  redis.call("PEXPIRE", KEYS[1], ARGV[4])
end
return 1
`)

func (s *Service) reserveTicketIDs(
	ctx context.Context,
	subject string,
	ids []string,
	now time.Time,
	expiresAt time.Time,
) error {
	if len(ids) == 0 || s.store == nil || !s.store.Enabled() || s.store.Client() == nil {
		return ErrTicketUnavailable
	}
	ttl := expiresAt.Sub(now)
	if ttl <= 0 {
		return ErrTicketUnavailable
	}
	args := make([]any, 0, len(ids)+4)
	args = append(args,
		strconv.FormatInt(now.UnixMilli(), 10),
		strconv.FormatInt(expiresAt.UnixMilli(), 10),
		strconv.Itoa(s.maxOutstanding),
		strconv.FormatInt(ttl.Milliseconds(), 10),
	)
	for _, id := range ids {
		args = append(args, id)
	}
	result, err := reserveTicketsScript.Run(
		ctx,
		s.store.Client(),
		[]string{ticketReservationKey(subject)},
		args...,
	).Int64()
	if err != nil {
		return fmt.Errorf("%w: reserve tickets", ErrTicketUnavailable)
	}
	if result != 1 {
		return ErrTicketIssueLimit
	}
	return nil
}

func (s *Service) releaseTicketIDs(ctx context.Context, subject string, ids ...string) {
	if len(ids) == 0 || s.store == nil || s.store.Client() == nil {
		return
	}
	values := make([]any, 0, len(ids))
	for _, id := range ids {
		values = append(values, id)
	}
	_ = s.store.Client().ZRem(ctx, ticketReservationKey(subject), values...).Err()
}

func ticketReservationKey(subject string) string {
	return "promptgate:model-lab:ticket-reservations:v1:" + subject
}
