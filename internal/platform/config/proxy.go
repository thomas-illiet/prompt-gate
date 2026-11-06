package config

import (
	"errors"
	"net/url"
	"strings"
	"time"

	platformhttp "promptgate/backend/internal/platform/httpclient"
)

// LoadProxy reads the LLM proxy configuration from a strict YAML file.
func LoadProxy(configPath string) (ProxyConfig, error) {
	file, err := loadFile(configPath)
	if err != nil {
		return ProxyConfig{}, err
	}
	trustedProxies, err := parseCIDRList(file.Proxy.TrustedProxies, "proxy.trusted_proxies")
	if err != nil {
		return ProxyConfig{}, err
	}
	usageCost, err := file.usageCostConfig()
	if err != nil {
		return ProxyConfig{}, err
	}

	cfg := ProxyConfig{
		LogConfig:         file.logConfig(),
		DatabaseURLConfig: file.databaseConfig(),
		TLSConfig:         file.tlsConfig(),
		RedisConfig:       file.redisConfig(),
		ServerConfig: ServerConfig{
			Port:               string(file.Proxy.Port),
			CORSAllowedOrigins: append([]string(nil), file.Common.CORS.AllowedOrigins...),
		},
		SessionConfig:   file.sessionConfig(),
		SecretsConfig:   file.secretsConfig(),
		PublicURLConfig: file.publicURLConfig(),
		ProxyRuntimeConfig: ProxyRuntimeConfig{
			ProxyBasePath:                 file.Proxy.BasePath,
			ProxyTrustForwardHeaders:      file.Proxy.TrustForwardHeaders,
			ProxyTrustedProxies:           trustedProxies,
			ProxyClientIPDebug:            file.Proxy.ClientIPDebug,
			ProxyDebugRequests:            file.Proxy.DebugRequests,
			ProxyReloadDebounce:           time.Duration(file.Common.ProxyReloadDebounce),
			ProxyMaxBufferedRequestBytes:  file.Proxy.MaxBufferedRequestBytes,
			ProxyMaxBufferedResponseBytes: file.Proxy.MaxBufferedResponseBytes,
			ProxyUpstreamTimeout:          time.Duration(file.Proxy.UpstreamTimeout),
			IPValidationEnabled:           file.Proxy.IPValidation.Enabled,
			IPValidationURLTemplate:       strings.TrimSpace(file.Proxy.IPValidation.URLTemplate),
			IPValidationBearerToken:       strings.TrimSpace(file.Proxy.IPValidation.BearerToken),
			IPValidationTimeout:           time.Duration(file.Proxy.IPValidation.Timeout),
			IPValidationCacheTTL:          time.Duration(file.Proxy.IPValidation.CacheTTL),
		},
		ModelLab: file.modelLabConfig(),
		OTel: OTelConfig{
			Enabled:            file.Proxy.OTel.Enabled,
			Endpoint:           strings.TrimSpace(file.Proxy.OTel.Endpoint),
			APIKey:             strings.TrimSpace(file.Proxy.OTel.APIKey),
			ProjectName:        strings.TrimSpace(file.Proxy.OTel.ProjectName),
			ServiceName:        strings.TrimSpace(file.Proxy.OTel.ServiceName),
			Environment:        strings.TrimSpace(file.Proxy.OTel.Environment),
			CapturePrompts:     file.Proxy.OTel.CapturePrompts,
			CaptureOutput:      file.Proxy.OTel.CaptureOutput,
			CaptureThinking:    file.Proxy.OTel.CaptureThinking,
			ExportTimeout:      time.Duration(file.Proxy.OTel.ExportTimeout),
			BatchTimeout:       time.Duration(file.Proxy.OTel.BatchTimeout),
			MaxQueueSize:       file.Proxy.OTel.MaxQueueSize,
			MaxExportBatchSize: file.Proxy.OTel.MaxExportBatchSize,
			Insecure:           file.Proxy.OTel.Insecure,
		},
		UsageCost: usageCost,
	}

	if err := validateDatabaseConfig(cfg.DatabaseURLConfig); err != nil {
		return ProxyConfig{}, err
	}
	if err := validateRedisConfig(cfg.RedisConfig); err != nil {
		return ProxyConfig{}, err
	}
	if err := validateRedisOverrides(file.Common.Redis); err != nil {
		return ProxyConfig{}, err
	}
	if cfg.JWTSecret == "" {
		return ProxyConfig{}, errors.New("common.security.jwt_secret is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return ProxyConfig{}, errors.New("common.security.jwt_secret must be at least 32 characters")
	}
	if cfg.SecretsKey == "" {
		return ProxyConfig{}, errors.New("common.security.secrets_key is required")
	}
	if err := validateModelLabConfig(cfg.ModelLab); err != nil {
		return ProxyConfig{}, err
	}
	if cfg.SessionTTL <= 0 {
		return ProxyConfig{}, errors.New("common.session.ttl must be greater than zero")
	}
	if cfg.ProxyBaseURL == "" {
		return ProxyConfig{}, errors.New("common.public_urls.proxy is required")
	}
	if err := validateModelLabProxyBaseURL(cfg.ProxyBaseURL); err != nil {
		return ProxyConfig{}, err
	}
	if err := validateProxyBasePath(cfg.ProxyBasePath); err != nil {
		return ProxyConfig{}, err
	}
	if err := validateProxyPublicURLPath(cfg.ProxyBaseURL, cfg.ProxyBasePath); err != nil {
		return ProxyConfig{}, err
	}
	if err := validateOptionalFile("common.tls.ca_file", cfg.CAFile); err != nil {
		return ProxyConfig{}, err
	}
	if err := validateOTelConfig(cfg.OTel); err != nil {
		return ProxyConfig{}, err
	}
	if cfg.ProxyMaxBufferedRequestBytes <= 0 {
		return ProxyConfig{}, errors.New("proxy.max_buffered_request_bytes must be greater than zero")
	}
	if cfg.ModelLab.MaxPromptBytes > cfg.ProxyMaxBufferedRequestBytes {
		return ProxyConfig{}, errors.New("common.model_lab.max_prompt_bytes must not exceed proxy.max_buffered_request_bytes")
	}
	if cfg.ProxyMaxBufferedResponseBytes <= 0 {
		return ProxyConfig{}, errors.New("proxy.max_buffered_response_bytes must be greater than zero")
	}
	if cfg.ProxyUpstreamTimeout <= 0 {
		return ProxyConfig{}, errors.New("proxy.upstream_timeout must be greater than zero")
	}
	if cfg.IPValidationEnabled {
		if err := platformhttp.ValidateIPValidation(cfg.IPValidationURLTemplate, cfg.IPValidationBearerToken); err != nil {
			return ProxyConfig{}, err
		}
	}
	positiveDurations := []positiveDuration{{"common.proxy_reload_debounce", cfg.ProxyReloadDebounce}}
	if cfg.IPValidationEnabled {
		positiveDurations = append(positiveDurations,
			positiveDuration{"proxy.ip_validation.timeout", cfg.IPValidationTimeout},
			positiveDuration{"proxy.ip_validation.cache_ttl", cfg.IPValidationCacheTTL},
		)
	}
	if err := validatePositiveDurations(positiveDurations...); err != nil {
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
		return errors.New("proxy.otel.endpoint is required when proxy.otel.enabled is true")
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		return errors.New("proxy.otel.endpoint must be an absolute HTTP(S) URL")
	}
	if endpoint.Scheme != "https" && !cfg.Insecure {
		return errors.New("proxy.otel.endpoint must use https unless proxy.otel.insecure is true")
	}
	if cfg.ProjectName == "" || cfg.ServiceName == "" || cfg.Environment == "" {
		return errors.New("proxy.otel.project_name, proxy.otel.service_name, and proxy.otel.environment must not be empty")
	}
	if err := validatePositiveDurations(
		positiveDuration{"proxy.otel.export_timeout", cfg.ExportTimeout},
		positiveDuration{"proxy.otel.batch_timeout", cfg.BatchTimeout},
	); err != nil {
		return err
	}
	if cfg.MaxQueueSize <= 0 || cfg.MaxExportBatchSize <= 0 {
		return errors.New("proxy.otel queue and batch sizes must be greater than zero")
	}
	if cfg.MaxExportBatchSize > cfg.MaxQueueSize {
		return errors.New("proxy.otel.max_export_batch_size must not exceed proxy.otel.max_queue_size")
	}
	return nil
}
