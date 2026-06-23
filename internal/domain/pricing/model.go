package pricing

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Scope string

const (
	ScopeGlobal Scope = "global"
	ScopeModel  Scope = "model"
)

const globalPriceID = "00000000-0000-0000-0000-000000000001"

// UsagePrice stores both global fallback and provider/model-specific token prices.
type UsagePrice struct {
	ID                   string    `gorm:"type:uuid;primaryKey" json:"id"`
	Scope                Scope     `gorm:"not null;uniqueIndex:idx_usage_prices_scope_provider_model" json:"scope"`
	ProviderName         string    `gorm:"not null;default:'';uniqueIndex:idx_usage_prices_scope_provider_model" json:"providerName"`
	Model                string    `gorm:"not null;default:'';uniqueIndex:idx_usage_prices_scope_provider_model" json:"model"`
	InputUSDPer1MTokens  float64   `gorm:"column:input_usd_per_1m_tokens;not null;default:0" json:"inputUsdPer1MTokens"`
	OutputUSDPer1MTokens float64   `gorm:"column:output_usd_per_1m_tokens;not null;default:0" json:"outputUsdPer1MTokens"`
	CreatedAt            time.Time `gorm:"not null" json:"createdAt"`
	UpdatedAt            time.Time `gorm:"not null" json:"updatedAt"`
}

func (UsagePrice) TableName() string {
	return "usage_prices"
}

// BeforeCreate assigns a stable id for the global row and random ids for model rows.
func (p *UsagePrice) BeforeCreate(_ *gorm.DB) error {
	if p.ID != "" {
		return nil
	}
	if p.Scope == ScopeGlobal {
		p.ID = globalPriceID
		return nil
	}
	p.ID = uuid.NewString()
	return nil
}

type PriceRates struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

type ConfigResponse struct {
	Fallback PriceRates         `json:"fallback"`
	Models   []ModelPriceRecord `json:"models"`
}

type ModelPriceRecord struct {
	ID           string    `json:"id,omitempty"`
	ProviderName string    `json:"providerName"`
	Model        string    `json:"model"`
	Input        float64   `json:"input"`
	Output       float64   `json:"output"`
	CreatedAt    time.Time `json:"createdAt,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
}

type UpdateConfigInput struct {
	Fallback PriceRates         `json:"fallback"`
	Models   []ModelPriceRecord `json:"models"`
}

type ConfigurationCheckResponse struct {
	Configured     bool                 `json:"configured"`
	MissingPrices  []MissingModelPrice  `json:"missingPrices"`
	ProviderErrors []ProviderModelError `json:"providerErrors"`
	CheckedAt      time.Time            `json:"checkedAt"`
}

type MissingModelPrice struct {
	ProviderName string `json:"providerName"`
	Model        string `json:"model"`
}

type ProviderModelError struct {
	ProviderName string `json:"providerName"`
	Message      string `json:"message"`
}
