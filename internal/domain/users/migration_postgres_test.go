package users

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/database"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSyncUserPostgresBootstrapsExactlyOneAdminConcurrently(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("PROMPTGATE_TEST_POSTGRES_URL"))
	if databaseURL == "" {
		t.Skip("PROMPTGATE_TEST_POSTGRES_URL is not set")
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil || parsedURL.Scheme == "" {
		t.Fatalf("PROMPTGATE_TEST_POSTGRES_URL must be a PostgreSQL URL: %v", err)
	}
	baseDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL integration database: %v", err)
	}
	baseSQL, err := baseDB.DB()
	if err != nil {
		t.Fatalf("expose PostgreSQL integration database: %v", err)
	}
	defer baseSQL.Close()

	schema := fmt.Sprintf("users_bootstrap_%d", time.Now().UTC().UnixNano())
	if err := baseDB.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create PostgreSQL integration schema: %v", err)
	}
	defer func() {
		if err := baseDB.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop PostgreSQL integration schema: %v", err)
		}
	}()

	query := parsedURL.Query()
	query.Set("search_path", schema)
	parsedURL.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsedURL.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL integration schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("expose isolated PostgreSQL integration schema: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(8)

	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate PostgreSQL users: %v", err)
	}

	const concurrentLogins = 8
	start := make(chan struct{})
	profiles := make(chan auth.UserProfile, concurrentLogins)
	errorsCh := make(chan error, concurrentLogins)
	var waitGroup sync.WaitGroup
	for index := range concurrentLogins {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			profile, syncErr := service.SyncUser(context.Background(), auth.Identity{
				Sub:               fmt.Sprintf("bootstrap-subject-%d", index),
				PreferredUsername: fmt.Sprintf("bootstrap-user-%d", index),
			})
			if syncErr != nil {
				errorsCh <- syncErr
				return
			}
			profiles <- profile
		}()
	}
	close(start)
	waitGroup.Wait()
	close(profiles)
	close(errorsCh)

	for syncErr := range errorsCh {
		t.Errorf("concurrent PostgreSQL user sync: %v", syncErr)
	}
	var admins, limited int
	for profile := range profiles {
		switch profile.Role {
		case auth.RoleAdmin:
			admins++
		case auth.RoleLimited:
			limited++
		default:
			t.Errorf("unexpected bootstrap role %q", profile.Role)
		}
	}
	if admins != 1 || limited != concurrentLogins-1 {
		t.Fatalf("expected one admin and %d limited users, got admins=%d limited=%d", concurrentLogins-1, admins, limited)
	}
}

// TestUserMigrationPostgresMakesSSOFieldsNullable validates the production
// NOT NULL migration and composite identity constraint on PostgreSQL.
func TestUserMigrationPostgresMakesSSOFieldsNullable(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("PROMPTGATE_TEST_POSTGRES_URL"))
	if databaseURL == "" {
		t.Skip("PROMPTGATE_TEST_POSTGRES_URL is not set")
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil || parsedURL.Scheme == "" {
		t.Fatalf("PROMPTGATE_TEST_POSTGRES_URL must be a PostgreSQL URL: %v", err)
	}
	baseDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL integration database: %v", err)
	}
	baseSQL, err := baseDB.DB()
	if err != nil {
		t.Fatalf("expose PostgreSQL integration database: %v", err)
	}
	defer baseSQL.Close()

	schema := fmt.Sprintf("users_migration_%d", time.Now().UTC().UnixNano())
	if err := baseDB.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create PostgreSQL integration schema: %v", err)
	}
	defer func() {
		if err := baseDB.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop PostgreSQL integration schema: %v", err)
		}
	}()

	query := parsedURL.Query()
	query.Set("search_path", schema)
	parsedURL.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsedURL.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL integration schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("expose isolated PostgreSQL integration schema: %v", err)
	}
	defer sqlDB.Close()

	if err := db.Exec(`CREATE TABLE users (
		id uuid PRIMARY KEY,
		external_sub text NOT NULL,
		email text NOT NULL DEFAULT '',
		preferred_username text NOT NULL,
		name text NOT NULL DEFAULT '',
		type varchar(16) NOT NULL DEFAULT 'user',
		role varchar(16) NOT NULL DEFAULT 'none',
		note text NOT NULL DEFAULT '',
		is_active boolean NOT NULL DEFAULT true,
		last_login_at timestamptz NOT NULL,
		created_at timestamptz,
		updated_at timestamptz
	)`).Error; err != nil {
		t.Fatalf("create legacy PostgreSQL users table: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_users_external_sub ON users(external_sub)`).Error; err != nil {
		t.Fatalf("create legacy PostgreSQL subject index: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO users (id, external_sub, preferred_username, last_login_at) VALUES (?, ?, ?, ?)`,
		"11111111-1111-1111-1111-111111111111",
		"legacy-sub",
		"legacy-user",
		time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("seed legacy PostgreSQL user: %v", err)
	}

	if err := NewService(db).AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate PostgreSQL users: %v", err)
	}

	var nullableColumns int64
	if err := db.Raw(`SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'users'
		  AND column_name IN ('external_sub', 'last_login_at')
		  AND is_nullable = 'YES'`).Scan(&nullableColumns).Error; err != nil {
		t.Fatalf("inspect PostgreSQL nullability: %v", err)
	}
	if nullableColumns != 2 {
		t.Fatalf("expected both SSO columns nullable, got %d", nullableColumns)
	}

	pendingRows := []User{
		{
			ID:                "22222222-2222-2222-2222-222222222222",
			PreferredUsername: "shared-identifier",
			Type:              auth.UserTypeUser,
			Role:              auth.RoleNone,
			IsActive:          true,
		},
		{
			ID:                "33333333-3333-3333-3333-333333333333",
			PreferredUsername: "other-user",
			Type:              auth.UserTypeUser,
			Role:              auth.RoleNone,
			IsActive:          true,
		},
		{
			ID:                "44444444-4444-4444-4444-444444444444",
			PreferredUsername: "shared-identifier",
			Type:              auth.UserTypeService,
			Role:              auth.RoleUser,
			IsActive:          true,
		},
	}
	if err := db.Create(&pendingRows).Error; err != nil {
		t.Fatalf("create multiple PostgreSQL NULL subjects: %v", err)
	}

	duplicateHuman := pendingRows[0]
	duplicateHuman.ID = "55555555-5555-5555-5555-555555555555"
	if err := db.Create(&duplicateHuman).Error; !database.IsUniqueViolation(err) {
		t.Fatalf("expected composite identity uniqueness violation, got %v", err)
	}
}
