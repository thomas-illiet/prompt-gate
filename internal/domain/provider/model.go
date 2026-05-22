package provider

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProviderType string

const (
	ProviderTypeOpenAI    ProviderType = "openai"
	ProviderTypeOllama    ProviderType = "ollama"
	ProviderTypeAnthropic ProviderType = "anthropic"
)

type ProviderConfig map[string]interface{}

// Value serializes provider config as JSON for database storage.
func (c ProviderConfig) Value() (driver.Value, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan deserializes provider config from database storage.
func (c *ProviderConfig) Scan(v interface{}) error {
	switch val := v.(type) {
	case []byte:
		return json.Unmarshal(val, c)
	case string:
		return json.Unmarshal([]byte(val), c)
	case nil:
		*c = ProviderConfig{}
		return nil
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

type Provider struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name             string         `gorm:"not null;uniqueIndex" json:"name"`
	DisplayName      string         `gorm:"not null;default:''" json:"displayName"`
	Type             ProviderType   `gorm:"not null" json:"type"`
	BaseURL          string         `gorm:"not null;default:''" json:"baseUrl"`
	APIKeyCiphertext string         `gorm:"column:api_key_ciphertext;not null;default:''" json:"-"`
	Config           ProviderConfig `gorm:"type:jsonb;default:'{}'" json:"config"`
	Enabled          bool           `gorm:"not null;default:true" json:"enabled"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate assigns ids and normalized names before provider insertion.
func (p *Provider) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	p.Name = normalizeName(p.Name)
	return nil
}

// BeforeUpdate normalizes provider names before updates.
func (p *Provider) BeforeUpdate(_ *gorm.DB) error {
	p.Name = normalizeName(p.Name)
	return nil
}

// normalizeName returns the canonical provider name form.
func normalizeName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
