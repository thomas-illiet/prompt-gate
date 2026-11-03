package migrations

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"promptgate/backend/internal/platform/testconfig"
)

func migrationPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	endpoint := testconfig.PostgresURL(t)
	base, err := gorm.Open(postgres.Open(endpoint), &gorm.Config{})
	if err != nil {
		testconfig.Unavailable(t, "PostgreSQL", err)
	}
	sqlBase, _ := base.DB()
	t.Cleanup(func() { _ = sqlBase.Close() })
	schema := fmt.Sprintf("migrations_%d", time.Now().UnixNano())
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	parsed, _ := url.Parse(endpoint)
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestPostgresMigrationSerializesAndDoesNotReplayBaseline(t *testing.T) {
	db := migrationPostgres(t)
	ctx := context.Background()
	// A pre-ledger installation must be adopted without losing its data.
	if err := migrateBaseline(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE release_sentinel (value text); INSERT INTO release_sentinel VALUES ('preserved')`).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- Run(ctx, db) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(&schemaMigration{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("migration count %d: %v", count, err)
	}
	// Recreating a retired table makes accidental replay observable.
	if err := db.Exec("CREATE TABLE mcp_servers (id text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable("mcp_servers") {
		t.Fatal("completed migration replayed")
	}
	var sentinel string
	if err := db.Table("release_sentinel").Select("value").Scan(&sentinel).Error; err != nil || sentinel != "preserved" {
		t.Fatalf("data changed: %q %v", sentinel, err)
	}
}

func TestPostgresFailedMigrationRollsBackAndCanRetry(t *testing.T) {
	db := migrationPostgres(t)
	ctx := context.Background()
	// A conflicting relation fails the baseline after some schema work has run.
	if err := db.Exec("CREATE VIEW subscription_plans AS SELECT 1 AS id").Error; err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, db); err == nil {
		t.Fatal("expected migration failure")
	}
	if db.Migrator().HasTable("schema_migrations") || db.Migrator().HasTable("users") {
		t.Fatal("failed migration committed partial DDL or journal")
	}
	if err := db.Exec("DROP VIEW subscription_plans").Error; err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, db); err != nil {
		t.Fatalf("retry: %v", err)
	}
}
