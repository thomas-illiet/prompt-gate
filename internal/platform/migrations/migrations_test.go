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
