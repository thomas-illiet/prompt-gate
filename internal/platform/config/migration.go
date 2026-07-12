package config

import (
	"errors"
	"github.com/spf13/viper"
)

// LoadMigration reads the configuration required to run database migrations.
func LoadMigration() (MigrationConfig, error) {
	v := viper.New()
	v.SetEnvPrefix("PROMPTGATE")
	v.AutomaticEnv()

	v.SetDefault("log_level", "info")

	cfg := MigrationConfig{
		LogConfig:         loadLogConfig(v),
		DatabaseURLConfig: loadDatabaseURLConfig(v),
	}

	if cfg.DatabaseURL == "" {
		return MigrationConfig{}, errors.New("PROMPTGATE_DATABASE_URL is required")
	}

	return cfg, nil
}
