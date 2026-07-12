package config

import (
	"errors"
	"strings"

	"github.com/spf13/viper"

	"promptgate/backend/internal/platform/proxylimits"
)

// LoadProxy reads the configuration required to run the LLM proxy server.
func LoadProxy() (ProxyConfig, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("proxy_port", "8081")
	v.SetDefault("log_level", "info")
	v.SetDefault("session_cookie_name", "promptgate_session")
	v.SetDefault("session_ttl", "8h")
	v.SetDefault("proxy_trust_forward_headers", false)
	v.SetDefault("proxy_trusted_proxies", "")
	v.SetDefault("redis_cache_ttl", "5m")
	v.SetDefault("proxy_reload_debounce", "250ms")
	v.SetDefault("proxy_max_buffered_request_bytes", proxylimits.DefaultMaxBufferedRequestBytes)
	v.SetDefault("proxy_max_buffered_response_bytes", proxylimits.DefaultMaxBufferedResponseBytes)
	v.SetDefault("proxy_upstream_timeout", proxylimits.DefaultUpstreamTimeout)

	trustedProxies, err := parseCIDRList(
		v.GetString("proxy_trusted_proxies"),
		"PROMPTGATE_PROXY_TRUSTED_PROXIES",
	)
	if err != nil {
		return ProxyConfig{}, err
	}

	cfg := ProxyConfig{
		LogConfig:         loadLogConfig(v),
		DatabaseURLConfig: loadDatabaseURLConfig(v),
		TLSConfig:         loadTLSConfig(v),
		RedisConfig:       loadRedisConfig(v),
		ServerConfig:      loadServerConfig(v, "proxy_port"),
		SessionConfig:     loadSessionConfig(v),
		SecretsConfig:     loadSecretsConfig(v),
		PublicURLConfig: PublicURLConfig{
			FrontendBaseURL: strings.TrimRight(strings.TrimSpace(v.GetString("frontend_base_url")), "/"),
		},
		ProxyRuntimeConfig: ProxyRuntimeConfig{
			ProxyTrustForwardHeaders:      v.GetBool("proxy_trust_forward_headers"),
			ProxyTrustedProxies:           trustedProxies,
			ProxyReloadDebounce:           v.GetDuration("proxy_reload_debounce"),
			ProxyMaxBufferedRequestBytes:  v.GetInt64("proxy_max_buffered_request_bytes"),
			ProxyMaxBufferedResponseBytes: v.GetInt64("proxy_max_buffered_response_bytes"),
			ProxyUpstreamTimeout:          v.GetDuration("proxy_upstream_timeout"),
		},
	}

	if cfg.DatabaseURL == "" {
		return ProxyConfig{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return ProxyConfig{}, errors.New("PROMPTGATE_JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return ProxyConfig{}, errors.New("PROMPTGATE_JWT_SECRET must be at least 32 characters")
	}
	if cfg.SecretsKey == "" {
		return ProxyConfig{}, errors.New("PROMPTGATE_SECRETS_KEY is required")
	}
	if cfg.SessionTTL <= 0 {
		return ProxyConfig{}, errors.New("PROMPTGATE_SESSION_TTL must be greater than zero")
	}
	if cfg.RedisURL == "" {
		return ProxyConfig{}, errors.New("PROMPTGATE_REDIS_URL is required")
	}
	if err := validateOptionalFile("PROMPTGATE_CA_FILE", cfg.CAFile); err != nil {
		return ProxyConfig{}, err
	}
	if cfg.ProxyMaxBufferedRequestBytes <= 0 {
		return ProxyConfig{}, errors.New("PROMPTGATE_PROXY_MAX_BUFFERED_REQUEST_BYTES must be greater than zero")
	}
	if cfg.ProxyMaxBufferedResponseBytes <= 0 {
		return ProxyConfig{}, errors.New("PROMPTGATE_PROXY_MAX_BUFFERED_RESPONSE_BYTES must be greater than zero")
	}
	if cfg.ProxyUpstreamTimeout <= 0 {
		return ProxyConfig{}, errors.New("PROMPTGATE_PROXY_UPSTREAM_TIMEOUT must be greater than zero")
	}
	if err := validatePositiveDurations(
		positiveDuration{"PROMPTGATE_REDIS_CACHE_TTL", cfg.RedisCacheTTL},
		positiveDuration{"PROMPTGATE_PROXY_RELOAD_DEBOUNCE", cfg.ProxyReloadDebounce},
	); err != nil {
		return ProxyConfig{}, err
	}
	if len(cfg.CORSAllowedOrigins) == 0 && cfg.FrontendBaseURL != "" {
		cfg.CORSAllowedOrigins = []string{cfg.FrontendBaseURL}
	}
	cfg.CORSAllowedOrigins = expandLoopbackOrigins(cfg.CORSAllowedOrigins)

	return cfg, nil
}
