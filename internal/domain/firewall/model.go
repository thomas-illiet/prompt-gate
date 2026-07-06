package firewall

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

type RuleType string

const (
	RuleTypeGlobal         RuleType = "global"
	RuleTypeServiceAccount RuleType = "service_account"
	RuleTypeUser           RuleType = "user"
)

const (
	MinPriority = 1
	MaxPriority = 9999
)

// FirewallRule is the database model for all firewall rules.
type FirewallRule struct {
	ID            string   `gorm:"type:uuid;primaryKey"`
	Type          RuleType `gorm:"column:type;type:varchar(32);not null;default:'global';index"`
	ReferentielID *string  `gorm:"column:referentiel_id;type:uuid;index;uniqueIndex:idx_firewall_rules_priority_referentiel,priority:2"`
	Address       string   `gorm:"not null"`
	Description   string   `gorm:"type:text"`
	Priority      int      `gorm:"not null;uniqueIndex:idx_firewall_rules_priority_referentiel,priority:1"`
	Action        Action   `gorm:"type:varchar(8);not null;index"`
	Enabled       bool     `gorm:"not null;index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BeforeCreate generates a UUID primary key if not set.
func (r *FirewallRule) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(r.ID) == "" {
		r.ID = uuid.NewString()
	}
	if strings.TrimSpace(string(r.Type)) == "" {
		r.Type = RuleTypeGlobal
	}
	return nil
}

// toResponse converts a firewall model into its admin API shape.
func (r *FirewallRule) toResponse() RuleResponse {
	response := RuleResponse{
		ID:          r.ID,
		Address:     r.Address,
		Description: r.Description,
		Priority:    r.Priority,
		Action:      r.Action,
		Enabled:     r.Enabled,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	if r.Type == RuleTypeServiceAccount && r.ReferentielID != nil {
		response.ServiceAccountID = *r.ReferentielID
	}
	if r.Type == RuleTypeUser && r.ReferentielID != nil {
		response.UserID = *r.ReferentielID
	}
	return response
}

// RuleResponse is the JSON shape returned to admin callers.
type RuleResponse struct {
	ID               string    `json:"id"`
	ServiceAccountID string    `json:"serviceAccountId,omitempty"`
	UserID           string    `json:"userId,omitempty"`
	Address          string    `json:"address"`
	Description      string    `json:"description"`
	Priority         int       `json:"priority"`
	Action           Action    `json:"action"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ListParams struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

type ListResult struct {
	Items    []RuleResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int64          `json:"total"`
}

type CreateRuleInput struct {
	Address     string `json:"address"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Action      Action `json:"action"`
	Enabled     bool   `json:"enabled"`
}

type UpdateRuleInput struct {
	Address     *string `json:"address,omitempty"`
	Description *string `json:"description,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
	Action      *Action `json:"action,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

type MovePriorityInput struct {
	Direction string `json:"direction"`
}
