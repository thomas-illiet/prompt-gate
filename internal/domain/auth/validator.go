package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

var validSigningMethods = []string{
	jwt.SigningMethodRS256.Alg(),
	jwt.SigningMethodRS384.Alg(),
	jwt.SigningMethodRS512.Alg(),
	jwt.SigningMethodPS256.Alg(),
	jwt.SigningMethodPS384.Alg(),
	jwt.SigningMethodPS512.Alg(),
	jwt.SigningMethodES256.Alg(),
	jwt.SigningMethodES384.Alg(),
	jwt.SigningMethodES512.Alg(),
	jwt.SigningMethodEdDSA.Alg(),
}

type Validator struct {
	issuer string
	jwks   keyfunc.Keyfunc
}

// NewValidator creates a JWT validator that fetches and refreshes keys from jwksURL.
func NewValidator(ctx context.Context, issuer string, jwksURL string) (*Validator, error) {
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("create JWKS client: %w", err)
	}

	return &Validator{
		issuer: issuer,
		jwks:   jwks,
	}, nil
}

// ValidateAccessToken parses and validates a raw JWT access token, returning its identity claims.
func (v *Validator) ValidateAccessToken(rawToken string) (Identity, error) {
	claims := &keycloakClaims{}
	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		v.jwks.Keyfunc,
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithValidMethods(validSigningMethods),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		return Identity{}, errors.New("token is invalid")
	}

	return claims.Identity(), nil
}

// Close is a no-op; the JWKS refresh lifecycle is managed by the context passed to NewValidator.
func (v *Validator) Close() {
	// The JWKS refresh lifecycle is tied to the context passed to NewValidator.
}
