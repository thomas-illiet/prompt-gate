package groups

import (
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/users"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Group struct {
	ID            uuid.UUID           `gorm:"type:uuid;primaryKey"`
	Name          string              `gorm:"not null;uniqueIndex"`
	DisplayName   string              `gorm:"not null;default:''"`
	Description   string              `gorm:"type:text;not null;default:''"`
	Providers     []provider.Provider `gorm:"many2many:access_group_providers;joinForeignKey:GroupID;joinReferences:ProviderID"`
	ModelPatterns []GroupModelPattern `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Members       []users.User        `gorm:"many2many:access_group_members;joinForeignKey:GroupID;joinReferences:UserID"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

type legacyGroupEnabledColumn struct {
	Enabled bool `gorm:"column:enabled"`
}

func (legacyGroupEnabledColumn) TableName() string {
	return "access_groups"
}

func (Group) TableName() string {
	return "access_groups"
}

func (g *Group) BeforeCreate(_ *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	g.Name = normalizeName(g.Name)
	return nil
}

func (g *Group) BeforeUpdate(_ *gorm.DB) error {
	g.Name = normalizeName(g.Name)
	return nil
}

type GroupProvider struct {
	GroupID    uuid.UUID         `gorm:"type:uuid;primaryKey"`
	ProviderID uuid.UUID         `gorm:"type:uuid;primaryKey"`
	Group      Group             `gorm:"foreignKey:GroupID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Provider   provider.Provider `gorm:"foreignKey:ProviderID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt  time.Time
}

func (GroupProvider) TableName() string {
	return "access_group_providers"
}

type GroupModelPattern struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	GroupID   uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_access_group_model_pattern"`
	Group     Group     `gorm:"foreignKey:GroupID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Pattern   string    `gorm:"not null;uniqueIndex:idx_access_group_model_pattern"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (GroupModelPattern) TableName() string {
	return "access_group_model_patterns"
}

func (p *GroupModelPattern) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.Pattern = strings.TrimSpace(p.Pattern)
	return nil
}

func (p *GroupModelPattern) BeforeUpdate(_ *gorm.DB) error {
	p.Pattern = strings.TrimSpace(p.Pattern)
	return nil
}

type GroupMember struct {
	GroupID   uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID    string     `gorm:"type:uuid;primaryKey"`
	Group     Group      `gorm:"foreignKey:GroupID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	User      users.User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt time.Time
}

func (GroupMember) TableName() string {
	return "access_group_members"
}

type ProviderSummary struct {
	ID          uuid.UUID             `json:"id"`
	Name        string                `json:"name"`
	DisplayName string                `json:"displayName"`
	Type        provider.ProviderType `json:"type"`
	Enabled     bool                  `json:"enabled"`
}

type MemberSummary struct {
	ID                string        `json:"id"`
	PreferredUsername string        `json:"preferredUsername"`
	Email             string        `json:"email"`
	Name              string        `json:"name"`
	Type              auth.UserType `json:"type"`
	Role              auth.AppRole  `json:"role"`
	IsActive          bool          `json:"isActive"`
}

type GroupResponse struct {
	ID                uuid.UUID         `json:"id"`
	Name              string            `json:"name"`
	DisplayName       string            `json:"displayName"`
	Description       string            `json:"description"`
	Providers         []ProviderSummary `json:"providers"`
	ModelPatterns     []string          `json:"modelPatterns"`
	Members           []MemberSummary   `json:"members"`
	ProviderCount     int               `json:"providerCount"`
	ModelPatternCount int               `json:"modelPatternCount"`
	MemberCount       int               `json:"memberCount"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

type ListParams struct {
	Page     int
	PageSize int
	Search   string
	SortBy   string
	SortDir  string
}

type ListResult struct {
	Items    []GroupResponse `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

type CreateGroupInput struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"displayName"`
	Description   string   `json:"description"`
	ProviderIDs   []string `json:"providerIds"`
	ModelPatterns []string `json:"modelPatterns"`
}

type UpdateGroupInput struct {
	Name          *string   `json:"name,omitempty"`
	DisplayName   *string   `json:"displayName,omitempty"`
	Description   *string   `json:"description,omitempty"`
	ProviderIDs   *[]string `json:"providerIds,omitempty"`
	ModelPatterns *[]string `json:"modelPatterns,omitempty"`
}

type ReplaceUserGroupsInput struct {
	GroupIDs []string `json:"groupIds"`
}

type ValidateModelPatternsInput struct {
	ProviderIDs   []string `json:"providerIds"`
	ModelPatterns []string `json:"modelPatterns"`
}

type ModelPatternValidationResponse struct {
	MatchedModelCount        int                                    `json:"matchedModelCount"`
	MatchedModels            []string                               `json:"matchedModels"`
	ProviderResults          []ModelPatternProviderValidationResult `json:"providerResults"`
	UnavailableProviderCount int                                    `json:"unavailableProviderCount"`
}

type ModelPatternProviderValidationResult struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	DisplayName         string    `json:"displayName"`
	AvailableModelCount int       `json:"availableModelCount"`
	MatchedModelCount   int       `json:"matchedModelCount"`
	MatchedModels       []string  `json:"matchedModels"`
	ModelsError         string    `json:"modelsError,omitempty"`
}

func normalizeName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
