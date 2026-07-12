package config

import (
	"errors"
	"github.com/spf13/viper"
)

// LoadSchedule reads configuration required to run scheduled background jobs.
func LoadSchedule() (ScheduleConfig, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("log_level", "info")
	v.SetDefault("token_cleanup_interval", "1h")
	v.SetDefault("user_access_expiration_interval", "1h")
	v.SetDefault("redis_cache_ttl", "5m")
	v.SetDefault("proxy_reload_debounce", "250ms")
	v.SetDefault("usage_raw_retention", "2160h")
	v.SetDefault("usage_raw_cleanup_interval", "1h")
	v.SetDefault("subscription_quota_sync_interval", "5m")

	cfg := ScheduleConfig{
		LogConfig:         loadLogConfig(v),
		DatabaseURLConfig: loadDatabaseURLConfig(v),
		TLSConfig:         loadTLSConfig(v),
		RedisConfig:       loadRedisConfig(v),
		SecretsConfig:     loadSecretsConfig(v),
		ScheduleIntervals: ScheduleIntervals{
			TokenCleanupInterval:          v.GetDuration("token_cleanup_interval"),
			UserAccessExpirationInterval:  v.GetDuration("user_access_expiration_interval"),
			UsageRawRetention:             v.GetDuration("usage_raw_retention"),
			UsageRawCleanupInterval:       v.GetDuration("usage_raw_cleanup_interval"),
			SubscriptionQuotaSyncInterval: v.GetDuration("subscription_quota_sync_interval"),
		},
		ProxyRuntimeConfig: ProxyRuntimeConfig{
			ProxyReloadDebounce: v.GetDuration("proxy_reload_debounce"),
		},
	}

	if cfg.DatabaseURL == "" {
		return ScheduleConfig{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return ScheduleConfig{}, errors.New("PROMPTGATE_JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return ScheduleConfig{}, errors.New("PROMPTGATE_JWT_SECRET must be at least 32 characters")
	}
	if cfg.SecretsKey == "" {
		return ScheduleConfig{}, errors.New("PROMPTGATE_SECRETS_KEY is required")
	}
	if cfg.RedisURL == "" {
		return ScheduleConfig{}, errors.New("PROMPTGATE_REDIS_URL is required")
	}
	if err := validateOptionalFile("PROMPTGATE_CA_FILE", cfg.CAFile); err != nil {
		return ScheduleConfig{}, err
	}
	if cfg.UsageRawRetention <= 0 {
		return ScheduleConfig{}, errors.New("PROMPTGATE_USAGE_RAW_RETENTION must be greater than zero")
	}
	if cfg.UsageRawCleanupInterval <= 0 {
		return ScheduleConfig{}, errors.New("PROMPTGATE_USAGE_RAW_CLEANUP_INTERVAL must be greater than zero")
	}
	if cfg.SubscriptionQuotaSyncInterval <= 0 {
		return ScheduleConfig{}, errors.New("PROMPTGATE_SUBSCRIPTION_QUOTA_SYNC_INTERVAL must be greater than zero")
	}
	if err := validatePositiveDurations(
		positiveDuration{"PROMPTGATE_TOKEN_CLEANUP_INTERVAL", cfg.TokenCleanupInterval},
		positiveDuration{"PROMPTGATE_USER_ACCESS_EXPIRATION_INTERVAL", cfg.UserAccessExpirationInterval},
		positiveDuration{"PROMPTGATE_REDIS_CACHE_TTL", cfg.RedisCacheTTL},
		positiveDuration{"PROMPTGATE_PROXY_RELOAD_DEBOUNCE", cfg.ProxyReloadDebounce},
	); err != nil {
		return ScheduleConfig{}, err
	}

	return cfg, nil
}
