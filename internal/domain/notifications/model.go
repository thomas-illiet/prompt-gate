package notifications

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Kind identifies the business reason for an email message.
type Kind string

const (
	KindWelcome        Kind = "welcome"
	KindQuotaExhausted Kind = "quota_exhausted"
	KindQuotaReset     Kind = "quota_reset_granted"
	KindGroupChanged   Kind = "group_changed"
	KindTokenCreated   Kind = "token_created"
	KindTokenExpired   Kind = "token_expired"
	KindTokenRevoked   Kind = "token_revoked"
	KindMonthlyReport  Kind = "monthly_usage_report"
	KindManual         Kind = "manual"
)

// DeliveryStatus is the durable state of one recipient delivery.
type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "pending"
	DeliveryProcessing DeliveryStatus = "processing"
	DeliverySent       DeliveryStatus = "sent"
	DeliveryFailed     DeliveryStatus = "failed"
)

// Message stores immutable content shared by one or more recipient deliveries.
type Message struct {
	ID              string  `gorm:"type:uuid;primaryKey"`
	Kind            Kind    `gorm:"type:varchar(48);not null;index"`
	Subject         string  `gorm:"type:varchar(255);not null"`
	MarkdownBody    string  `gorm:"type:text;not null"`
	TextBody        string  `gorm:"type:text;not null"`
	HTMLBody        string  `gorm:"type:text;not null"`
	CreatedByUserID *string `gorm:"type:uuid;index"`
	IdempotencyKey  *string `gorm:"type:varchar(64);uniqueIndex"`
	Audience        string  `gorm:"type:varchar(32);not null;default:'automatic'"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Deliveries      []Delivery `gorm:"foreignKey:MessageID"`
}

// BeforeCreate assigns a UUID to a new message.
func (m *Message) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(m.ID) == "" {
		m.ID = uuid.NewString()
	}
	return nil
}

// Delivery stores the SMTP lifecycle for one resolved recipient.
type Delivery struct {
	ID               string         `gorm:"type:uuid;primaryKey"`
	MessageID        string         `gorm:"type:uuid;not null;index;constraint:OnDelete:CASCADE"`
	UserID           *string        `gorm:"type:uuid;index"`
	RecipientEmail   string         `gorm:"type:varchar(320);not null;index"`
	RecipientName    string         `gorm:"type:varchar(255);not null;default:''"`
	Status           DeliveryStatus `gorm:"type:varchar(16);not null;index"`
	Attempts         int            `gorm:"not null;default:0"`
	NextAttemptAt    time.Time      `gorm:"not null;index"`
	LeaseUntil       *time.Time     `gorm:"index"`
	SentAt           *time.Time     `gorm:"index"`
	LastAttemptAt    *time.Time
	LastError        string  `gorm:"type:text;not null;default:''"`
	DeduplicationKey *string `gorm:"type:varchar(255);uniqueIndex"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// BeforeCreate assigns a UUID and default state to a new delivery.
func (d *Delivery) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(d.ID) == "" {
		d.ID = uuid.NewString()
	}
	if d.Status == "" {
		d.Status = DeliveryPending
	}
	if d.NextAttemptAt.IsZero() {
		d.NextAttemptAt = time.Now().UTC()
	}
	return nil
}

// QuotaState remembers whether each quota window has already emitted an alert.
type QuotaState struct {
	UserID      string `gorm:"type:uuid;primaryKey"`
	Exhausted5H bool   `gorm:"not null;default:false"`
	Exhausted7D bool   `gorm:"not null;default:false"`
	Sequence    int64  `gorm:"not null;default:0"`
	UpdatedAt   time.Time
}

// MessageSummary is the paginated administration projection.
type MessageSummary struct {
	ID         string    `json:"id"`
	Kind       Kind      `json:"kind"`
	Subject    string    `json:"subject"`
	Audience   string    `json:"audience"`
	Recipients int64     `json:"recipients"`
	Pending    int64     `json:"pending"`
	Processing int64     `json:"processing"`
	Sent       int64     `json:"sent"`
	Failed     int64     `json:"failed"`
	CreatedAt  time.Time `json:"createdAt"`
}

// DeliveryResponse is the admin-safe delivery representation.
type DeliveryResponse struct {
	ID             string         `json:"id"`
	UserID         *string        `json:"userId"`
	RecipientEmail string         `json:"recipientEmail"`
	RecipientName  string         `json:"recipientName"`
	Status         DeliveryStatus `json:"status"`
	Attempts       int            `json:"attempts"`
	NextAttemptAt  time.Time      `json:"nextAttemptAt"`
	SentAt         *time.Time     `json:"sentAt"`
	LastAttemptAt  *time.Time     `json:"lastAttemptAt"`
	LastError      string         `json:"lastError"`
}

// MessageDetail includes stored content and per-recipient delivery state.
type MessageDetail struct {
	MessageSummary
	MarkdownBody string             `json:"markdownBody"`
	TextBody     string             `json:"textBody"`
	HTMLBody     string             `json:"htmlBody"`
	Deliveries   []DeliveryResponse `json:"deliveries"`
}

// ListResult is a paginated message history.
type ListResult struct {
	Items    []MessageSummary `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int64            `json:"total"`
}
