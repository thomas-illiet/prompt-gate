package monitoring

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
)

type MonitoringService struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name                string     `gorm:"not null;uniqueIndex" json:"name"`
	DisplayName         string     `gorm:"not null;default:''" json:"displayName"`
	URL                 string     `gorm:"not null" json:"url"`
	ExpectedStatusCode  int        `gorm:"not null;default:200" json:"expectedStatusCode"`
	IntervalSeconds     int        `gorm:"not null;default:60" json:"intervalSeconds"`
	Enabled             bool       `gorm:"not null;index" json:"enabled"`
	Status              Status     `gorm:"type:varchar(16);not null;default:'ok';index" json:"status"`
	LastCheckedAt       *time.Time `gorm:"column:last_checked_at;index" json:"lastCheckedAt"`
	LastStatusCode      *int       `gorm:"column:last_status_code" json:"lastStatusCode"`
	LastError           string     `gorm:"not null;default:''" json:"lastError"`
	LastLatencyMS       int64      `gorm:"column:last_latency_ms;not null;default:0" json:"lastLatencyMs"`
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutiveFailures"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// BeforeCreate assigns ids and normalized names before monitoring insertion.
func (s *MonitoringService) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.Name = normalizeName(s.Name)
	if s.Status == "" {
		s.Status = StatusOK
	}
	return nil
}

// BeforeUpdate normalizes monitoring service names before updates.
func (s *MonitoringService) BeforeUpdate(_ *gorm.DB) error {
	s.Name = normalizeName(s.Name)
	return nil
}

// normalizeName returns the canonical monitoring service name form.
func normalizeName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
