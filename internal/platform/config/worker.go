package config

import (
	"errors"
	"strings"
	"time"
)

// LoadWorker reads the background worker configuration from a strict YAML file.
func LoadWorker(configPath string) (WorkerConfig, error) {
	file, err := loadFile(configPath)
	if err != nil {
		return WorkerConfig{}, err
	}
	cfg := WorkerConfig{
		LogConfig:         file.logConfig(),
		DatabaseURLConfig: file.databaseConfig(),
		RedisConfig:       file.redisConfig(),
		WorkerRuntimeConfig: WorkerRuntimeConfig{
			WorkerBatchSize:          file.Worker.BatchSize,
			WorkerBlockTimeout:       time.Duration(file.Worker.BlockTimeout),
			WorkerPendingIdleTimeout: time.Duration(file.Worker.PendingIdleTimeout),
			WorkerConsumerName:       strings.TrimSpace(file.Worker.ConsumerName),
		},
	}
	if err := validateDatabaseConfig(cfg.DatabaseURLConfig); err != nil {
		return WorkerConfig{}, err
	}
	if err := validateRedisConfig(cfg.RedisConfig); err != nil {
		return WorkerConfig{}, err
	}
	if err := validateRedisOverrides(file.Common.Redis); err != nil {
		return WorkerConfig{}, err
	}
	if cfg.WorkerBatchSize <= 0 {
		return WorkerConfig{}, errors.New("worker.batch_size must be greater than zero")
	}
	if cfg.WorkerBlockTimeout <= 0 {
		return WorkerConfig{}, errors.New("worker.block_timeout must be greater than zero")
	}
	if cfg.WorkerPendingIdleTimeout <= 0 {
		return WorkerConfig{}, errors.New("worker.pending_idle_timeout must be greater than zero")
	}
	return cfg, nil
}
