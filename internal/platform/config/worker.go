package config

import (
	"errors"
	"strings"

	"github.com/spf13/viper"
)

// LoadWorker reads configuration required to run the generic background worker.
func LoadWorker() (WorkerConfig, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("log_level", "info")
	v.SetDefault("redis_cache_ttl", "5m")
	v.SetDefault("worker_batch_size", 100)
	v.SetDefault("worker_block_timeout", "5s")
	v.SetDefault("worker_pending_idle_timeout", "30s")

	cfg := WorkerConfig{
		LogConfig:         loadLogConfig(v),
		DatabaseURLConfig: loadDatabaseURLConfig(v),
		RedisConfig:       loadRedisConfig(v),
		WorkerRuntimeConfig: WorkerRuntimeConfig{
			WorkerBatchSize:          v.GetInt64("worker_batch_size"),
			WorkerBlockTimeout:       v.GetDuration("worker_block_timeout"),
			WorkerPendingIdleTimeout: v.GetDuration("worker_pending_idle_timeout"),
			WorkerConsumerName:       strings.TrimSpace(v.GetString("worker_consumer_name")),
		},
	}

	if cfg.DatabaseURL == "" {
		return WorkerConfig{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return WorkerConfig{}, errors.New("PROMPTGATE_REDIS_URL is required")
	}
	if cfg.WorkerBatchSize <= 0 {
		return WorkerConfig{}, errors.New("PROMPTGATE_WORKER_BATCH_SIZE must be greater than zero")
	}
	if cfg.WorkerBlockTimeout <= 0 {
		return WorkerConfig{}, errors.New("PROMPTGATE_WORKER_BLOCK_TIMEOUT must be greater than zero")
	}
	if cfg.WorkerPendingIdleTimeout <= 0 {
		return WorkerConfig{}, errors.New("PROMPTGATE_WORKER_PENDING_IDLE_TIMEOUT must be greater than zero")
	}
	if err := validatePositiveDurations(
		positiveDuration{"PROMPTGATE_REDIS_CACHE_TTL", cfg.RedisCacheTTL},
	); err != nil {
		return WorkerConfig{}, err
	}

	return cfg, nil
}
