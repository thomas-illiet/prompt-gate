package firewall

import (
	"context"
	"errors"

	"promptgate/backend/internal/platform/configevents"

	"gorm.io/gorm"
)

var (
	ErrRuleNotFound       = errors.New("firewall rule not found")
	ErrInvalidAddress     = errors.New("invalid_ipv4_address")
	ErrInvalidAction      = errors.New("invalid_action")
	ErrPriorityOutOfRange = errors.New("priority_out_of_range")
	ErrPriorityConflict   = errors.New("priority_conflict")
	ErrInvalidDirection   = errors.New("invalid_direction")
	ErrInvalidSort        = errors.New("invalid_sort")
)

type Service struct {
	db       *gorm.DB
	notifier configevents.Notifier
}

// NewService creates a firewall service backed by GORM.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db, notifier: configevents.NoopNotifier{}}
}

// SetNotifier configures config event publication after firewall mutations.
func (s *Service) SetNotifier(notifier configevents.Notifier) {
	if notifier == nil {
		notifier = configevents.NoopNotifier{}
	}
	s.notifier = notifier
}

// AutoMigrate migrates firewall tables.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&FirewallRule{})
}
