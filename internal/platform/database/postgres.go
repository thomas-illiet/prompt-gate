package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	defaultConnMaxLifetime = 30 * time.Minute
	defaultMaxIdleConns    = 5
	defaultMaxOpenConns    = 10
)

// PoolOptions controls the PostgreSQL connection pool. Zero values retain the
// application defaults so existing callers do not need to specify options.
type PoolOptions struct {
	ConnMaxLifetime time.Duration
	MaxIdleConns    int
	MaxOpenConns    int
}

func resolvePoolOptions(configured PoolOptions) PoolOptions {
	options := PoolOptions{
		ConnMaxLifetime: defaultConnMaxLifetime,
		MaxIdleConns:    defaultMaxIdleConns,
		MaxOpenConns:    defaultMaxOpenConns,
	}
	if configured.ConnMaxLifetime > 0 {
		options.ConnMaxLifetime = configured.ConnMaxLifetime
	}
	if configured.MaxIdleConns > 0 {
		options.MaxIdleConns = configured.MaxIdleConns
	}
	if configured.MaxOpenConns > 0 {
		options.MaxOpenConns = configured.MaxOpenConns
	}
	return options
}

// OpenPostgres opens and verifies a PostgreSQL connection with sensible pool settings.
func OpenPostgres(ctx context.Context, databaseURL string, poolOptions ...PoolOptions) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("expose sql db: %w", err)
	}

	configured := PoolOptions{}
	if len(poolOptions) > 0 {
		configured = poolOptions[0]
	}
	options := resolvePoolOptions(configured)

	sqlDB.SetConnMaxLifetime(options.ConnMaxLifetime)
	sqlDB.SetMaxIdleConns(options.MaxIdleConns)
	sqlDB.SetMaxOpenConns(options.MaxOpenConns)

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}
