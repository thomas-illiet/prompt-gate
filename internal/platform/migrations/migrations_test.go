package migrations

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRemoveLegacyMCPDropsTableIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE mcp_servers (id TEXT PRIMARY KEY, headers TEXT)`).Error; err != nil {
		t.Fatalf("create legacy mcp table: %v", err)
	}

	ctx := context.Background()
	if err := removeLegacyMCP(ctx, db); err != nil {
		t.Fatalf("remove legacy mcp schema: %v", err)
	}
	if db.Migrator().HasTable("mcp_servers") {
		t.Fatal("legacy mcp table still exists")
	}
	if err := removeLegacyMCP(ctx, db); err != nil {
		t.Fatalf("remove absent legacy mcp schema: %v", err)
	}
}

func TestUserIPValidationBypassMigrationUpgradesVersionOne(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schemaMigration{Version: 1, Name: "release_baseline"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE users (id text PRIMARY KEY, type text NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO users (id, type) VALUES ('existing-user', 'user'), ('existing-service', 'service')").Error; err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := Run(context.Background(), db); err != nil {
			t.Fatal(err)
		}
	}
	var enabled int64
	if err := db.Table("users").Where("bypass_ip_validation <> ? OR bypass_ip_validation IS NULL", false).Count(&enabled).Error; err != nil || enabled != 0 {
		t.Fatalf("existing accounts must default to false: count=%d err=%v", enabled, err)
	}
	var applied int64
	if err := db.Model(&schemaMigration{}).Count(&applied).Error; err != nil || applied != 2 {
		t.Fatalf("expected both migrations recorded: count=%d err=%v", applied, err)
	}
}
