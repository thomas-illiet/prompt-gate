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
	tokenDomain "promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newDashboardTestHandler(t *testing.T) (*Handler, *gorm.DB) {
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
	if err := db.AutoMigrate(&tokenDomain.Token{}); err != nil {
		t.Fatalf("auto-migrate tokens table: %v", err)
	}

	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	for _, user := range []users.User{
		{
			ID:                "11111111-1111-1111-1111-111111111111",
			ExternalSub:       "sub-1",
			Email:             "one@example.com",
			PreferredUsername: "one",
			Name:              "One",
			Type:              auth.UserTypeUser,
			Role:              auth.RoleUser,
			IsActive:          true,
			LastLoginAt:       now,
		},
		{
			ID:                "22222222-2222-2222-2222-222222222222",
			ExternalSub:       "service:worker",
			PreferredUsername: "worker",
			Name:              "Worker",
			Type:              auth.UserTypeService,
			Role:              auth.RoleUser,
			IsActive:          true,
			LastLoginAt:       now,
		},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	return NewHandler(userService, nil, nil, nil, nil, nil, proxy.NewService(db)), db
}

func seedDashboardUsage(t *testing.T, db *gorm.DB, userID string, id string, at time.Time, inputTokens int64, outputTokens int64) {
	t.Helper()
	if err := db.Create(&proxy.Interception{
		ID:           id,
		InitiatorID:  userID,
		Provider:     "openai",
		ProviderType: "openai",
		Model:        "gpt-5",
		StartedAt:    at,
		Metadata:     "{}",
	}).Error; err != nil {
		t.Fatalf("seed interception: %v", err)
	}
	if err := db.Create(&proxy.TokenUsage{
		InterceptionID:     id,
		ProviderResponseID: "response-" + id,
		InputTokens:        inputTokens,
		OutputTokens:       outputTokens,
		Metadata:           "{}",
		CreatedAt:          at.Add(time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}
}

func TestHandleAdminDashboardTokensReturnsGlobalResult(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	now := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", now, 10, 20)
	seedDashboardUsage(t, db, "22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", now, 30, 40)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/tokens?window=7d", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminDashboardTokens(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardTokensResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TotalTokens != 100 {
		t.Fatalf("expected global token total, got %#v", body)
	}
}

func TestHandleAdminDashboardAdoptionReturnsResult(t *testing.T) {
	handler, db := newDashboardTestHandler(t)
	now := time.Now().UTC().Add(-time.Hour)
	seedDashboardUsage(t, db, "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", now, 10, 20)
	seedDashboardUsage(t, db, "22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", now, 30, 40)
	if err := db.Create(&tokenDomain.Token{
		UserID:      "11111111-1111-1111-1111-111111111111",
		Name:        "active",
		TokenHash:   "active-hash",
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		Description: "",
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/adoption?window=7d", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminDashboardAdoption(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body proxy.DashboardAdoptionResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.ActiveUsers != 1 || body.ActiveServiceAccounts != 1 || body.ActiveVirtualKeys != 1 {
		t.Fatalf("unexpected adoption response: %#v", body)
	}
}

func TestHandleAdminDashboardRejectsInvalidWindow(t *testing.T) {
	handler, _ := newDashboardTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/tokens?window=14d", nil)
	recorder := httptest.NewRecorder()

	handler.HandleAdminDashboardTokens(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
