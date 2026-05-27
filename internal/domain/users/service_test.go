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

type recordingTokenRevoker struct {
	calls [][]string
}

func (r *recordingTokenRevoker) RevokeUserTokensTx(_ context.Context, _ *gorm.DB, userIDs []string, _ time.Time) (int64, error) {
	copied := append([]string(nil), userIDs...)
	r.calls = append(r.calls, copied)
	return int64(len(userIDs)), nil
}

// newTestService creates an in-memory user service for tests.
func newTestService(t *testing.T) *Service {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate users table: %v", err)
	}
	createUsageTables(t, db)

	return service
}

func createUsageTables(t *testing.T, db *gorm.DB) {
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

func seedTokenConsumption(t *testing.T, db *gorm.DB, userID, interceptionID string, inputTokens, outputTokens int64) {
	t.Helper()

	if err := db.Exec(
		"INSERT INTO interceptions (id, initiator_id) VALUES (?, ?)",
		interceptionID,
		userID,
	).Error; err != nil {
		t.Fatalf("seed interception: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO token_usages (id, interception_id, input_tokens, output_tokens) VALUES (?, ?, ?, ?)",
		"usage-"+interceptionID,
		interceptionID,
		inputTokens,
		outputTokens,
	).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}
}

func adminUserByID(t *testing.T, items []AdminUser, id string) AdminUser {
	t.Helper()

	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("admin user %s not found in %#v", id, items)
	return AdminUser{}
}

func serviceAccountByID(t *testing.T, items []ServiceAccount, id string) ServiceAccount {
	t.Helper()

	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("service account %s not found in %#v", id, items)
	return ServiceAccount{}
}

// TestSyncUserBootstrapsFirstAdminThenDefaultsToNone verifies first-user bootstrap behavior.
func TestSyncUserBootstrapsFirstAdminThenDefaultsToNone(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	first, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-1",
		PreferredUsername: "promptgate-admin",
		Email:             "admin@example.com",
		Name:              "PromptGate",
	})
	if err != nil {
		t.Fatalf("sync first user: %v", err)
	}

	second, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-2",
		PreferredUsername: "promptgate-user",
		Email:             "user@example.com",
		Name:              "PromptGate User",
	})
	if err != nil {
		t.Fatalf("sync second user: %v", err)
	}

	if first.Role != auth.RoleAdmin {
		t.Fatalf("expected first user to be admin, got %q", first.Role)
	}

	if second.Role != auth.RoleNone {
		t.Fatalf("expected second user to default to none, got %q", second.Role)
	}

	if !second.IsActive {
		t.Fatal("expected second user to be active on creation")
	}
}

// TestSyncUserPreservesRoleAndStatusWhileRefreshingIdentity verifies sync does not overwrite admin-managed fields.
func TestSyncUserPreservesRoleAndStatusWhileRefreshingIdentity(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-1",
		PreferredUsername: "promptgate-user",
		Email:             "user@example.com",
		Name:              "PromptGate User",
	})
	if err != nil {
		t.Fatalf("sync initial user: %v", err)
	}

	updated, err := service.UpdateUser(ctx, created.ID, UpdateUserInput{
		Role:     auth.RoleManager,
		IsActive: false,
	})
	if err != nil {
		t.Fatalf("update user before re-sync: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	resynced, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-1",
		PreferredUsername: "promptgate-manager",
		Email:             "manager@example.com",
		Name:              "PromptGate Manager",
	})
	if err != nil {
		t.Fatalf("re-sync user: %v", err)
	}

	if resynced.Role != auth.RoleManager {
		t.Fatalf("expected manager role to be preserved, got %q", resynced.Role)
	}

	if resynced.IsActive {
		t.Fatal("expected inactive status to be preserved")
	}

	if resynced.Email != "manager@example.com" || resynced.Name != "PromptGate Manager" || resynced.PreferredUsername != "promptgate-manager" {
		t.Fatalf("expected identity fields to be refreshed, got %#v", resynced)
	}

	if !resynced.LastLoginAt.After(updated.LastLoginAt) {
		t.Fatal("expected last login timestamp to move forward on re-sync")
	}
}

// TestDeleteAndRecreateUserKeepsNoneRoleWhenTableNotEmpty verifies recreated users are not promoted after bootstrap.
func TestDeleteAndRecreateUserKeepsNoneRoleWhenTableNotEmpty(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "admin-sub",
		PreferredUsername: "promptgate-admin",
		Email:             "admin@example.com",
		Name:              "PromptGate",
	}); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}

	created, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "user-sub",
		PreferredUsername: "promptgate-user",
		Email:             "user@example.com",
		Name:              "PromptGate User",
	})
	if err != nil {
		t.Fatalf("sync user to delete: %v", err)
	}

	if err := service.DeleteUser(ctx, created.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	recreated, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "user-sub",
		PreferredUsername: "promptgate-user",
		Email:             "user@example.com",
		Name:              "PromptGate User",
	})
	if err != nil {
		t.Fatalf("recreate deleted user: %v", err)
	}

	if recreated.Role != auth.RoleNone {
		t.Fatalf("expected recreated user to come back as none, got %q", recreated.Role)
	}

	if recreated.ID == created.ID {
		t.Fatal("expected recreated user to get a fresh local id")
	}
}

func TestCreateServiceAccountForcesServiceTypeAndUserRole(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	account, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "worker_bot",
		Name:       "Worker Bot",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}
	if account.Identifier != "worker_bot" {
		t.Fatalf("expected normalized identifier, got %q", account.Identifier)
	}
	if account.Role != auth.RoleUser {
		t.Fatalf("expected service account role user, got %q", account.Role)
	}
	if !account.IsActive {
		t.Fatal("expected service account to be active")
	}

	var record User
	if err := service.db.First(&record, "id = ?", account.ID).Error; err != nil {
		t.Fatalf("load service account record: %v", err)
	}
	if record.Type != auth.UserTypeService {
		t.Fatalf("expected type service, got %q", record.Type)
	}
	if record.Role != auth.RoleUser {
		t.Fatalf("expected stored role user, got %q", record.Role)
	}
	if !strings.HasPrefix(record.ExternalSub, "service:") {
		t.Fatalf("expected service external sub, got %q", record.ExternalSub)
	}
}

func TestListUsersIncludesHistoricalTokenConsumption(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	first, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-1",
		PreferredUsername: "promptgate-user",
		Email:             "user@example.com",
		Name:              "PromptGate User",
	})
	if err != nil {
		t.Fatalf("sync first user: %v", err)
	}
	second, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-2",
		PreferredUsername: "quiet-user",
		Email:             "quiet@example.com",
		Name:              "Quiet User",
	})
	if err != nil {
		t.Fatalf("sync second user: %v", err)
	}

	seedTokenConsumption(t, service.db, first.ID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 11, 13)
	seedTokenConsumption(t, service.db, first.ID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", 17, 19)
	seedTokenConsumption(t, service.db, second.ID, "cccccccc-cccc-cccc-cccc-cccccccccccc", 101, 103)

	result, err := service.ListUsers(ctx, ListParams{
		Page:     1,
		PageSize: 10,
		Type:     auth.UserTypeUser,
	})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	gotFirst := adminUserByID(t, result.Items, first.ID)
	if gotFirst.InputTokens != 28 || gotFirst.OutputTokens != 32 {
		t.Fatalf("expected first user token totals 28/32, got %#v", gotFirst)
	}
	gotSecond := adminUserByID(t, result.Items, second.ID)
	if gotSecond.InputTokens != 101 || gotSecond.OutputTokens != 103 {
		t.Fatalf("expected second user token totals 101/103, got %#v", gotSecond)
	}
}

func TestListServiceAccountsIncludesHistoricalTokenConsumption(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	active, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create active service account: %v", err)
	}
	quiet, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "quiet",
		Name:       "Quiet",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create quiet service account: %v", err)
	}
	if active.InputTokens != 0 || active.OutputTokens != 0 {
		t.Fatalf("expected created service account to start at zero tokens, got %#v", active)
	}

	seedTokenConsumption(t, service.db, active.ID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 23, 29)

	accounts, err := service.ListServiceAccounts(ctx)
	if err != nil {
		t.Fatalf("list service accounts: %v", err)
	}

	gotActive := serviceAccountByID(t, accounts, active.ID)
	if gotActive.InputTokens != 23 || gotActive.OutputTokens != 29 {
		t.Fatalf("expected active account token totals 23/29, got %#v", gotActive)
	}
	gotQuiet := serviceAccountByID(t, accounts, quiet.ID)
	if gotQuiet.InputTokens != 0 || gotQuiet.OutputTokens != 0 {
		t.Fatalf("expected quiet account zero token totals, got %#v", gotQuiet)
	}
}

func TestServiceAccountIdentifierValidationAndConflict(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "Bad Identifier",
		Name:       "Bad",
		IsActive:   true,
	}); !errors.Is(err, ErrInvalidServiceAccountIdentifier) {
		t.Fatalf("expected invalid identifier, got %v", err)
	}

	if _, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "worker",
		Name:       "",
		IsActive:   true,
	}); !errors.Is(err, ErrInvalidServiceAccountName) {
		t.Fatalf("expected invalid name, got %v", err)
	}

	if _, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	}); err != nil {
		t.Fatalf("create first service account: %v", err)
	}
	if _, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker Again",
		IsActive:   true,
	}); !errors.Is(err, ErrServiceAccountConflict) {
		t.Fatalf("expected identifier conflict, got %v", err)
	}
}

func TestUpdateUserValidatesExpirationAndRevokesTokensWhenRoleBecomesNone(t *testing.T) {
	service := newTestService(t)
	revoker := &recordingTokenRevoker{}
	service.SetTokenRevoker(revoker)
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

	future := time.Now().UTC().Add(time.Hour)
	updated, err := service.UpdateUser(ctx, created.ID, UpdateUserInput{
		Role:      auth.RoleUser,
		IsActive:  true,
		ExpiresAt: &future,
	})
	if err != nil {
		t.Fatalf("update future expiration: %v", err)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(future) {
		t.Fatalf("expected future expiration to be stored, got %v", updated.ExpiresAt)
	}

	past := time.Now().UTC().Add(-time.Minute)
	if _, err := service.UpdateUser(ctx, created.ID, UpdateUserInput{
		Role:      auth.RoleUser,
		IsActive:  true,
		ExpiresAt: &past,
	}); err != ErrInvalidExpiration {
		t.Fatalf("expected ErrInvalidExpiration, got %v", err)
	}

	updated, err = service.UpdateUser(ctx, created.ID, UpdateUserInput{
		Role:      auth.RoleNone,
		IsActive:  true,
		ExpiresAt: &future,
	})
	if err != nil {
		t.Fatalf("update role none: %v", err)
	}
	if updated.ExpiresAt != nil {
		t.Fatalf("expected expiration to be cleared, got %v", updated.ExpiresAt)
	}
	if len(revoker.calls) != 1 || len(revoker.calls[0]) != 1 || revoker.calls[0][0] != created.ID {
		t.Fatalf("expected token revocation for user %s, got %#v", created.ID, revoker.calls)
	}
}

func TestExpireAccessClearsExpiredRolesAndRevokesTokens(t *testing.T) {
	service := newTestService(t)
	revoker := &recordingTokenRevoker{}
	service.SetTokenRevoker(revoker)
	ctx := context.Background()
	now := time.Now().UTC()

	expired, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "expired-sub",
		PreferredUsername: "expired-user",
		Email:             "expired@example.com",
		Name:              "Expired User",
	})
	if err != nil {
		t.Fatalf("sync expired user: %v", err)
	}
	future, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "future-sub",
		PreferredUsername: "future-user",
		Email:             "future@example.com",
		Name:              "Future User",
	})
	if err != nil {
		t.Fatalf("sync future user: %v", err)
	}
	alreadyNone, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "none-sub",
		PreferredUsername: "none-user",
		Email:             "none@example.com",
		Name:              "None User",
	})
	if err != nil {
		t.Fatalf("sync none user: %v", err)
	}

	expiredAt := now.Add(-time.Minute)
	futureAt := now.Add(time.Hour)
	if err := service.db.Model(&User{}).Where("id = ?", expired.ID).Updates(map[string]any{
		"role":       auth.RoleUser,
		"expires_at": expiredAt,
	}).Error; err != nil {
		t.Fatalf("seed expired access: %v", err)
	}
	if err := service.db.Model(&User{}).Where("id = ?", future.ID).Updates(map[string]any{
		"role":       auth.RoleUser,
		"expires_at": futureAt,
	}).Error; err != nil {
		t.Fatalf("seed future access: %v", err)
	}
	if err := service.db.Model(&User{}).Where("id = ?", alreadyNone.ID).Update("expires_at", expiredAt).Error; err != nil {
		t.Fatalf("seed none access: %v", err)
	}

	count, err := service.ExpireAccess(ctx, now)
	if err != nil {
		t.Fatalf("expire access: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one expired user, got %d", count)
	}

	gotExpired, err := service.GetUser(ctx, expired.ID)
	if err != nil {
		t.Fatalf("get expired user: %v", err)
	}
	if gotExpired.Role != auth.RoleNone {
		t.Fatalf("expected expired user role none, got %q", gotExpired.Role)
	}
	if gotExpired.ExpiresAt != nil {
		t.Fatalf("expected expired user expiration cleared, got %v", gotExpired.ExpiresAt)
	}
	if !gotExpired.IsActive {
		t.Fatal("expected expiration to keep user active flag unchanged")
	}

	gotFuture, err := service.GetUser(ctx, future.ID)
	if err != nil {
		t.Fatalf("get future user: %v", err)
	}
	if gotFuture.Role != auth.RoleUser {
		t.Fatalf("expected future user role user, got %q", gotFuture.Role)
	}
	if gotFuture.ExpiresAt == nil || !gotFuture.ExpiresAt.Equal(futureAt) {
		t.Fatalf("expected future expiration to remain, got %v", gotFuture.ExpiresAt)
	}
	if len(revoker.calls) != 1 || len(revoker.calls[0]) != 1 || revoker.calls[0][0] != expired.ID {
		t.Fatalf("expected token revocation for expired user %s, got %#v", expired.ID, revoker.calls)
	}
}
