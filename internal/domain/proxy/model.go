package proxy

import (
	"strings"
	"time"

	"promptgate/backend/internal/domain/users"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	tokenUsageEndpointEmbeddings = "/embeddings"
	tokenUsageTypeCompletion     = "completion"
	tokenUsageTypeEmbedding      = "embedding"
)

type Interception struct {
	ID           string      `gorm:"type:uuid;primaryKey" json:"id"`
	InitiatorID  string      `gorm:"type:uuid;not null;index" json:"initiatorId"`
	Initiator    *users.User `gorm:"foreignKey:InitiatorID;references:ID;constraint:OnDelete:RESTRICT" json:"-"`
	Provider     string      `gorm:"not null" json:"provider"`
	ProviderType string      `gorm:"not null;default:''" json:"providerType"`
	Model        string      `gorm:"not null" json:"model"`
	ClientIP     string      `gorm:"not null;default:''" json:"clientIp"`
	StartedAt    time.Time   `gorm:"not null" json:"startedAt"`
	EndedAt      *time.Time  `json:"endedAt"`
	Metadata     string      `json:"metadata"`
}

type TokenUsage struct {
	ID                    string        `gorm:"type:uuid;primaryKey" json:"id"`
	InterceptionID        string        `gorm:"type:uuid;not null;index" json:"interceptionId"`
	Interception          *Interception `gorm:"foreignKey:InterceptionID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	ProviderResponseID    string        `gorm:"not null" json:"providerResponseId"`
	InputTokens           int64         `gorm:"not null;default:0" json:"inputTokens"`
	OutputTokens          int64         `gorm:"not null;default:0" json:"outputTokens"`
	CacheReadInputTokens  int64         `gorm:"not null;default:0" json:"cacheReadInputTokens"`
	CacheWriteInputTokens int64         `gorm:"not null;default:0" json:"cacheWriteInputTokens"`
	Type                  string        `gorm:"not null;default:'completion';index" json:"type"`
	Metadata              string        `json:"metadata"`
	CreatedAt             time.Time     `gorm:"not null" json:"createdAt"`
}

type UserPrompt struct {
	ID                 string        `gorm:"type:uuid;primaryKey" json:"id"`
	InterceptionID     string        `gorm:"type:uuid;not null;index" json:"interceptionId"`
	Interception       *Interception `gorm:"foreignKey:InterceptionID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	ProviderResponseID string        `gorm:"not null" json:"providerResponseId"`
	Prompt             string        `gorm:"not null" json:"prompt"`
	Metadata           string        `json:"metadata"`
	CreatedAt          time.Time     `gorm:"not null" json:"createdAt"`
}

type ToolUsage struct {
	ID                 string        `gorm:"type:uuid;primaryKey" json:"id"`
	InterceptionID     string        `gorm:"type:uuid;not null;index" json:"interceptionId"`
	Interception       *Interception `gorm:"foreignKey:InterceptionID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	ProviderResponseID string        `gorm:"not null" json:"providerResponseId"`
	ServerURL          *string       `json:"serverUrl"`
	Tool               string        `gorm:"not null" json:"tool"`
	Input              string        `gorm:"not null" json:"input"`
	Injected           bool          `gorm:"not null;default:false" json:"injected"`
	InvocationError    *string       `json:"invocationError"`
	Metadata           string        `json:"metadata"`
	CreatedAt          time.Time     `gorm:"not null" json:"createdAt"`
}

// BeforeCreate assigns a UUID before inserting an interception.
func (i *Interception) BeforeCreate(_ *gorm.DB) error {
	setID(&i.ID)
	return nil
}

// BeforeCreate assigns a UUID before inserting token usage.
func (u *TokenUsage) BeforeCreate(_ *gorm.DB) error {
	setID(&u.ID)
	return nil
}

// BeforeCreate assigns a UUID before inserting a user prompt.
func (p *UserPrompt) BeforeCreate(_ *gorm.DB) error {
	setID(&p.ID)
	return nil
}

// BeforeCreate assigns a UUID before inserting tool usage.
func (u *ToolUsage) BeforeCreate(_ *gorm.DB) error {
	setID(&u.ID)
	return nil
}

// setID initializes an id when it has not already been set.
func setID(id *string) {
	if strings.TrimSpace(*id) == "" {
		*id = uuid.NewString()
	}
}
