package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AppRole string

const (
	RoleNone    AppRole = "none"
	RoleUser    AppRole = "user"
	RoleManager AppRole = "manager"
	RoleAdmin   AppRole = "admin"
)

// IsValid reports whether the role is one of the defined AppRole constants.
func (r AppRole) IsValid() bool {
	switch r {
	case RoleNone, RoleUser, RoleManager, RoleAdmin:
		return true
	default:
		return false
	}
}

type UserType string

const (
	UserTypeUser    UserType = "user"
	UserTypeService UserType = "service"
)

// IsValid reports whether the type is one of the defined UserType constants.
func (t UserType) IsValid() bool {
	switch t {
	case UserTypeUser, UserTypeService:
		return true
	default:
		return false
	}
}

type UserProfile struct {
	ID                      string    `json:"id"`
	Sub                     string    `json:"sub"`
	PreferredUsername       string    `json:"preferredUsername"`
	Email                   string    `json:"email"`
	Name                    string    `json:"name"`
	Type                    UserType  `json:"type"`
	Role                    AppRole   `json:"role"`
	IsActive                bool      `json:"isActive"`
	FirewallOverrideEnabled bool      `json:"firewallOverrideEnabled"`
	LastLoginAt             time.Time `json:"lastLoginAt"`
}

type Identity struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferredUsername"`
	Email             string `json:"email"`
	Name              string `json:"name"`
}

type keycloakClaims struct {
	jwt.RegisteredClaims
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
}

// Identity maps Keycloak JWT claims to an Identity value.
func (claims *keycloakClaims) Identity() Identity {
	return Identity{
		Sub:               claims.Subject,
		PreferredUsername: claims.PreferredUsername,
		Email:             claims.Email,
		Name:              claims.Name,
	}
}
