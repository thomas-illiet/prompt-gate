package database

import (
	"context"
	"errors"
	"testing"

	"promptgate/backend/internal/platform/testconfig"
)

func TestPostgresPoolLifecycleAndCancellation(t *testing.T) {
	ctx := context.Background()
	endpoint := testconfig.PostgresURL(t)
	db, err := OpenPostgres(ctx, endpoint, PoolOptions{MaxOpenConns: 2, MaxIdleConns: 1})
	if err != nil {
		testconfig.Unavailable(t, "PostgreSQL", err)
	}
	defer Close(db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB.Stats().MaxOpenConnections != 2 {
		t.Fatal("configured connection bound ignored")
	}
	if err := db.Exec("SELECT 1").Error; err != nil {
		t.Fatal(err)
	}
	if err := Close(db); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.PingContext(ctx); err == nil {
		t.Fatal("database pool still open after Close")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := OpenPostgres(canceled, endpoint); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
	if _, err := OpenPostgres(ctx, "postgres://%bad"); err == nil {
		t.Fatal("invalid database URL accepted")
	}
	if err := Close(nil); err != nil {
		t.Fatal(err)
	}
}
