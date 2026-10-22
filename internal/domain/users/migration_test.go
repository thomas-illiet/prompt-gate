package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	return db
}

func createLegacyUsersTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE users (
		id text PRIMARY KEY,
		external_sub text NOT NULL,
		email text NOT NULL DEFAULT '',
		preferred_username text NOT NULL,
		name text NOT NULL DEFAULT '',
		type text NOT NULL DEFAULT 'user',
		role text NOT NULL DEFAULT 'none',
		note text NOT NULL DEFAULT '',
		is_active numeric NOT NULL DEFAULT 1,
		last_login_at datetime NOT NULL,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_users_external_sub ON users(external_sub)`).Error; err != nil {
		t.Fatalf("create legacy subject index: %v", err)
	}
}

func TestAutoMigrateRejectsLegacyPreferredUsernameDuplicates(t *testing.T) {
	db := openMigrationTestDB(t)
	createLegacyUsersTable(t, db)
	now := time.Now().UTC()
	for _, id := range []string{"one", "two"} {
		if err := db.Exec(
			`INSERT INTO users (id, external_sub, preferred_username, type, last_login_at) VALUES (?, ?, ?, ?, ?)`,
			id,
			"sub-"+id,
			"duplicate",
			auth.UserTypeUser,
			now,
		).Error; err != nil {
			t.Fatalf("seed legacy duplicate %s: %v", id, err)
		}
	}

	err := NewService(db).AutoMigrate(context.Background())
	if !errors.Is(err, ErrDuplicatePreferredUsername) {
		t.Fatalf("expected explicit duplicate preflight error, got %v", err)
	}
	if !strings.Contains(err.Error(), `preferred_username="duplicate"`) {
		t.Fatalf("expected duplicate identity details, got %v", err)
	}
}

func TestAutoMigrateMakesSSOFieldsNullableAndAddsCompositeIdentityIndex(t *testing.T) {
	db := openMigrationTestDB(t)
	createLegacyUsersTable(t, db)
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO users (id, external_sub, preferred_username, type, last_login_at) VALUES (?, ?, ?, ?, ?)`,
		"legacy",
		"legacy-sub",
		"legacy-user",
		auth.UserTypeUser,
		now,
	).Error; err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}

	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate legacy users: %v", err)
	}
	if !db.Migrator().HasIndex(&User{}, "idx_users_type_preferred_username") {
		t.Fatal("expected composite type/preferred_username index")
	}

	nullability := map[string]bool{}
	columns, err := db.Migrator().ColumnTypes(&User{})
	if err != nil {
		t.Fatalf("load migrated column types: %v", err)
	}
	for _, column := range columns {
		if nullable, ok := column.Nullable(); ok {
			nullability[column.Name()] = nullable
		}
	}
	if !nullability["external_sub"] || !nullability["last_login_at"] {
		t.Fatalf("expected nullable SSO columns, got %#v", nullability)
	}

	pendingHuman := User{
		ID:                "pending-human",
		PreferredUsername: "shared-identifier",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleNone,
		IsActive:          true,
	}
	pendingService := User{
		ID:                "pending-service",
		PreferredUsername: "shared-identifier",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
	}
	if err := db.Create(&pendingHuman).Error; err != nil {
		t.Fatalf("create pending human: %v", err)
	}
	if err := db.Create(&pendingService).Error; err != nil {
		t.Fatalf("create same-identifier service account: %v", err)
	}
	duplicateHuman := pendingHuman
	duplicateHuman.ID = "duplicate-human"
	if err := db.Create(&duplicateHuman).Error; err == nil {
		t.Fatal("expected duplicate human identity to violate composite index")
	}
}
