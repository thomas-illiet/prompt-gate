package subscriptions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"promptgate/backend/internal/domain/users"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// MaxQuotaResetCredits is the maximum number of reset credits a user can hold.
	MaxQuotaResetCredits = 2

	maxQuotaResetIdempotencyKeyLength = 512
)

// QuotaResetActorType identifies the trusted origin of a reset ledger event.
type QuotaResetActorType string

const (
	QuotaResetActorSystem       QuotaResetActorType = "system"
	QuotaResetActorUser         QuotaResetActorType = "user"
	QuotaResetActorAdminSession QuotaResetActorType = "admin_session"
	QuotaResetActorAdminAPIKey  QuotaResetActorType = "admin_api_key"
)

// QuotaResetActor carries internal audit identity for an administrator grant.
type QuotaResetActor struct {
	Type   QuotaResetActorType
	UserID *string
}

var (
	ErrQuotaResetUserNotFound          = errors.New("quota reset user not found")
	ErrQuotaResetBalanceFull           = errors.New("quota reset balance full")
	ErrQuotaResetUnavailable           = errors.New("quota reset credit unavailable")
	ErrQuotaResetNotApplicable         = errors.New("quota reset not applicable")
	ErrQuotaResetPending               = errors.New("quota reset pending")
	ErrInvalidQuotaResetIdempotencyKey = errors.New("invalid quota reset idempotency key")
)

// QuotaResetEventType identifies why a reset balance changed.
type QuotaResetEventType string

const (
	QuotaResetEventMonthlyGrant QuotaResetEventType = "monthly_grant"
	QuotaResetEventAdminGrant   QuotaResetEventType = "admin_grant"
	QuotaResetEventUsed         QuotaResetEventType = "quota_reset_used"
)

// QuotaResetEventStatus tracks whether a reserved reset reached Redis.
type QuotaResetEventStatus string

const (
	QuotaResetEventPending QuotaResetEventStatus = "pending"
	QuotaResetEventApplied QuotaResetEventStatus = "applied"
)

// QuotaResetWallet stores the durable reset balance and sequencing metadata for one user.
type QuotaResetWallet struct {
	UserID                string      `gorm:"type:uuid;primaryKey"`
	User                  *users.User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Available             int         `gorm:"not null;default:0;check:quota_reset_wallet_available,available >= 0 AND available <= 2"`
	LastMonthlyGrantMonth *time.Time  `gorm:"column:last_monthly_grant_month;type:date"`
	ResetSequence         int64       `gorm:"column:reset_sequence;not null;default:0"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// QuotaResetEvent is the append-only balance ledger. Reset consumption is first
// persisted as pending and becomes applied after its Redis barrier succeeds.
type QuotaResetEvent struct {
	ID                     string                `gorm:"type:uuid;primaryKey"`
	UserID                 string                `gorm:"type:uuid;not null;index;uniqueIndex:idx_quota_reset_event_idempotency,priority:1"`
	User                   *users.User           `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Type                   QuotaResetEventType   `gorm:"type:varchar(32);not null;index;index:idx_quota_reset_reconcile,priority:1;uniqueIndex:idx_quota_reset_event_idempotency,priority:2"`
	Status                 QuotaResetEventStatus `gorm:"type:varchar(16);not null;index;index:idx_quota_reset_reconcile,priority:2"`
	Delta                  int                   `gorm:"not null"`
	BalanceAfter           int                   `gorm:"column:balance_after;not null;check:quota_reset_event_balance, balance_after >= 0 AND balance_after <= 2"`
	ActorType              QuotaResetActorType   `gorm:"column:actor_type;type:varchar(32);not null"`
	ActorUserID            *string               `gorm:"column:actor_user_id;type:uuid"`
	ActorUser              *users.User           `gorm:"foreignKey:ActorUserID;references:ID;constraint:OnDelete:SET NULL"`
	IdempotencyKey         *string               `gorm:"column:idempotency_key;type:char(64);uniqueIndex:idx_quota_reset_event_idempotency,priority:3"`
	GrantMonth             *time.Time            `gorm:"column:grant_month;type:date"`
	ResetSequence          *int64                `gorm:"column:reset_sequence"`
	ResetCutoffAt          *time.Time            `gorm:"column:reset_cutoff_at"`
	ReconcileAttempts      int64                 `gorm:"column:reconcile_attempts;not null;default:0;index:idx_quota_reset_reconcile,priority:3"`
	LastReconcileAttemptAt *time.Time            `gorm:"column:last_reconcile_attempt_at"`
	AppliedAt              *time.Time            `gorm:"column:applied_at"`
	CreatedAt              time.Time             `gorm:"index;index:idx_quota_reset_reconcile,priority:4"`
}

// BeforeCreate assigns a stable UUID to quota reset ledger entries.
func (e *QuotaResetEvent) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(e.ID) == "" {
		e.ID = uuid.NewString()
	}
	return nil
}

// QuotaResetSummary is the public reset-credit state for one human user.
type QuotaResetSummary struct {
	Available          int  `json:"available"`
	Maximum            int  `json:"maximum"`
	HasResettableQuota bool `json:"hasResettableQuota"`
	PendingReset       bool `json:"pendingReset"`
}

// QuotaResetHistoryItem is the public, privacy-preserving reset ledger projection.
type QuotaResetHistoryItem struct {
	ID           string              `json:"id"`
	Type         QuotaResetEventType `json:"type"`
	Delta        int                 `json:"delta"`
	BalanceAfter int                 `json:"balanceAfter"`
	CreatedAt    time.Time           `json:"createdAt"`
}

// QuotaResetHistoryParams controls personal reset-history pagination and sorting.
type QuotaResetHistoryParams struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

// QuotaResetHistoryResult is a paginated reset ledger response.
type QuotaResetHistoryResult struct {
	Items    []QuotaResetHistoryItem `json:"items"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
	Total    int64                   `json:"total"`
}

// QuotaResetUseStatus is the externally visible reset application state.
type QuotaResetUseStatus string

const (
	QuotaResetUsePending QuotaResetUseStatus = "pending"
	QuotaResetUseApplied QuotaResetUseStatus = "applied"
)

// QuotaResetUseResult reports the durable balance and, once applied, the current live quota.
// Pending resets omit Quota because Redis has not yet produced an authoritative live state.
type QuotaResetUseResult struct {
	Status  QuotaResetUseStatus `json:"status"`
	Summary QuotaResetSummary   `json:"summary"`
	Quota   *QuotaStatus        `json:"quota,omitempty"`
}

// QuotaResetDistributionResult summarizes one idempotent monthly distribution pass.
type QuotaResetDistributionResult struct {
	Eligible         int64
	Granted          int64
	AlreadyProcessed int64
	SkippedAtMaximum int64
}

func normalizeQuotaResetIdempotencyKey(value string) (*string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maxQuotaResetIdempotencyKeyLength {
		return nil, ErrInvalidQuotaResetIdempotencyKey
	}
	digest := sha256.Sum256([]byte(value))
	hashed := hex.EncodeToString(digest[:])
	return &hashed, nil
}

func quotaResetHistoryItem(event QuotaResetEvent) QuotaResetHistoryItem {
	return QuotaResetHistoryItem{
		ID:           event.ID,
		Type:         event.Type,
		Delta:        event.Delta,
		BalanceAfter: event.BalanceAfter,
		CreatedAt:    event.CreatedAt.UTC(),
	}
}
