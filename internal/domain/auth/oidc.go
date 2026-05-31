package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"promptgate/backend/internal/platform/config"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	defaultLoginRedirectPath  = "/dashboard"
	defaultLogoutRedirectPath = "/login"
)

type oidcProviderMetadata struct {
	EndSessionEndpoint string `json:"end_session_endpoint"`
}

type OIDCService struct {
	oauthConfig        oauth2.Config
	idTokenVerifier    *oidc.IDTokenVerifier
	endSessionEndpoint string
	frontendBaseURL    string
	sessionStore       *SessionStore
	userSynchronizer   UserSynchronizer
	validator          *Validator
}

// NewOIDCService discovers the OIDC provider and initializes the OAuth2/OIDC service.
func NewOIDCService(ctx context.Context, cfg config.Config, validator *Validator, sessionStore *SessionStore, userSynchronizer UserSynchronizer) (*OIDCService, error) {
	provider, err := oidc.NewProvider(ctx, cfg.KeycloakIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}

	var metadata oidcProviderMetadata
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("read OIDC metadata: %w", err)
	}

	oauthConfig := oauth2.Config{
		ClientID:     cfg.KeycloakClientID,
		ClientSecret: cfg.KeycloakClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.OIDCCallbackURL(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return &OIDCService{
		oauthConfig:        oauthConfig,
		idTokenVerifier:    provider.Verifier(&oidc.Config{ClientID: cfg.KeycloakClientID}),
		endSessionEndpoint: metadata.EndSessionEndpoint,
		frontendBaseURL:    cfg.FrontendBaseURL,
		sessionStore:       sessionStore,
		userSynchronizer:   userSynchronizer,
		validator:          validator,
	}, nil
}

// AuthorizationURL creates an authorization request and returns the OIDC provider login URL.
func (s *OIDCService) AuthorizationURL(redirectPath string, frontendOrigin string) (string, error) {
	request, err := s.sessionStore.CreateAuthorizationRequest(
		normalizeFrontendPath(redirectPath, defaultLoginRedirectPath),
		resolveFrontendBaseURL(s.frontendBaseURL, frontendOrigin),
	)
	if err != nil {
		return "", fmt.Errorf("create authorization request: %w", err)
	}

	return s.oauthConfig.AuthCodeURL(
		request.State,
		oidc.Nonce(request.Nonce),
		oauth2.SetAuthURLParam("code_challenge", pkceChallengeS256(request.CodeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

// ConsumeAuthorizationRequest delegates to the session store to retrieve and remove an authorization request by state.
func (s *OIDCService) ConsumeAuthorizationRequest(state string) (AuthorizationRequest, bool) {
	return s.sessionStore.ConsumeAuthorizationRequest(state)
}

// ExchangeCode exchanges an authorization code for tokens, validates them, syncs the user, and creates a session.
func (s *OIDCService) ExchangeCode(ctx context.Context, state string, code string) (Session, string, string, error) {
	request, ok := s.sessionStore.ConsumeAuthorizationRequest(state)
	if !ok {
		return Session{}, "", "", errors.New("authorization request has expired or is invalid")
	}

	token, err := s.oauthConfig.Exchange(
		ctx,
		code,
		oauth2.SetAuthURLParam("code_verifier", request.CodeVerifier),
	)
	if err != nil {
		return Session{}, "", request.FrontendBaseURL, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return Session{}, "", request.FrontendBaseURL, errors.New("missing id_token in token response")
	}

	idToken, err := s.idTokenVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Session{}, "", request.FrontendBaseURL, fmt.Errorf("verify id_token: %w", err)
	}

	var claims struct {
		Nonce string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Session{}, "", request.FrontendBaseURL, fmt.Errorf("read id_token claims: %w", err)
	}

	if claims.Nonce != request.Nonce {
		return Session{}, "", request.FrontendBaseURL, errors.New("invalid nonce in id_token")
	}

	identity, err := s.validator.ValidateAccessToken(token.AccessToken)
	if err != nil {
		return Session{}, "", request.FrontendBaseURL, fmt.Errorf("validate access token: %w", err)
	}

	user, err := s.userSynchronizer.SyncUser(ctx, identity)
	if err != nil {
		return Session{}, "", request.FrontendBaseURL, fmt.Errorf("synchronize user: %w", err)
	}

	session, err := s.sessionStore.CreateSession(user, rawIDToken)
	if err != nil {
		return Session{}, "", request.FrontendBaseURL, fmt.Errorf("create session: %w", err)
	}

	return session, request.RedirectPath, request.FrontendBaseURL, nil
}

// DeleteSession removes a session by ID from the session store.
func (s *OIDCService) DeleteSession(sessionID string) {
	s.sessionStore.DeleteSession(sessionID)
}

// FrontendRedirectURL builds an absolute frontend URL for the given path, optionally overriding the origin.
func (s *OIDCService) FrontendRedirectURL(path string, frontendOrigin string) (string, error) {
	return resolveFrontendURL(
		resolveFrontendBaseURL(s.frontendBaseURL, frontendOrigin),
		normalizeFrontendPath(path, defaultLoginRedirectPath),
	)
}

// LoginErrorRedirectURL builds a frontend /login URL with error query parameters.
func (s *OIDCService) LoginErrorRedirectURL(redirectPath string, authError string, description string, frontendOrigin string) (string, error) {
	base, err := url.Parse(resolveFrontendBaseURL(s.frontendBaseURL, frontendOrigin))
	if err != nil {
		return "", fmt.Errorf("parse frontend base URL: %w", err)
	}

	base.Path = "/login"
	base.RawQuery = ""
	base.Fragment = ""

	query := base.Query()
	query.Set("redirect", normalizeFrontendPath(redirectPath, defaultLoginRedirectPath))
	if authError != "" {
		query.Set("authError", authError)
	}
	if description != "" {
		query.Set("authErrorDescription", description)
	}
	base.RawQuery = query.Encode()

	return base.String(), nil
}

// LogoutRedirectURL builds the OIDC end-session URL if available, otherwise falls back to a frontend redirect.
func (s *OIDCService) LogoutRedirectURL(session Session, redirectPath string, frontendOrigin string) (string, error) {
	if s.endSessionEndpoint == "" {
		return s.FrontendRedirectURL(redirectPath, frontendOrigin)
	}

	endSessionURL, err := url.Parse(s.endSessionEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse end session endpoint: %w", err)
	}

	postLogoutRedirectURL, err := s.FrontendRedirectURL(
		normalizeFrontendPath(redirectPath, defaultLogoutRedirectPath),
		frontendOrigin,
	)
	if err != nil {
		return "", err
	}

	query := endSessionURL.Query()
	if session.IDToken != "" {
		query.Set("id_token_hint", session.IDToken)
	}
	query.Set("post_logout_redirect_uri", postLogoutRedirectURL)
	endSessionURL.RawQuery = query.Encode()

	return endSessionURL.String(), nil
}

// resolveFrontendBaseURL returns the override origin if it is an allowed loopback alias of defaultBaseURL, else returns defaultBaseURL.
func resolveFrontendBaseURL(defaultBaseURL string, overrideOrigin string) string {
	if strings.TrimSpace(overrideOrigin) == "" {
		return defaultBaseURL
	}

	base, err := url.Parse(defaultBaseURL)
	if err != nil {
		return defaultBaseURL
	}

	override, err := parseOriginURL(overrideOrigin)
	if err != nil {
		return defaultBaseURL
	}

	if !isAllowedFrontendOriginOverride(base, override) {
		return defaultBaseURL
	}

	base.Scheme = override.Scheme
	base.Host = override.Host
	base.User = nil
	base.RawQuery = ""
	base.Fragment = ""

	return strings.TrimRight(base.String(), "/")
}

// parseOriginURL parses and validates a scheme+host-only origin URL.
func parseOriginURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("origin must include scheme and host")
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return nil, errors.New("origin must not include a path")
	}

	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("origin must not include query or fragment")
	}

	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed, nil
}

// isAllowedFrontendOriginOverride reports whether override is the same origin as base or a loopback alias with the same scheme and port.
func isAllowedFrontendOriginOverride(base *url.URL, override *url.URL) bool {
	if sameOrigin(base, override) {
		return true
	}

	return base.Scheme == override.Scheme &&
		originPort(base) == originPort(override) &&
		isLoopbackHostname(base.Hostname()) &&
		isLoopbackHostname(override.Hostname())
}

// sameOrigin reports whether two URLs share the same scheme, host, and port.
func sameOrigin(left *url.URL, right *url.URL) bool {
	return left.Scheme == right.Scheme &&
		originPort(left) == originPort(right) &&
		normalizeOriginHostname(left.Hostname()) ==
			normalizeOriginHostname(right.Hostname())
}

// originPort returns the port for a URL, defaulting to 443 for HTTPS and 80 for HTTP.
func originPort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}

	switch value.Scheme {
	case "https":
		return "443"
	default:
		return "80"
	}
}

// isLoopbackHostname reports whether hostname is a loopback address.
func isLoopbackHostname(hostname string) bool {
	switch normalizeOriginHostname(hostname) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// normalizeOriginHostname lowercases and strips brackets from an IPv6 hostname.
func normalizeOriginHostname(hostname string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(hostname)), "[]")
}

// resolveFrontendURL resolves path against baseURL and returns the absolute URL string.
func resolveFrontendURL(baseURL string, path string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse frontend base URL: %w", err)
	}

	reference, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse frontend path: %w", err)
	}

	return base.ResolveReference(reference).String(), nil
}

// normalizeFrontendPath validates and normalizes a frontend redirect path, returning fallback if invalid.
func normalizeFrontendPath(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fallback
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return fallback
	}

	if parsed.IsAbs() || parsed.Host != "" {
		return fallback
	}

	if parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/") {
		return fallback
	}

	result := parsed.Path
	if parsed.RawQuery != "" {
		result += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		result += "#" + parsed.Fragment
	}

	return result
}

// SessionCookieExpiry returns the Max-Age seconds for a session cookie, or 0 if expired or zero.
func SessionCookieExpiry(expiresAt time.Time) int {
	if expiresAt.IsZero() {
		return 0
	}

	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		return 0
	}

	return maxAge
}

// pkceChallengeS256 computes the S256 PKCE code challenge for the given verifier.
func pkceChallengeS256(codeVerifier string) string {
	sum := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
