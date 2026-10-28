package tokens

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"promptgate/backend/internal/domain/notifications"

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

	var records []Token
	if err := tx.WithContext(ctx).Where("user_id IN ? AND revoked_at IS NULL", userIDs).Find(&records).Error; err != nil {
		return 0, err
	}
	result := tx.WithContext(ctx).
		Model(&Token{}).
		Where("user_id IN ? AND revoked_at IS NULL", userIDs).
		Update("revoked_at", revokedAt.UTC())
	if result.Error != nil {
		return 0, result.Error
	}
	for _, record := range records {
		if err := s.enqueueTokenNotification(ctx, tx, record, notifications.KindTokenRevoked, "revoked"); err != nil {
			return 0, err
		}
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
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var records []Token
		if err := tx.Where("expires_at < ? AND expired_at IS NULL AND revoked_at IS NULL", now).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		ids := make([]string, len(records))
		for i := range records {
			ids[i] = records[i].ID
		}
		if err := tx.Model(&Token{}).Where("id IN ?", ids).Update("expired_at", now).Error; err != nil {
			return fmt.Errorf("mark expired tokens: %w", err)
		}
		for _, record := range records {
			if err := s.enqueueTokenNotification(ctx, tx, record, notifications.KindTokenExpired, "expired"); err != nil {
				return err
			}
		}
		return nil
	})
}
