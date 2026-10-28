package config

import (
	"errors"
	"time"
)

// LoadSchedule reads the scheduler configuration from a strict YAML file.
func LoadSchedule(configPath string) (ScheduleConfig, error) {
	file, err := loadFile(configPath)
	if err != nil {
		return ScheduleConfig{}, err
	}
	usageCost, err := file.usageCostConfig()
	if err != nil {
		return ScheduleConfig{}, err
	}
	cfg := ScheduleConfig{
		LogConfig:                  file.logConfig(),
		DatabaseURLConfig:          file.databaseConfig(),
		TLSConfig:                  file.tlsConfig(),
		RedisConfig:                file.redisConfig(),
		SecretsConfig:              file.secretsConfig(),
		PublicURLConfig:            file.publicURLConfig(),
		NotificationConfig:         notificationConfig(file),
		NotificationScheduleConfig: notificationScheduleConfig(file),
		UsageCost:                  usageCost,
		ScheduleIntervals: ScheduleIntervals{
			TokenCleanupInterval:           time.Duration(file.Common.Maintenance.TokenCleanupInterval),
			UserAccessExpirationInterval:   time.Duration(file.Common.Maintenance.UserAccessExpirationInterval),
			UsageRawRetention:              time.Duration(file.Schedule.UsageRawRetention),
			UsageRawCleanupInterval:        time.Duration(file.Schedule.UsageRawCleanupInterval),
			SubscriptionQuotaSyncInterval:  time.Duration(file.Schedule.SubscriptionQuotaSyncInterval),
			QuotaResetDistributionInterval: time.Duration(file.Common.Maintenance.QuotaResetDistributionInterval),
		},
		ProxyRuntimeConfig: ProxyRuntimeConfig{ProxyReloadDebounce: time.Duration(file.Common.ProxyReloadDebounce)},
	}
	if err := validateDatabaseConfig(cfg.DatabaseURLConfig); err != nil {
		return ScheduleConfig{}, err
	}
	if err := validateRedisConfig(cfg.RedisConfig); err != nil {
		return ScheduleConfig{}, err
	}
	if err := validateRedisOverrides(file.Common.Redis); err != nil {
		return ScheduleConfig{}, err
	}
	if cfg.JWTSecret == "" {
		return ScheduleConfig{}, errors.New("common.security.jwt_secret is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return ScheduleConfig{}, errors.New("common.security.jwt_secret must be at least 32 characters")
	}
	if cfg.SecretsKey == "" {
		return ScheduleConfig{}, errors.New("common.security.secrets_key is required")
	}
	if err := validateOptionalFile("common.tls.ca_file", cfg.CAFile); err != nil {
		return ScheduleConfig{}, err
	}
	if err := validateNotificationConfig(cfg.NotificationConfig); err != nil {
		return ScheduleConfig{}, err
	}
	if cfg.NotificationsEnabled {
		if err := validatePositiveDurations(
			positiveDuration{"schedule.notification_cleanup_interval", cfg.NotificationCleanupInterval},
			positiveDuration{"schedule.notification_retention", cfg.NotificationRetention},
			positiveDuration{"schedule.quota_notification_interval", cfg.QuotaNotificationInterval},
		); err != nil {
			return ScheduleConfig{}, err
		}
	}
	if err := validatePositiveDurations(
		positiveDuration{"common.maintenance.token_cleanup_interval", cfg.TokenCleanupInterval},
		positiveDuration{"common.maintenance.user_access_expiration_interval", cfg.UserAccessExpirationInterval},
		positiveDuration{"schedule.usage_raw_retention", cfg.UsageRawRetention},
		positiveDuration{"schedule.usage_raw_cleanup_interval", cfg.UsageRawCleanupInterval},
		positiveDuration{"schedule.subscription_quota_sync_interval", cfg.SubscriptionQuotaSyncInterval},
		positiveDuration{"common.maintenance.quota_reset_distribution_interval", cfg.QuotaResetDistributionInterval},
		positiveDuration{"common.proxy_reload_debounce", cfg.ProxyReloadDebounce},
	); err != nil {
		return ScheduleConfig{}, err
	}
	return cfg, nil
}
