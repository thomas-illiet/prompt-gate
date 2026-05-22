package tokens

import (
	"strings"
	"time"

	"promptgate/backend/internal/domain/users"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Token is the database model for stored API tokens.
type Token struct {
	ID          string     `gorm:"type:uuid;primaryKey"`
	UserID      string     `gorm:"type:uuid;not null;index"`
	User        users.User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Name        string     `gorm:"not null"`
	Description string     `gorm:"not null;default:''"`
	TokenHash   string     `gorm:"column:token_hash;uniqueIndex;not null"`
	ExpiresAt   time.Time  `gorm:"not null;index"`
	CreatedAt   time.Time
	RevokedAt   *time.Time `gorm:"index"`
	ExpiredAt   *time.Time `gorm:"index"`
}

// BeforeCreate generates a UUID primary key if not set.
func (t *Token) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(t.ID) == "" {
		t.ID = uuid.NewString()
	}
	return nil
}

// toResponse converts a token model into its API response shape.
func (t *Token) toResponse() TokenResponse {
	return TokenResponse{
		ID:          t.ID,
		UserID:      t.UserID,
		Name:        t.Name,
		Description: t.Description,
		ExpiresAt:   t.ExpiresAt,
		CreatedAt:   t.CreatedAt,
		RevokedAt:   t.RevokedAt,
		ExpiredAt:   t.ExpiredAt,
	}
}

// TokenResponse is the JSON shape returned to callers.
type TokenResponse struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
	ExpiredAt   *time.Time `json:"expiredAt,omitempty"`
}

// CreateTokenRequest is the request body for token creation.
type CreateTokenRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	ExpiresInDays *int   `json:"expiresInDays,omitempty"`
}

// CreateTokenResponse is returned after creation — only time the raw JWT is exposed.
type CreateTokenResponse struct {
	Token     string        `json:"token"`
	TokenInfo TokenResponse `json:"tokenInfo"`
}
