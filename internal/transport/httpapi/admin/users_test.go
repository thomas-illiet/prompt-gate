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

	return NewHandler(service, nil, nil, nil, nil, nil), service
}

// createAdminUsersUsageTables creates admin users usage tables.
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

// TestHandleAdminUpdateUserStoresExpiration verifies handle admin update user stores expiration.
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

// TestHandleAdminUpdateUserNoteStoresNote verifies handle admin update user note stores note.
func TestHandleAdminUpdateUserNoteStoresNote(t *testing.T) {
	handler, service := newUsersTestHandler(t)
	ctx := context.Background()

	created, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-note",
		PreferredUsername: "note-user",
		Email:             "note@example.com",
		Name:              "Note User",
	})
	if err != nil {
		t.Fatalf("sync user: %v", err)
	}

	body, err := json.Marshal(map[string]string{
		"note": "Follow up with security before renewal.",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/users/"+created.ID+"/note",
		bytes.NewReader(body),
	)
	req.SetPathValue("id", created.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminUpdateUserNote(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response users.AdminUser
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Note != "Follow up with security before renewal." {
		t.Fatalf("expected response note, got %q", response.Note)
	}

	updated, err := service.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.Note != response.Note {
		t.Fatalf("expected persisted note %q, got %q", response.Note, updated.Note)
	}
}

// TestHandleAdminUpdateUserNoteErrors verifies handle admin update user note errors.
func TestHandleAdminUpdateUserNoteErrors(t *testing.T) {
	handler, service := newUsersTestHandler(t)
	ctx := context.Background()

	created, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-note-errors",
		PreferredUsername: "note-errors",
		Email:             "note-errors@example.com",
		Name:              "Note Errors",
	})
	if err != nil {
		t.Fatalf("sync user: %v", err)
	}

	longBody, err := json.Marshal(map[string]string{
		"note": strings.Repeat("a", 2001),
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	invalidReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/users/"+created.ID+"/note",
		bytes.NewReader(longBody),
	)
	invalidReq.SetPathValue("id", created.ID)
	invalidRecorder := httptest.NewRecorder()

	handler.HandleAdminUpdateUserNote(invalidRecorder, invalidReq)

	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
	if !strings.Contains(invalidRecorder.Body.String(), "invalid_note") {
		t.Fatalf("expected invalid_note response, got %s", invalidRecorder.Body.String())
	}

	missingReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/users/missing-user/note",
		bytes.NewBufferString(`{"note":"hello"}`),
	)
	missingReq.SetPathValue("id", "missing-user")
	missingRecorder := httptest.NewRecorder()

	handler.HandleAdminUpdateUserNote(missingRecorder, missingReq)

	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", missingRecorder.Code, missingRecorder.Body.String())
	}
	if !strings.Contains(missingRecorder.Body.String(), "user_not_found") {
		t.Fatalf("expected user_not_found response, got %s", missingRecorder.Body.String())
	}
}
