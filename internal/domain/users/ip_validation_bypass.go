package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"promptgate/backend/internal/domain/auth"
)

// IPValidationBypassEvent records an administrative change to an account's exception.
type IPValidationBypassEvent struct {
	ID          int64   `gorm:"primaryKey"`
	UserID      string  `gorm:"type:uuid;not null;index"`
	ActorType   string  `gorm:"not null"`
	ActorUserID *string `gorm:"type:uuid"`
	Previous    bool    `gorm:"not null"`
	Enabled     bool    `gorm:"not null"`
	CreatedAt   time.Time
}

// CanBypassIPValidation reads the current exception independently of the token cache.
func (s *Service) CanBypassIPValidation(ctx context.Context, id string) (bool, error) {
	var account User
	result := s.db.WithContext(ctx).
		Select("id").
		Where("id = ? AND type = ? AND bypass_ip_validation = ? AND is_active = ?", id, auth.UserTypeUser, true, true).
		Where("role IN ? AND (expires_at IS NULL OR expires_at > ?)",
			[]auth.AppRole{auth.RoleLimited, auth.RoleUser, auth.RoleAdmin}, s.nowUTC()).
		Limit(1).Find(&account)
	return result.RowsAffected == 1, result.Error
}

// SetIPValidationBypass updates a human account and records the transition atomically.
func (s *Service) SetIPValidationBypass(ctx context.Context, id string, enabled bool, actorUserID *string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account User
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "bypass_ip_validation").
			Where("id = ? AND type = ?", id, auth.UserTypeUser).
			Take(&account).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("load IP validation bypass: %w", err)
		}
		if account.BypassIPValidation == enabled {
			return nil
		}
		if err := tx.Model(&account).Update("bypass_ip_validation", enabled).Error; err != nil {
			return fmt.Errorf("update IP validation bypass: %w", err)
		}

		actorType := "admin_api_key"
		if actorUserID != nil {
			actorType = "admin_session"
		}
		event := IPValidationBypassEvent{
			UserID: id, ActorType: actorType, ActorUserID: actorUserID,
			Previous: !enabled, Enabled: enabled, CreatedAt: s.nowUTC(),
		}
		if err := tx.Create(&event).Error; err != nil {
			return fmt.Errorf("audit IP validation bypass: %w", err)
		}
		return nil
	})
}
