package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// OpenPostgres opens and verifies a PostgreSQL connection with sensible pool settings.
func OpenPostgres(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("expose sql db: %w", err)
	}

	sqlDB.SetConnMaxLifetime(envDuration("PROMPTGATE_DB_CONN_MAX_LIFETIME", 30*time.Minute))
	sqlDB.SetMaxIdleConns(envInt("PROMPTGATE_DB_MAX_IDLE_CONNS", 5))
	sqlDB.SetMaxOpenConns(envInt("PROMPTGATE_DB_MAX_OPEN_CONNS", 10))

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
