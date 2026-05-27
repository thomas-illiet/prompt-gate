package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPromptsTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	userService := users.NewService(db)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate users table: %v", err)
	}
	if err := proxy.AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("auto-migrate proxy tables: %v", err)
	}

	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	if err := db.Create(&users.User{
		ID:                "11111111-1111-1111-1111-111111111111",
		ExternalSub:       "sub-1",
		Email:             "one@example.com",
		PreferredUsername: "one",
		Name:              "One",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return NewHandler(userService, nil, nil, nil, nil, proxy.NewService(db)), db
}

func seedAdminPrompt(t *testing.T, db *gorm.DB) {
	t.Helper()
	at := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	interceptionID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	if err := db.Create(&proxy.Interception{
		ID:           interceptionID,
		InitiatorID:  "11111111-1111-1111-1111-111111111111",
		Provider:     "openai",
		ProviderType: "openai",
		Model:        "gpt-5",
		StartedAt:    at,
		Metadata:     "{}",
	}).Error; err != nil {
		t.Fatalf("seed interception: %v", err)
	}
	if err := db.Create(&proxy.UserPrompt{
		InterceptionID:     interceptionID,
		ProviderResponseID: "response-" + interceptionID,
		Prompt:             "Alpha prompt",
		Metadata:           "{}",
		CreatedAt:          at.Add(time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed prompt: %v", err)
	}
}

func TestHandleAdminListPromptsReturnsResult(t *testing.T) {
	handler, db := newPromptsTestHandler(t)
	seedAdminPrompt(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/prompts?search=alpha", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminListPrompts(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var body proxy.AdminPromptListResult
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Total != 1 || body.Items[0].Prompt != "Alpha prompt" || body.Items[0].UserEmail != "one@example.com" {
		t.Fatalf("unexpected admin prompt response: %#v", body)
	}
}

func TestHandleAdminListPromptsRejectsInvalidSort(t *testing.T) {
	handler, _ := newPromptsTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/prompts?sortBy=unknown", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminListPrompts(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "invalid_sort" {
		t.Fatalf("expected invalid_sort, got %#v", body)
	}
}

func TestHandleAdminListPromptsRequiresProxyService(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/prompts", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminListPrompts(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
