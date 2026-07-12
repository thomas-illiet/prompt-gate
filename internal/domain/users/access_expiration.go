package users

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"

	"gorm.io/gorm"
)

// ExpireAccess removes roles whose access expiration date has passed and revokes their tokens.
func (s *Service) ExpireAccess(ctx context.Context, now time.Time) (int64, error) {
	now = now.UTC()
	var expiredIDs []string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&User{}).
			Where("expires_at IS NOT NULL AND expires_at <= ? AND role <> ?", now, auth.RoleNone).
			Pluck("id", &expiredIDs).Error; err != nil {
			return fmt.Errorf("list expired user access: %w", err)
		}
		if len(expiredIDs) == 0 {
			return nil
		}

		if err := tx.Model(&User{}).
			Where("id IN ?", expiredIDs).
			Updates(map[string]any{"role": auth.RoleNone, "expires_at": nil}).Error; err != nil {
			return fmt.Errorf("expire user access: %w", err)
		}

		if s.tokenRevoker != nil {
			if _, err := s.tokenRevoker.RevokeUserTokensTx(ctx, tx, expiredIDs, now); err != nil {
				return fmt.Errorf("revoke expired user tokens: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(expiredIDs) == 0 {
		return 0, nil
	}

	s.notifier.Notify(ctx, configevents.DomainAuth)
	return int64(len(expiredIDs)), nil
}

// StartAccessExpiration starts a background goroutine that periodically expires user access.
func (s *Service) StartAccessExpiration(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}

	go func() {
		s.expireAccessLog(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.expireAccessLog(ctx)
			}
		}
	}()
}

func (s *Service) expireAccessLog(ctx context.Context) {
	count, err := s.ExpireAccess(ctx, time.Now().UTC())
	if err != nil {
		slog.Error("failed to expire user access", "error", err)
		return
	}
	if count > 0 {
		slog.Info("expired user access", "users", count)
	}
}
