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

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAdminTokenTestHandler(t *testing.T) (*Handler, *users.Service, *tokens.Service) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	userService := users.NewService(db)
	tokenService := tokens.NewService(db, "test-secret")
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	if err := tokenService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}
	createAdminUsersUsageTables(t, db)
	return NewHandler(Dependencies{Users: userService, Tokens: tokenService}), userService, tokenService
}

func TestHandleAdminCreateUserTokenAllowsInactiveNoneUser(t *testing.T) {
	handler, userService, tokenService := newAdminTokenTestHandler(t)
	ctx := context.Background()
	if _, err := userService.SyncUser(ctx, auth.Identity{Sub: "admin", PreferredUsername: "admin"}); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	user, err := userService.SyncUser(ctx, auth.Identity{Sub: "limited", PreferredUsername: "limited"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := userService.UpdateUser(ctx, user.ID, users.UpdateUserInput{Role: auth.RoleNone, IsActive: false}); err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+user.ID+"/tokens", bytes.NewBufferString(`{"name":"admin_created","description":"managed"}`))
	req.SetPathValue("id", user.ID)
	recorder := httptest.NewRecorder()
	handler.HandleAdminCreateUserToken(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response tokens.CreateTokenResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Token == "" || response.TokenInfo.UserID != user.ID {
		t.Fatalf("unexpected token response: %#v", response)
	}
	if _, err := tokenService.ValidateToken(ctx, response.Token, userService); err != tokens.ErrAccountInactive {
		t.Fatalf("expected inactive token rejection, got %v", err)
	}
	if _, err := userService.UpdateUser(ctx, user.ID, users.UpdateUserInput{Role: auth.RoleLimited, IsActive: true}); err != nil {
		t.Fatalf("activate limited user: %v", err)
	}
	if _, err := tokenService.ValidateToken(ctx, response.Token, userService); err != nil {
		t.Fatalf("expected token to become valid after activation, got %v", err)
	}
}

func TestHandleAdminCreateUserTokenRejectsServiceAccount(t *testing.T) {
	handler, userService, _ := newAdminTokenTestHandler(t)
	account, err := userService.CreateServiceAccount(context.Background(), users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+account.ID+"/tokens", bytes.NewBufferString(`{"name":"wrong_route"}`))
	req.SetPathValue("id", account.ID)
	recorder := httptest.NewRecorder()
	handler.HandleAdminCreateUserToken(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
