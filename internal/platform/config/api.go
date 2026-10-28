package config

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"
)

const maxAdminContactEmailBytes = 254

// LoadApi reads the management API configuration from a strict YAML file.
func LoadApi(configPath string) (APIConfig, error) {
	file, err := loadFile(configPath)
	if err != nil {
		return APIConfig{}, err
	}
	usageCost, err := file.usageCostConfig()
	if err != nil {
		return APIConfig{}, err
	}

	cfg := APIConfig{
		EnvironmentConfig: file.environmentConfig(),
		LogConfig:         file.logConfig(),
		DatabaseURLConfig: file.databaseConfig(),
		TLSConfig:         file.tlsConfig(),
		RedisConfig:       file.redisConfig(),
		ServerConfig: ServerConfig{
			Port:               string(file.API.Port),
			CORSAllowedOrigins: append([]string(nil), file.Common.CORS.AllowedOrigins...),
		},
		SessionConfig: file.sessionConfig(),
		SecretsConfig: file.secretsConfig(),
		KeycloakConfig: KeycloakConfig{
			KeycloakIssuerURL:    strings.TrimSpace(file.API.OIDC.IssuerURL),
			KeycloakJWKSURL:      strings.TrimSpace(file.API.OIDC.JWKSURL),
			KeycloakClientID:     strings.TrimSpace(file.API.OIDC.ClientID),
			KeycloakClientSecret: strings.TrimSpace(file.API.OIDC.ClientSecret),
			OIDCRedirectURL:      strings.TrimSpace(file.API.OIDC.RedirectURL),
		},
		PublicURLConfig: file.publicURLConfig(),
		APIHTTPConfig: APIHTTPConfig{
			StaticAssetsDir:   file.API.StaticAssetsDir,
			AdminAPIKey:       strings.TrimSpace(file.API.AdminAPIKey),
			AdminContactEmail: strings.TrimSpace(file.API.AdminContactEmail),
		},
		ScheduleIntervals: ScheduleIntervals{
			TokenCleanupInterval:           time.Duration(file.Common.Maintenance.TokenCleanupInterval),
			UserAccessExpirationInterval:   time.Duration(file.Common.Maintenance.UserAccessExpirationInterval),
			QuotaResetDistributionInterval: time.Duration(file.Common.Maintenance.QuotaResetDistributionInterval),
		},
		ProxyRuntimeConfig: ProxyRuntimeConfig{ProxyReloadDebounce: time.Duration(file.Common.ProxyReloadDebounce)},
		ModelLab:           file.modelLabConfig(),
		UsageCost:          usageCost,
	}

	if err := validateDatabaseConfig(cfg.DatabaseURLConfig); err != nil {
		return APIConfig{}, err
	}
	if err := validateRedisConfig(cfg.RedisConfig); err != nil {
		return APIConfig{}, err
	}
	if err := validateRedisOverrides(file.Common.Redis); err != nil {
		return APIConfig{}, err
	}
	if cfg.KeycloakIssuerURL == "" {
		return APIConfig{}, errors.New("api.oidc.issuer_url is required")
	}
	if cfg.KeycloakJWKSURL == "" {
		return APIConfig{}, errors.New("api.oidc.jwks_url is required")
	}
	if cfg.KeycloakClientID == "" {
		return APIConfig{}, errors.New("api.oidc.client_id is required")
	}
	if err := validateAdminContactEmail(cfg.AdminContactEmail); err != nil {
		return APIConfig{}, err
	}
	if err := validateOptionalFile("common.tls.ca_file", cfg.CAFile); err != nil {
		return APIConfig{}, err
	}
	if cfg.FrontendBaseURL == "" {
		return APIConfig{}, errors.New("common.public_urls.frontend is required")
	}
	if cfg.BackendBaseURL == "" {
		return APIConfig{}, errors.New("common.public_urls.backend is required")
	}
	if cfg.OIDCRedirectURL == "" {
		cfg.OIDCRedirectURL = cfg.OIDCCallbackURL()
	}
	if err := validateOIDCRedirectURL(cfg.OIDCRedirectURL); err != nil {
		return APIConfig{}, err
	}
	if cfg.ProxyBaseURL == "" {
		cfg.ProxyBaseURL = deriveProxyBaseURL(cfg.BackendBaseURL, string(file.Proxy.Port))
	}
	if err := validateModelLabProxyBaseURL(cfg.ProxyBaseURL); err != nil {
		return APIConfig{}, err
	}
	if cfg.SessionTTL <= 0 {
		return APIConfig{}, errors.New("common.session.ttl must be greater than zero")
	}
	if cfg.JWTSecret == "" {
		return APIConfig{}, errors.New("common.security.jwt_secret is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return APIConfig{}, errors.New("common.security.jwt_secret must be at least 32 characters")
	}
	if cfg.SecretsKey == "" {
		return APIConfig{}, errors.New("common.security.secrets_key is required")
	}
	if err := validateModelLabConfig(cfg.ModelLab); err != nil {
		return APIConfig{}, err
	}
	if err := validatePositiveDurations(
		positiveDuration{"common.maintenance.token_cleanup_interval", cfg.TokenCleanupInterval},
		positiveDuration{"common.maintenance.user_access_expiration_interval", cfg.UserAccessExpirationInterval},
		positiveDuration{"common.maintenance.quota_reset_distribution_interval", cfg.QuotaResetDistributionInterval},
		positiveDuration{"common.proxy_reload_debounce", cfg.ProxyReloadDebounce},
	); err != nil {
		return APIConfig{}, err
	}
	if cfg.StaticAssetsDir != "" {
		info, err := os.Stat(cfg.StaticAssetsDir)
		if err != nil {
			return APIConfig{}, fmt.Errorf("api.static_assets_dir is not accessible: %w", err)
		}
		if !info.IsDir() {
			return APIConfig{}, errors.New("api.static_assets_dir must be a directory")
		}
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		cfg.CORSAllowedOrigins = []string{cfg.FrontendBaseURL}
	}
	cfg.CORSAllowedOrigins = expandLoopbackOrigins(cfg.CORSAllowedOrigins)
	return cfg, nil
}

func validateAdminContactEmail(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxAdminContactEmailBytes {
		return errors.New("api.admin_contact_email must be at most 254 bytes")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return errors.New("api.admin_contact_email must be one valid email address without a display name")
	}
	return nil
}
