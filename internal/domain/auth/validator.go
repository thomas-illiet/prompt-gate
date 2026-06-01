package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

type ValidatorOption func(*validatorOptions)

type validatorOptions struct {
	httpClient *http.Client
}

// WithValidatorHTTPClient configures the HTTP client used to fetch and refresh JWKS keys.
func WithValidatorHTTPClient(client *http.Client) ValidatorOption {
	return func(options *validatorOptions) {
		options.httpClient = client
	}
}

// NewValidator creates a JWT validator that fetches and refreshes keys from jwksURL.
func NewValidator(ctx context.Context, issuer string, jwksURL string, opts ...ValidatorOption) (*Validator, error) {
	options := validatorOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	var (
		jwks keyfunc.Keyfunc
		err  error
	)
	if options.httpClient != nil {
		jwks, err = keyfunc.NewDefaultOverrideCtx(ctx, []string{jwksURL}, keyfunc.Override{
			Client: options.httpClient,
		})
	} else {
		jwks, err = keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	}
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
