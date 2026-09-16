package config

import (
	"errors"
	"fmt"
	"net/url"
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
	v.SetDefault("proxy_debug_requests", false)
	v.SetDefault("redis_cache_ttl", "5m")
	v.SetDefault("proxy_reload_debounce", "250ms")
	v.SetDefault("proxy_max_buffered_request_bytes", proxylimits.DefaultMaxBufferedRequestBytes)
	v.SetDefault("proxy_max_buffered_response_bytes", proxylimits.DefaultMaxBufferedResponseBytes)
	v.SetDefault("proxy_upstream_timeout", proxylimits.DefaultUpstreamTimeout)
	v.SetDefault("otel_enabled", false)
	v.SetDefault("otel_project_name", "prompt-gate")
	v.SetDefault("otel_service_name", "promptgate-proxy")
	v.SetDefault("otel_environment", "production")
	v.SetDefault("otel_capture_prompts", false)
	v.SetDefault("otel_capture_output", false)
	v.SetDefault("otel_export_timeout", "10s")
	v.SetDefault("otel_batch_timeout", "5s")
	v.SetDefault("otel_max_queue_size", 2048)
	v.SetDefault("otel_max_export_batch_size", 512)
	v.SetDefault("otel_insecure", false)
	v.SetDefault("usage_cost_enabled", true)
	v.SetDefault("usage_cost_input", "5.00")
	v.SetDefault("usage_cost_output", "30.00")
	v.SetDefault("usage_cost_embedding", "0.02")

	trustedProxies, err := parseCIDRList(
		v.GetString("proxy_trusted_proxies"),
		"PROMPTGATE_PROXY_TRUSTED_PROXIES",
	)
	if err != nil {
		return ProxyConfig{}, err
	}

	usageCost, err := loadUsageCostConfig(v)
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
			ProxyDebugRequests:            v.GetBool("proxy_debug_requests"),
			ProxyReloadDebounce:           v.GetDuration("proxy_reload_debounce"),
			ProxyMaxBufferedRequestBytes:  v.GetInt64("proxy_max_buffered_request_bytes"),
			ProxyMaxBufferedResponseBytes: v.GetInt64("proxy_max_buffered_response_bytes"),
			ProxyUpstreamTimeout:          v.GetDuration("proxy_upstream_timeout"),
		},
		OTel: OTelConfig{
			Enabled:            v.GetBool("otel_enabled"),
			Endpoint:           strings.TrimSpace(v.GetString("otel_endpoint")),
			APIKey:             strings.TrimSpace(v.GetString("otel_api_key")),
			ProjectName:        strings.TrimSpace(v.GetString("otel_project_name")),
			ServiceName:        strings.TrimSpace(v.GetString("otel_service_name")),
			Environment:        strings.TrimSpace(v.GetString("otel_environment")),
			CapturePrompts:     v.GetBool("otel_capture_prompts"),
			CaptureOutput:      v.GetBool("otel_capture_output"),
			ExportTimeout:      v.GetDuration("otel_export_timeout"),
			BatchTimeout:       v.GetDuration("otel_batch_timeout"),
			MaxQueueSize:       v.GetInt("otel_max_queue_size"),
			MaxExportBatchSize: v.GetInt("otel_max_export_batch_size"),
			CAFile:             strings.TrimSpace(v.GetString("otel_ca_file")),
			Insecure:           v.GetBool("otel_insecure"),
		},
		UsageCost: usageCost,
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
	if err := validateOTelConfig(cfg.OTel); err != nil {
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

func validateOTelConfig(cfg OTelConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Endpoint == "" {
		return errors.New("PROMPTGATE_OTEL_ENDPOINT is required when PROMPTGATE_OTEL_ENABLED is true")
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		return errors.New("PROMPTGATE_OTEL_ENDPOINT must be an absolute HTTP(S) URL")
	}
	if endpoint.Scheme != "https" && !cfg.Insecure {
		return errors.New("PROMPTGATE_OTEL_ENDPOINT must use https unless PROMPTGATE_OTEL_INSECURE is true")
	}
	if cfg.ProjectName == "" || cfg.ServiceName == "" || cfg.Environment == "" {
		return errors.New("PROMPTGATE_OTEL_PROJECT_NAME, PROMPTGATE_OTEL_SERVICE_NAME, and PROMPTGATE_OTEL_ENVIRONMENT must not be empty")
	}
	if err := validatePositiveDurations(
		positiveDuration{"PROMPTGATE_OTEL_EXPORT_TIMEOUT", cfg.ExportTimeout},
		positiveDuration{"PROMPTGATE_OTEL_BATCH_TIMEOUT", cfg.BatchTimeout},
	); err != nil {
		return err
	}
	if cfg.MaxQueueSize <= 0 || cfg.MaxExportBatchSize <= 0 {
		return errors.New("PROMPTGATE_OTEL queue and batch sizes must be greater than zero")
	}
	if cfg.MaxExportBatchSize > cfg.MaxQueueSize {
		return errors.New("PROMPTGATE_OTEL_MAX_EXPORT_BATCH_SIZE must not exceed PROMPTGATE_OTEL_MAX_QUEUE_SIZE")
	}
	if err := validateOptionalFile("PROMPTGATE_OTEL_CA_FILE", cfg.CAFile); err != nil {
		return fmt.Errorf("validate OpenTelemetry CA: %w", err)
	}
	return nil
}
