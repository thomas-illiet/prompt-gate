package subscriptions

import (
	"context"
	"log/slog"
	"time"
)

func (s *Service) logQuotaSync(ctx context.Context, store *RedisStore) {
	count, err := store.SyncQuotaStates(ctx, s)
	if err != nil {
		slog.Error("failed to sync subscription quota states", "error", err)
		return
	}
	if count > 0 {
		slog.Info("synced subscription quota states", "users", count)
	}
}

// StartQuotaStateSync starts a context-bound quota projection job. A non-positive
// interval disables the job and never reaches time.NewTicker.
func (s *Service) StartQuotaStateSync(ctx context.Context, store *RedisStore, interval time.Duration) {
	if store == nil || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.logQuotaSync(ctx, store)
			}
		}
	}()
}
