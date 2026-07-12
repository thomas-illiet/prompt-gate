package tokens

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// RevokeUserTokensTx marks all non-revoked tokens for the given users as revoked inside tx.
func (s *Service) RevokeUserTokensTx(ctx context.Context, tx *gorm.DB, userIDs []string, revokedAt time.Time) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}
	if tx == nil {
		tx = s.db
	}

	result := tx.WithContext(ctx).
		Model(&Token{}).
		Where("user_id IN ? AND revoked_at IS NULL", userIDs).
		Update("revoked_at", revokedAt.UTC())
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// StartCleanup starts a context-bound job that periodically marks expired tokens.
func (s *Service) StartCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
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
				if err := s.markExpired(ctx); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("failed to mark expired tokens", "error", err)
				}
			}
		}
	}()
}

func (s *Service) markExpired(ctx context.Context) error {
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&Token{}).
		Where("expires_at < ? AND expired_at IS NULL", now).
		Update("expired_at", now)
	if result.Error != nil {
		return fmt.Errorf("mark expired tokens: %w", result.Error)
	}
	return nil
}
