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
	StartedAt    time.Time   `gorm:"not null;index" json:"startedAt"`
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

type ProxyDailyUsageKPI struct {
	Day                    time.Time   `gorm:"type:date;not null;uniqueIndex:idx_proxy_daily_usage_kpis_day_initiator;index" json:"day"`
	InitiatorID            string      `gorm:"type:uuid;not null;uniqueIndex:idx_proxy_daily_usage_kpis_day_initiator;index" json:"initiatorId"`
	Initiator              *users.User `gorm:"foreignKey:InitiatorID;references:ID;constraint:OnDelete:RESTRICT" json:"-"`
	Requests               int64       `gorm:"not null;default:0" json:"requests"`
	Prompts                int64       `gorm:"not null;default:0" json:"prompts"`
	ToolCalls              int64       `gorm:"not null;default:0" json:"toolCalls"`
	TotalDurationMs        int64       `gorm:"not null;default:0" json:"totalDurationMs"`
	InputTokens            int64       `gorm:"not null;default:0" json:"inputTokens"`
	OutputTokens           int64       `gorm:"not null;default:0" json:"outputTokens"`
	CacheReadInputTokens   int64       `gorm:"not null;default:0" json:"cacheReadInputTokens"`
	CacheWriteInputTokens  int64       `gorm:"not null;default:0" json:"cacheWriteInputTokens"`
	CompletionInputTokens  int64       `gorm:"not null;default:0" json:"completionInputTokens"`
	CompletionOutputTokens int64       `gorm:"not null;default:0" json:"completionOutputTokens"`
	CompletionTokens       int64       `gorm:"not null;default:0" json:"completionTokens"`
	EmbeddingTokens        int64       `gorm:"not null;default:0" json:"embeddingTokens"`
	TotalTokens            int64       `gorm:"not null;default:0" json:"totalTokens"`
	CreatedAt              time.Time   `gorm:"not null" json:"createdAt"`
	UpdatedAt              time.Time   `gorm:"not null" json:"updatedAt"`
}

type ProxyDailyUsageBreakdown struct {
	ID                     string      `gorm:"type:uuid;primaryKey" json:"id"`
	Day                    time.Time   `gorm:"type:date;not null;uniqueIndex:idx_proxy_daily_usage_breakdowns_unique;index" json:"day"`
	InitiatorID            string      `gorm:"type:uuid;not null;uniqueIndex:idx_proxy_daily_usage_breakdowns_unique;index" json:"initiatorId"`
	Initiator              *users.User `gorm:"foreignKey:InitiatorID;references:ID;constraint:OnDelete:RESTRICT" json:"-"`
	Dimension              string      `gorm:"not null;uniqueIndex:idx_proxy_daily_usage_breakdowns_unique;index" json:"dimension"`
	Name                   string      `gorm:"not null;uniqueIndex:idx_proxy_daily_usage_breakdowns_unique" json:"name"`
	Requests               int64       `gorm:"not null;default:0" json:"requests"`
	InputTokens            int64       `gorm:"not null;default:0" json:"inputTokens"`
	OutputTokens           int64       `gorm:"not null;default:0" json:"outputTokens"`
	CacheReadInputTokens   int64       `gorm:"not null;default:0" json:"cacheReadInputTokens"`
	CacheWriteInputTokens  int64       `gorm:"not null;default:0" json:"cacheWriteInputTokens"`
	CompletionInputTokens  int64       `gorm:"not null;default:0" json:"completionInputTokens"`
	CompletionOutputTokens int64       `gorm:"not null;default:0" json:"completionOutputTokens"`
	CompletionTokens       int64       `gorm:"not null;default:0" json:"completionTokens"`
	EmbeddingTokens        int64       `gorm:"not null;default:0" json:"embeddingTokens"`
	TotalTokens            int64       `gorm:"not null;default:0" json:"totalTokens"`
	CreatedAt              time.Time   `gorm:"not null" json:"createdAt"`
	UpdatedAt              time.Time   `gorm:"not null" json:"updatedAt"`
}

type ProcessedUsageEvent struct {
	EventID        string    `gorm:"primaryKey" json:"eventId"`
	RedisMessageID string    `gorm:"not null;default:''" json:"redisMessageId"`
	Type           string    `gorm:"not null;index" json:"type"`
	CreatedAt      time.Time `gorm:"not null" json:"createdAt"`
	ProcessedAt    time.Time `gorm:"not null;index" json:"processedAt"`
}

// TableName returns the stable dashboard KPI table name.
func (ProxyDailyUsageKPI) TableName() string {
	return "proxy_daily_usage_kpis"
}

// TableName returns the stable dashboard breakdown table name.
func (ProxyDailyUsageBreakdown) TableName() string {
	return "proxy_daily_usage_breakdowns"
}

// TableName returns the stable processed event table name.
func (ProcessedUsageEvent) TableName() string {
	return "processed_usage_events"
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

// BeforeCreate assigns a UUID before inserting a breakdown row.
func (b *ProxyDailyUsageBreakdown) BeforeCreate(_ *gorm.DB) error {
	setID(&b.ID)
	return nil
}

// setID initializes an id when it has not already been set.
func setID(id *string) {
	if strings.TrimSpace(*id) == "" {
		*id = uuid.NewString()
	}
}
