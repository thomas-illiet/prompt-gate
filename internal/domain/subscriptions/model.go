package subscriptions

import (
	"strings"
	"time"

	"promptgate/backend/internal/domain/users"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	Window5H = "5h"
	Window7D = "7d"
)

type SubscriptionPlan struct {
	ID            string    `gorm:"type:uuid;primaryKey" json:"id"`
	Name          string    `gorm:"not null;uniqueIndex" json:"name"`
	Description   string    `gorm:"type:text;not null;default:''" json:"description"`
	Quota5HTokens *int64    `gorm:"column:quota_5h_tokens" json:"quota5hTokens"`
	Quota7DTokens *int64    `gorm:"column:quota_7d_tokens" json:"quota7dTokens"`
	IsDefault     bool      `gorm:"column:is_default;not null;default:false;index" json:"isDefault"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type SubscriptionQuotaState struct {
	UserID          string      `gorm:"type:uuid;primaryKey" json:"userId"`
	User            *users.User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	HasSubscription bool        `gorm:"not null;default:false" json:"hasSubscription"`
	PlanID          *string     `gorm:"column:plan_id;type:uuid;index" json:"planId"`
	PlanName        string      `gorm:"not null;default:''" json:"planName"`
	Used5HTokens    int64       `gorm:"column:used_5h_tokens;not null;default:0" json:"used5hTokens"`
	Quota5HTokens   *int64      `gorm:"column:quota_5h_tokens" json:"quota5hTokens"`
	Reset5HAt       *time.Time  `gorm:"column:reset_5h_at" json:"reset5hAt"`
	Used7DTokens    int64       `gorm:"column:used_7d_tokens;not null;default:0" json:"used7dTokens"`
	Quota7DTokens   *int64      `gorm:"column:quota_7d_tokens" json:"quota7dTokens"`
	Reset7DAt       *time.Time  `gorm:"column:reset_7d_at" json:"reset7dAt"`
	SyncedAt        time.Time   `gorm:"not null;index" json:"syncedAt"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
}

type PlanInput struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Quota5HTokens *int64 `json:"quota5hTokens"`
	Quota7DTokens *int64 `json:"quota7dTokens"`
	IsDefault     bool   `json:"isDefault"`
}

type AssignPlanInput struct {
	PlanID *string `json:"planId"`
}

type PlanResponse struct {
	ID                           string    `json:"id"`
	Name                         string    `json:"name"`
	Description                  string    `json:"description"`
	Quota5HTokens                *int64    `json:"quota5hTokens"`
	Quota7DTokens                *int64    `json:"quota7dTokens"`
	IsDefault                    bool      `json:"isDefault"`
	AssignedUsersCount           int64     `json:"assignedUsersCount"`
	AssignedServiceAccountsCount int64     `json:"assignedServiceAccountsCount"`
	AssignedAccountsCount        int64     `json:"assignedAccountsCount"`
	CreatedAt                    time.Time `json:"createdAt"`
	UpdatedAt                    time.Time `json:"updatedAt"`
}

type PlanListParams struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

type PlanListResult struct {
	Items    []PlanResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int64          `json:"total"`
}

type QuotaStatus struct {
	HasSubscription   bool          `json:"hasSubscription"`
	Plan              *PlanResponse `json:"plan,omitempty"`
	Used5HTokens      int64         `json:"used5hTokens"`
	Quota5HTokens     *int64        `json:"quota5hTokens"`
	Remaining5HTokens *int64        `json:"remaining5hTokens"`
	Reset5HAt         *time.Time    `json:"reset5hAt"`
	Used7DTokens      int64         `json:"used7dTokens"`
	Quota7DTokens     *int64        `json:"quota7dTokens"`
	Remaining7DTokens *int64        `json:"remaining7dTokens"`
	Reset7DAt         *time.Time    `json:"reset7dAt"`
	SyncedAt          *time.Time    `json:"syncedAt,omitempty"`
}

type PlanSnapshot struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Quota5HTokens *int64 `json:"quota5hTokens"`
	Quota7DTokens *int64 `json:"quota7dTokens"`
	IsDefault     bool   `json:"isDefault"`
}

type Snapshot struct {
	DefaultPlanID *string                 `json:"defaultPlanId"`
	Plans         map[string]PlanSnapshot `json:"plans"`
	CreatedAt     time.Time               `json:"createdAt"`
}

type AssignmentSnapshot struct {
	HasUser bool    `json:"hasUser"`
	PlanID  *string `json:"planId"`
}

func (p *SubscriptionPlan) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(p.ID) == "" {
		p.ID = uuid.NewString()
	}
	return nil
}

func planResponse(plan SubscriptionPlan) PlanResponse {
	return PlanResponse{
		ID:            plan.ID,
		Name:          plan.Name,
		Description:   plan.Description,
		Quota5HTokens: cloneInt64Ptr(plan.Quota5HTokens),
		Quota7DTokens: cloneInt64Ptr(plan.Quota7DTokens),
		IsDefault:     plan.IsDefault,
		CreatedAt:     plan.CreatedAt,
		UpdatedAt:     plan.UpdatedAt,
	}
}

func snapshotFromPlan(plan SubscriptionPlan) PlanSnapshot {
	return PlanSnapshot{
		ID:            plan.ID,
		Name:          plan.Name,
		Description:   plan.Description,
		Quota5HTokens: cloneInt64Ptr(plan.Quota5HTokens),
		Quota7DTokens: cloneInt64Ptr(plan.Quota7DTokens),
		IsDefault:     plan.IsDefault,
	}
}

func responseFromSnapshot(plan PlanSnapshot) PlanResponse {
	return PlanResponse{
		ID:            plan.ID,
		Name:          plan.Name,
		Description:   plan.Description,
		Quota5HTokens: cloneInt64Ptr(plan.Quota5HTokens),
		Quota7DTokens: cloneInt64Ptr(plan.Quota7DTokens),
		IsDefault:     plan.IsDefault,
	}
}

func accountPlanFromSnapshot(plan PlanSnapshot) users.AccountSubscriptionPlan {
	return users.AccountSubscriptionPlan{
		ID:            plan.ID,
		Name:          plan.Name,
		Description:   plan.Description,
		Quota5HTokens: cloneInt64Ptr(plan.Quota5HTokens),
		Quota7DTokens: cloneInt64Ptr(plan.Quota7DTokens),
		IsDefault:     plan.IsDefault,
	}
}

func accountPlanFromResponse(plan PlanResponse) users.AccountSubscriptionPlan {
	return users.AccountSubscriptionPlan{
		ID:            plan.ID,
		Name:          plan.Name,
		Description:   plan.Description,
		Quota5HTokens: cloneInt64Ptr(plan.Quota5HTokens),
		Quota7DTokens: cloneInt64Ptr(plan.Quota7DTokens),
		IsDefault:     plan.IsDefault,
	}
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	next := *value
	return &next
}
