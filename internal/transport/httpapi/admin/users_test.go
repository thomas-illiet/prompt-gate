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
	"promptgate/backend/internal/domain/firewall"
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

	return NewHandler(Dependencies{Users: service}), service
}

// newUsersFirewallTestHandler creates an admin handler with users and firewall services.
func newUsersFirewallTestHandler(t *testing.T) (*Handler, *users.Service, *firewall.Service, *gorm.DB) {
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
	firewallService := firewall.NewService(db)
	if err := firewallService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate firewall table: %v", err)
	}
	createAdminUsersUsageTables(t, db)

	return NewHandler(Dependencies{Users: userService, Firewall: firewallService}), userService, firewallService, db
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

// TestHandleAdminUpdateUserFirewallOverridePreservesAndUpdates verifies omitted override is preserved and explicit override updates.
func TestHandleAdminUpdateUserFirewallOverridePreservesAndUpdates(t *testing.T) {
	handler, service := newUsersTestHandler(t)
	ctx := context.Background()

	created, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-firewall-override",
		PreferredUsername: "firewall-override-user",
		Email:             "firewall-override@example.com",
		Name:              "Firewall Override User",
	})
	if err != nil {
		t.Fatalf("sync user: %v", err)
	}

	enabled := true
	if _, err := service.UpdateUser(ctx, created.ID, users.UpdateUserInput{
		Role:                    created.Role,
		IsActive:                true,
		FirewallOverrideEnabled: &enabled,
	}); err != nil {
		t.Fatalf("prime firewall override: %v", err)
	}

	body, err := json.Marshal(map[string]any{
		"role":      auth.RoleUser,
		"isActive":  true,
		"expiresAt": nil,
	})
	if err != nil {
		t.Fatalf("marshal preserve request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+created.ID, bytes.NewReader(body))
	req.SetPathValue("id", created.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminUpdateUser(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	updated, err := service.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get preserved user: %v", err)
	}
	if !updated.FirewallOverrideEnabled {
		t.Fatal("expected omitted firewall override to preserve true")
	}

	body, err = json.Marshal(map[string]any{
		"role":                    auth.RoleUser,
		"isActive":                true,
		"firewallOverrideEnabled": false,
		"expiresAt":               nil,
	})
	if err != nil {
		t.Fatalf("marshal update request body: %v", err)
	}
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+created.ID, bytes.NewReader(body))
	req.SetPathValue("id", created.ID)
	recorder = httptest.NewRecorder()

	handler.HandleAdminUpdateUser(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	updated, err = service.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.FirewallOverrideEnabled {
		t.Fatal("expected explicit firewall override false")
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

// TestHandleAdminUserFirewallRules verifies handle admin user firewall rule CRUD and simulation.
func TestHandleAdminUserFirewallRules(t *testing.T) {
	handler, userService, firewallService, _ := newUsersFirewallTestHandler(t)
	ctx := context.Background()

	created, err := userService.SyncUser(ctx, auth.Identity{
		Sub:               "sub-user-firewall",
		PreferredUsername: "user-firewall",
		Email:             "user-firewall@example.com",
		Name:              "User Firewall",
	})
	if err != nil {
		t.Fatalf("sync user: %v", err)
	}

	body, err := json.Marshal(firewall.CreateRuleInput{
		Address:  "10.0.0.10",
		Priority: 1,
		Action:   firewall.ActionAllow,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+created.ID+"/firewall/rules", bytes.NewReader(body))
	req.SetPathValue("id", created.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminCreateUserFirewallRule(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var rule firewall.RuleResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &rule); err != nil {
		t.Fatalf("decode created rule: %v", err)
	}
	if rule.UserID != created.ID {
		t.Fatalf("expected userId %q, got %q", created.ID, rule.UserID)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+created.ID+"/firewall/rules", nil)
	listReq.SetPathValue("id", created.ID)
	listRecorder := httptest.NewRecorder()

	handler.HandleAdminListUserFirewallRules(listRecorder, listReq)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var list firewall.ListResult
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode firewall list: %v", err)
	}
	if list.Total != 1 || list.Items[0].ID != rule.ID {
		t.Fatalf("expected created firewall rule, got %#v", list)
	}

	updateBody, err := json.Marshal(firewall.UpdateRuleInput{
		Description: ptrString("Office network"),
	})
	if err != nil {
		t.Fatalf("marshal update body: %v", err)
	}
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+created.ID+"/firewall/rules/"+rule.ID, bytes.NewReader(updateBody))
	updateReq.SetPathValue("id", created.ID)
	updateReq.SetPathValue("ruleId", rule.ID)
	updateRecorder := httptest.NewRecorder()

	handler.HandleAdminUpdateUserFirewallRule(updateRecorder, updateReq)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	simulateBody, err := json.Marshal(map[string]string{"clientIp": "10.0.0.10"})
	if err != nil {
		t.Fatalf("marshal simulate body: %v", err)
	}
	simulateReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+created.ID+"/firewall/simulate", bytes.NewReader(simulateBody))
	simulateReq.SetPathValue("id", created.ID)
	simulateRecorder := httptest.NewRecorder()

	handler.HandleAdminSimulateUserFirewallRule(simulateRecorder, simulateReq)

	if simulateRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", simulateRecorder.Code, simulateRecorder.Body.String())
	}
	var simulation simulateFirewallResponse
	if err := json.Unmarshal(simulateRecorder.Body.Bytes(), &simulation); err != nil {
		t.Fatalf("decode simulate response: %v", err)
	}
	if !simulation.Allowed || simulation.MatchedRule == nil || simulation.MatchedRule.UserID != created.ID {
		t.Fatalf("expected user allow simulation, got %#v", simulation)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+created.ID+"/firewall/rules/"+rule.ID, nil)
	deleteReq.SetPathValue("id", created.ID)
	deleteReq.SetPathValue("ruleId", rule.ID)
	deleteRecorder := httptest.NewRecorder()

	handler.HandleAdminDeleteUserFirewallRule(deleteRecorder, deleteReq)

	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	remaining, err := firewallService.ListUserRulesPaged(ctx, created.ID, firewall.ListParams{})
	if err != nil {
		t.Fatalf("list remaining rules: %v", err)
	}
	if remaining.Total != 0 {
		t.Fatalf("expected no remaining rules, got %#v", remaining)
	}
}

// TestHandleAdminUserFirewallRejectsServiceAccount verifies user firewall routes hide service accounts.
func TestHandleAdminUserFirewallRejectsServiceAccount(t *testing.T) {
	handler, userService, _, _ := newUsersFirewallTestHandler(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+account.ID+"/firewall/rules", nil)
	req.SetPathValue("id", account.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminListUserFirewallRules(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "user_not_found") {
		t.Fatalf("expected user_not_found response, got %s", recorder.Body.String())
	}
}

// TestHandleAdminDeleteUserDeletesFirewallRules verifies deleting a user removes scoped firewall rules.
func TestHandleAdminDeleteUserDeletesFirewallRules(t *testing.T) {
	handler, userService, firewallService, db := newUsersFirewallTestHandler(t)
	ctx := context.Background()

	created, err := userService.SyncUser(ctx, auth.Identity{
		Sub:               "sub-delete-firewall",
		PreferredUsername: "delete-firewall",
		Email:             "delete-firewall@example.com",
		Name:              "Delete Firewall",
	})
	if err != nil {
		t.Fatalf("sync user: %v", err)
	}
	if _, err := firewallService.CreateUserRule(ctx, created.ID, firewall.CreateRuleInput{
		Address:  "10.0.0.10",
		Priority: 1,
		Action:   firewall.ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create user rule: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+created.ID, nil)
	req.SetPathValue("id", created.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminDeleteUser(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var count int64
	if err := db.Model(&firewall.FirewallRule{}).
		Where("type = ? AND referentiel_id = ?", firewall.RuleTypeUser, created.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count firewall rules: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected user firewall rules to be deleted, found %d", count)
	}
}

func ptrString(value string) *string {
	return &value
}
