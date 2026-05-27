package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newUsersTestHandler creates an admin handler wired to an in-memory users service.
func newUsersTestHandler(t *testing.T) (*Handler, *users.Service) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	service := users.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate users table: %v", err)
	}
	createAdminUsersUsageTables(t, db)

	return NewHandler(service, nil, nil, nil, nil), service
}

func createAdminUsersUsageTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE interceptions (
			id text PRIMARY KEY,
			initiator_id text NOT NULL
		)`,
		`CREATE TABLE token_usages (
			id text PRIMARY KEY,
			interception_id text NOT NULL,
			input_tokens integer NOT NULL DEFAULT 0,
			output_tokens integer NOT NULL DEFAULT 0
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create usage table: %v", err)
		}
	}
}

func TestHandleAdminUpdateUserStoresExpiration(t *testing.T) {
	handler, service := newUsersTestHandler(t)
	ctx := context.Background()

	created, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-1",
		PreferredUsername: "promptgate-user",
		Email:             "user@example.com",
		Name:              "PromptGate User",
	})
	if err != nil {
		t.Fatalf("sync user: %v", err)
	}

	expiresAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	body, err := json.Marshal(map[string]any{
		"role":      auth.RoleUser,
		"isActive":  true,
		"expiresAt": expiresAt.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/users/"+created.ID,
		bytes.NewReader(body),
	)
	req.SetPathValue("id", created.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminUpdateUser(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	updated, err := service.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expiration %s, got %v", expiresAt, updated.ExpiresAt)
	}
}
