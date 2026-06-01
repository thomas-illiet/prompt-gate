package config

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Port                         string
	LogLevel                     string
	DatabaseURL                  string
	KeycloakIssuerURL            string
	KeycloakJWKSURL              string
	KeycloakClientID             string
	KeycloakClientSecret         string
	KeycloakCACertPath           string
	FrontendBaseURL              string
	BackendBaseURL               string
	ProxyBaseURL                 string
	StaticAssetsDir              string
	SessionCookieName            string
	SessionTTL                   time.Duration
	CORSAllowedOrigins           []string
	JWTSecret                    string
	SecretsKey                   string
	TokenCleanupInterval         time.Duration
	UserAccessExpirationInterval time.Duration
	ProxyTrustForwardHeaders     bool
	RedisURL                     string
	RedisCacheTTL                time.Duration
	ProxyReloadDebounce          time.Duration
	UsageCost                    UsageCostConfig
}

type UsageCostConfig struct {
	Enabled   bool
	Input     float64
	Output    float64
	Embedding float64
}

// LoadApi reads configuration from environment variables prefixed with PROMPTGATE_ and validates required fields.
func LoadApi() (Config, error) {
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
		return Config{}, err
	}

	cfg := Config{
		Port:                         v.GetString("port"),
		LogLevel:                     v.GetString("log_level"),
		DatabaseURL:                  strings.TrimSpace(v.GetString("database_url")),
		KeycloakIssuerURL:            strings.TrimSpace(v.GetString("keycloak_issuer_url")),
		KeycloakJWKSURL:              strings.TrimSpace(v.GetString("keycloak_jwks_url")),
		KeycloakClientID:             strings.TrimSpace(v.GetString("keycloak_client_id")),
		KeycloakClientSecret:         strings.TrimSpace(v.GetString("keycloak_client_secret")),
		KeycloakCACertPath:           strings.TrimSpace(v.GetString("keycloak_ca_cert_path")),
		FrontendBaseURL:              strings.TrimRight(strings.TrimSpace(v.GetString("frontend_base_url")), "/"),
		BackendBaseURL:               strings.TrimRight(strings.TrimSpace(v.GetString("backend_base_url")), "/"),
		ProxyBaseURL:                 strings.TrimRight(strings.TrimSpace(v.GetString("proxy_base_url")), "/"),
		StaticAssetsDir:              strings.TrimSpace(v.GetString("static_assets_dir")),
		SessionCookieName:            v.GetString("session_cookie_name"),
		SessionTTL:                   v.GetDuration("session_ttl"),
		CORSAllowedOrigins:           v.GetStringSlice("cors_allowed_origins"),
		JWTSecret:                    strings.TrimSpace(v.GetString("jwt_secret")),
		SecretsKey:                   strings.TrimSpace(v.GetString("secrets_key")),
		TokenCleanupInterval:         v.GetDuration("token_cleanup_interval"),
		UserAccessExpirationInterval: v.GetDuration("user_access_expiration_interval"),
		RedisURL:                     strings.TrimSpace(v.GetString("redis_url")),
		RedisCacheTTL:                v.GetDuration("redis_cache_ttl"),
		ProxyReloadDebounce:          v.GetDuration("proxy_reload_debounce"),
		UsageCost:                    usageCost,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}

	if cfg.KeycloakIssuerURL == "" {
		return Config{}, errors.New("PROMPTGATE_KEYCLOAK_ISSUER_URL is required")
	}

	if cfg.KeycloakJWKSURL == "" {
		return Config{}, errors.New("PROMPTGATE_KEYCLOAK_JWKS_URL is required")
	}

	if cfg.KeycloakClientID == "" {
		return Config{}, errors.New("PROMPTGATE_KEYCLOAK_CLIENT_ID is required")
	}

	if cfg.KeycloakCACertPath != "" {
		info, err := os.Stat(cfg.KeycloakCACertPath)
		if err != nil {
			return Config{}, fmt.Errorf("PROMPTGATE_KEYCLOAK_CA_CERT_PATH is not accessible: %w", err)
		}
		if info.IsDir() {
			return Config{}, errors.New("PROMPTGATE_KEYCLOAK_CA_CERT_PATH must be a file")
		}
	}

	if cfg.FrontendBaseURL == "" {
		return Config{}, errors.New("PROMPTGATE_FRONTEND_BASE_URL is required")
	}

	if cfg.BackendBaseURL == "" {
		return Config{}, errors.New("PROMPTGATE_BACKEND_BASE_URL is required")
	}

	if cfg.ProxyBaseURL == "" {
		cfg.ProxyBaseURL = deriveProxyBaseURL(cfg.BackendBaseURL, v.GetString("proxy_port"))
	}

	if cfg.SessionTTL <= 0 {
		return Config{}, errors.New("PROMPTGATE_SESSION_TTL must be greater than zero")
	}

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("PROMPTGATE_JWT_SECRET is required")
	}

	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("PROMPTGATE_JWT_SECRET must be at least 32 characters")
	}

	if cfg.SecretsKey == "" {
		return Config{}, errors.New("PROMPTGATE_SECRETS_KEY is required")
	}

	if cfg.RedisURL == "" {
		return Config{}, errors.New("PROMPTGATE_REDIS_URL is required")
	}

	if cfg.StaticAssetsDir != "" {
		info, err := os.Stat(cfg.StaticAssetsDir)
		if err != nil {
			return Config{}, fmt.Errorf("PROMPTGATE_STATIC_ASSETS_DIR is not accessible: %w", err)
		}
		if !info.IsDir() {
			return Config{}, errors.New("PROMPTGATE_STATIC_ASSETS_DIR must be a directory")
		}
	}

	if len(cfg.CORSAllowedOrigins) == 0 {
		cfg.CORSAllowedOrigins = []string{cfg.FrontendBaseURL}
	}

	cfg.CORSAllowedOrigins = expandLoopbackOrigins(cfg.CORSAllowedOrigins)

	return cfg, nil
}

// LoadSchedule reads configuration required to run scheduled background jobs.
func LoadSchedule() (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("log_level", "info")
	v.SetDefault("token_cleanup_interval", "1h")
	v.SetDefault("user_access_expiration_interval", "1h")
	v.SetDefault("redis_cache_ttl", "5m")
	v.SetDefault("proxy_reload_debounce", "250ms")

	cfg := Config{
		LogLevel:                     v.GetString("log_level"),
		DatabaseURL:                  strings.TrimSpace(v.GetString("database_url")),
		JWTSecret:                    strings.TrimSpace(v.GetString("jwt_secret")),
		SecretsKey:                   strings.TrimSpace(v.GetString("secrets_key")),
		TokenCleanupInterval:         v.GetDuration("token_cleanup_interval"),
		UserAccessExpirationInterval: v.GetDuration("user_access_expiration_interval"),
		RedisURL:                     strings.TrimSpace(v.GetString("redis_url")),
		RedisCacheTTL:                v.GetDuration("redis_cache_ttl"),
		ProxyReloadDebounce:          v.GetDuration("proxy_reload_debounce"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("PROMPTGATE_JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("PROMPTGATE_JWT_SECRET must be at least 32 characters")
	}
	if cfg.SecretsKey == "" {
		return Config{}, errors.New("PROMPTGATE_SECRETS_KEY is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, errors.New("PROMPTGATE_REDIS_URL is required")
	}

	return cfg, nil
}

// LoadProxy reads the configuration required to run the LLM proxy server.
func LoadProxy() (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("proxy_port", "8081")
	v.SetDefault("log_level", "info")
	v.SetDefault("session_cookie_name", "promptgate_session")
	v.SetDefault("session_ttl", "8h")
	v.SetDefault("proxy_trust_forward_headers", false)
	v.SetDefault("redis_cache_ttl", "5m")
	v.SetDefault("proxy_reload_debounce", "250ms")

	cfg := Config{
		Port:                     v.GetString("proxy_port"),
		LogLevel:                 v.GetString("log_level"),
		DatabaseURL:              strings.TrimSpace(v.GetString("database_url")),
		FrontendBaseURL:          strings.TrimRight(strings.TrimSpace(v.GetString("frontend_base_url")), "/"),
		SessionCookieName:        v.GetString("session_cookie_name"),
		SessionTTL:               v.GetDuration("session_ttl"),
		CORSAllowedOrigins:       v.GetStringSlice("cors_allowed_origins"),
		JWTSecret:                strings.TrimSpace(v.GetString("jwt_secret")),
		SecretsKey:               strings.TrimSpace(v.GetString("secrets_key")),
		ProxyTrustForwardHeaders: v.GetBool("proxy_trust_forward_headers"),
		RedisURL:                 strings.TrimSpace(v.GetString("redis_url")),
		RedisCacheTTL:            v.GetDuration("redis_cache_ttl"),
		ProxyReloadDebounce:      v.GetDuration("proxy_reload_debounce"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("PROMPTGATE_JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("PROMPTGATE_JWT_SECRET must be at least 32 characters")
	}
	if cfg.SecretsKey == "" {
		return Config{}, errors.New("PROMPTGATE_SECRETS_KEY is required")
	}
	if cfg.SessionTTL <= 0 {
		return Config{}, errors.New("PROMPTGATE_SESSION_TTL must be greater than zero")
	}
	if cfg.RedisURL == "" {
		return Config{}, errors.New("PROMPTGATE_REDIS_URL is required")
	}
	if len(cfg.CORSAllowedOrigins) == 0 && cfg.FrontendBaseURL != "" {
		cfg.CORSAllowedOrigins = []string{cfg.FrontendBaseURL}
	}
	cfg.CORSAllowedOrigins = expandLoopbackOrigins(cfg.CORSAllowedOrigins)

	return cfg, nil
}

// LoadMigration reads the configuration required to run database migrations.
func LoadMigration() (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("log_level", "info")

	cfg := Config{
		LogLevel:    v.GetString("log_level"),
		DatabaseURL: strings.TrimSpace(v.GetString("database_url")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}

	return cfg, nil
}

// loadUsageCostConfig reads dashboard-only usage cost settings.
func loadUsageCostConfig(v *viper.Viper) (UsageCostConfig, error) {
	input, err := parseNonNegativeFloat(v, "usage_cost_input", "PROMPTGATE_USAGE_COST_INPUT")
	if err != nil {
		return UsageCostConfig{}, err
	}
	output, err := parseNonNegativeFloat(v, "usage_cost_output", "PROMPTGATE_USAGE_COST_OUTPUT")
	if err != nil {
		return UsageCostConfig{}, err
	}
	embedding, err := parseNonNegativeFloat(v, "usage_cost_embedding", "PROMPTGATE_USAGE_COST_EMBEDDING")
	if err != nil {
		return UsageCostConfig{}, err
	}
	return UsageCostConfig{
		Enabled:   v.GetBool("usage_cost_enabled"),
		Input:     input,
		Output:    output,
		Embedding: embedding,
	}, nil
}

// parseNonNegativeFloat parses a non-negative float from a viper key.
func parseNonNegativeFloat(v *viper.Viper, key string, envName string) (float64, error) {
	raw := strings.TrimSpace(v.GetString(key))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s must be a valid number", envName)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to zero", envName)
	}
	return value, nil
}

// ListenAddress returns the address the server should bind to, ensuring it starts with ":".
func (c Config) ListenAddress() string {
	if strings.HasPrefix(c.Port, ":") {
		return c.Port
	}

	return fmt.Sprintf(":%s", c.Port)
}

// OIDCCallbackURL returns the full OIDC redirect callback URL.
func (c Config) OIDCCallbackURL() string {
	return c.BackendBaseURL + "/auth/callback"
}

// deriveProxyBaseURL returns the API origin with the configured proxy port.
func deriveProxyBaseURL(backendBaseURL string, proxyPort string) string {
	parsed, err := url.Parse(backendBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return ""
	}
	port := strings.TrimPrefix(strings.TrimSpace(proxyPort), ":")
	if port == "" {
		port = "8081"
	}
	parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

// SessionCookieSecure returns true when the backend base URL uses HTTPS.
func (c Config) SessionCookieSecure() bool {
	return strings.HasPrefix(c.BackendBaseURL, "https://")
}

// SlogLevel converts the configured log level string to a slog.Level.
func (c Config) SlogLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// DatabaseLogValue returns a redacted database connection string safe for logging.
func (c Config) DatabaseLogValue() string {
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return "unparseable_database_url"
	}

	databaseName := path.Base(parsed.Path)
	if databaseName == "." || databaseName == "/" || databaseName == "" {
		databaseName = "unknown"
	}

	host := parsed.Hostname()
	if host == "" {
		host = "unknown"
	}

	port := parsed.Port()
	if port == "" {
		port = "default"
	}

	return fmt.Sprintf("%s/%s (port=%s)", host, databaseName, port)
}

// expandLoopbackOrigins expands loopback origins to include all loopback aliases (localhost, 127.0.0.1, ::1).
func expandLoopbackOrigins(origins []string) []string {
	values := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))

	for _, origin := range origins {
		normalized := strings.TrimRight(strings.TrimSpace(origin), "/")
		if normalized == "" {
			continue
		}

		addOrigin(&values, seen, normalized)

		parsed, err := url.Parse(normalized)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}

		if !isLoopbackHost(parsed.Hostname()) {
			continue
		}

		if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			continue
		}

		for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
			alias := url.URL{
				Scheme: parsed.Scheme,
				Host:   joinOriginHostPort(host, parsed.Port()),
			}
			addOrigin(&values, seen, alias.String())
		}
	}

	return values
}

// addOrigin appends origin to values if not already in seen.
func addOrigin(values *[]string, seen map[string]struct{}, origin string) {
	if _, ok := seen[origin]; ok {
		return
	}

	seen[origin] = struct{}{}
	*values = append(*values, origin)
}

// joinOriginHostPort formats a host and port into a valid URL host component.
func joinOriginHostPort(host string, port string) string {
	if port == "" {
		if host == "::1" {
			return "[::1]"
		}

		return host
	}

	return net.JoinHostPort(host, port)
}

// isLoopbackHost reports whether the host is a loopback address.
func isLoopbackHost(host string) bool {
	switch strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]") {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
