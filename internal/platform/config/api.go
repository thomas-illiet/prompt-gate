package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// LoadApi reads configuration from environment variables prefixed with PROMPTGATE_ and validates required fields.
func LoadApi() (APIConfig, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("port", "8080")
	v.SetDefault("log_level", "info")
	v.SetDefault("session_cookie_name", "promptgate_session")
	v.SetDefault("session_ttl", "8h")
	v.SetDefault("token_cleanup_interval", "1h")
	v.SetDefault("user_access_expiration_interval", "1h")
	v.SetDefault("proxy_port", "8081")
	v.SetDefault("redis_cache_ttl", "5m")
	v.SetDefault("proxy_reload_debounce", "250ms")
	v.SetDefault("usage_cost_enabled", true)
	v.SetDefault("usage_cost_input", "5.00")
	v.SetDefault("usage_cost_output", "30.00")
	v.SetDefault("usage_cost_embedding", "0.02")

	usageCost, err := loadUsageCostConfig(v)
	if err != nil {
		return APIConfig{}, err
	}

	cfg := APIConfig{
		LogConfig:         loadLogConfig(v),
		DatabaseURLConfig: loadDatabaseURLConfig(v),
		TLSConfig:         loadTLSConfig(v),
		RedisConfig:       loadRedisConfig(v),
		ServerConfig:      loadServerConfig(v, "port"),
		SessionConfig:     loadSessionConfig(v),
		SecretsConfig:     loadSecretsConfig(v),
		KeycloakConfig: KeycloakConfig{
			KeycloakIssuerURL:    strings.TrimSpace(v.GetString("keycloak_issuer_url")),
			KeycloakJWKSURL:      strings.TrimSpace(v.GetString("keycloak_jwks_url")),
			KeycloakClientID:     strings.TrimSpace(v.GetString("keycloak_client_id")),
			KeycloakClientSecret: strings.TrimSpace(v.GetString("keycloak_client_secret")),
		},
		PublicURLConfig: PublicURLConfig{
			FrontendBaseURL: strings.TrimRight(strings.TrimSpace(v.GetString("frontend_base_url")), "/"),
			BackendBaseURL:  strings.TrimRight(strings.TrimSpace(v.GetString("backend_base_url")), "/"),
			ProxyBaseURL:    strings.TrimRight(strings.TrimSpace(v.GetString("proxy_base_url")), "/"),
		},
		APIHTTPConfig: APIHTTPConfig{
			StaticAssetsDir: strings.TrimSpace(v.GetString("static_assets_dir")),
			AdminAPIKey:     strings.TrimSpace(v.GetString("admin_api_key")),
		},
		ScheduleIntervals: ScheduleIntervals{
			TokenCleanupInterval:         v.GetDuration("token_cleanup_interval"),
			UserAccessExpirationInterval: v.GetDuration("user_access_expiration_interval"),
		},
		ProxyRuntimeConfig: ProxyRuntimeConfig{
			ProxyReloadDebounce: v.GetDuration("proxy_reload_debounce"),
		},
		UsageCost: usageCost,
	}

	if cfg.DatabaseURL == "" {
		return APIConfig{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}

	if cfg.KeycloakIssuerURL == "" {
		return APIConfig{}, errors.New("PROMPTGATE_KEYCLOAK_ISSUER_URL is required")
	}

	if cfg.KeycloakJWKSURL == "" {
		return APIConfig{}, errors.New("PROMPTGATE_KEYCLOAK_JWKS_URL is required")
	}

	if cfg.KeycloakClientID == "" {
		return APIConfig{}, errors.New("PROMPTGATE_KEYCLOAK_CLIENT_ID is required")
	}

	if err := validateOptionalFile("PROMPTGATE_CA_FILE", cfg.CAFile); err != nil {
		return APIConfig{}, err
	}

	if cfg.FrontendBaseURL == "" {
		return APIConfig{}, errors.New("PROMPTGATE_FRONTEND_BASE_URL is required")
	}

	if cfg.BackendBaseURL == "" {
		return APIConfig{}, errors.New("PROMPTGATE_BACKEND_BASE_URL is required")
	}

	if cfg.ProxyBaseURL == "" {
		cfg.ProxyBaseURL = deriveProxyBaseURL(cfg.BackendBaseURL, v.GetString("proxy_port"))
	}

	if cfg.SessionTTL <= 0 {
		return APIConfig{}, errors.New("PROMPTGATE_SESSION_TTL must be greater than zero")
	}

	if cfg.JWTSecret == "" {
		return APIConfig{}, errors.New("PROMPTGATE_JWT_SECRET is required")
	}

	if len(cfg.JWTSecret) < 32 {
		return APIConfig{}, errors.New("PROMPTGATE_JWT_SECRET must be at least 32 characters")
	}

	if cfg.SecretsKey == "" {
		return APIConfig{}, errors.New("PROMPTGATE_SECRETS_KEY is required")
	}

	if cfg.RedisURL == "" {
		return APIConfig{}, errors.New("PROMPTGATE_REDIS_URL is required")
	}
	if err := validatePositiveDurations(
		positiveDuration{"PROMPTGATE_TOKEN_CLEANUP_INTERVAL", cfg.TokenCleanupInterval},
		positiveDuration{"PROMPTGATE_USER_ACCESS_EXPIRATION_INTERVAL", cfg.UserAccessExpirationInterval},
		positiveDuration{"PROMPTGATE_REDIS_CACHE_TTL", cfg.RedisCacheTTL},
		positiveDuration{"PROMPTGATE_PROXY_RELOAD_DEBOUNCE", cfg.ProxyReloadDebounce},
	); err != nil {
		return APIConfig{}, err
	}

	if cfg.StaticAssetsDir != "" {
		info, err := os.Stat(cfg.StaticAssetsDir)
		if err != nil {
			return APIConfig{}, fmt.Errorf("PROMPTGATE_STATIC_ASSETS_DIR is not accessible: %w", err)
		}
		if !info.IsDir() {
			return APIConfig{}, errors.New("PROMPTGATE_STATIC_ASSETS_DIR must be a directory")
		}
	}

	if len(cfg.CORSAllowedOrigins) == 0 {
		cfg.CORSAllowedOrigins = []string{cfg.FrontendBaseURL}
	}

	cfg.CORSAllowedOrigins = expandLoopbackOrigins(cfg.CORSAllowedOrigins)

	return cfg, nil
}
