package tokens

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testJWTSecret = "01234567890123456789012345678901"

// newTokenTestServices creates token and user services with an active test user.
func newTokenTestServices(t *testing.T) (*Service, *users.Service, *gorm.DB, auth.UserProfile) {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared&_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	userService := users.NewService(db)
	tokenService := NewService(db, testJWTSecret)
	userService.SetTokenRevoker(tokenService)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	if err := tokenService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}
	if err := firewall.NewService(db).AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate firewall: %v", err)
	}
	createUsageTables(t, db)
	user := users.User{
		ID:                "11111111-1111-1111-1111-111111111111",
		ExternalSub:       "sub",
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return tokenService, userService, db, userProfile(user)
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

// TestCreateTokenRequiresExistingUser verifies token rows cannot be orphaned.
func TestCreateTokenRequiresExistingUser(t *testing.T) {
	tokenService, _, _, user := newTokenTestServices(t)
	ctx := context.Background()

	user.ID = "22222222-2222-2222-2222-222222222222"
	if _, err := tokenService.CreateToken(ctx, user, "orphan", "", nil); err == nil {
		t.Fatal("expected creating a token for a missing user to fail")
	}
}

// TestDeleteUserCascadesTokens verifies deleting a user removes owned tokens.
func TestDeleteUserCascadesTokens(t *testing.T) {
	tokenService, userService, db, user := newTokenTestServices(t)
	ctx := context.Background()

	created, err := tokenService.CreateToken(ctx, user, "cli", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if err := userService.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int64
	if err := db.Model(&Token{}).Where("id = ?", created.TokenInfo.ID).Count(&count).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected token to be deleted by cascade, found %d", count)
	}
}

func TestDeleteServiceAccountCascadesTokens(t *testing.T) {
	tokenService, userService, db, _ := newTokenTestServices(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}
	profile, err := userService.ServiceAccountProfile(ctx, account.ID)
	if err != nil {
		t.Fatalf("load service account profile: %v", err)
	}
	created, err := tokenService.CreateToken(ctx, profile, "worker_token", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if err := userService.DeleteServiceAccount(ctx, account.ID); err != nil {
		t.Fatalf("delete service account: %v", err)
	}

	var count int64
	if err := db.Model(&Token{}).Where("id = ?", created.TokenInfo.ID).Count(&count).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected service account token to be deleted by cascade, found %d", count)
	}
}

// TestValidateTokenAcceptsActiveUserToken verifies valid active-user tokens are accepted.
func TestValidateTokenAcceptsActiveUserToken(t *testing.T) {
	tokenService, userService, _, user := newTokenTestServices(t)
	ctx := context.Background()

	created, err := tokenService.CreateToken(ctx, user, "cli", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	validated, err := tokenService.ValidateToken(ctx, created.Token, userService)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if validated.ID != user.ID {
		t.Fatalf("expected user %s, got %s", user.ID, validated.ID)
	}
}

func TestCreateTokenAcceptsRequestedTTLWithin365Days(t *testing.T) {
	tokenService, _, _, user := newTokenTestServices(t)
	ctx := context.Background()
	expiresInDays := 365
	before := time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)

	created, err := tokenService.CreateToken(ctx, user, "cli", "", &expiresInDays)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	after := time.Now().UTC().Add(time.Duration(expiresInDays) * 24 * time.Hour)
	if created.TokenInfo.ExpiresAt.Before(before) || created.TokenInfo.ExpiresAt.After(after) {
		t.Fatalf("expected expiration around 365 days from now, got %v", created.TokenInfo.ExpiresAt)
	}
}

func TestCreateTokenRejectsRequestedTTLOutsideRange(t *testing.T) {
	tokenService, _, _, user := newTokenTestServices(t)
	ctx := context.Background()

	for _, days := range []int{0, -1, 366} {
		if _, err := tokenService.CreateToken(ctx, user, "cli", "", &days); !errors.Is(err, ErrInvalidTTL) {
			t.Fatalf("expected ErrInvalidTTL for %d days, got %v", days, err)
		}
	}
}

func TestTokenSearchConditionCastsIDBeforeLowercase(t *testing.T) {
	if !strings.Contains(tokenSearchCondition, "LOWER(CAST(id AS TEXT))") {
		t.Fatalf("expected token search condition to cast uuid id before LOWER, got %q", tokenSearchCondition)
	}
}

func TestListTokensPagedSearchesByID(t *testing.T) {
	tokenService, _, _, user := newTokenTestServices(t)
	ctx := context.Background()

	created, err := tokenService.CreateToken(ctx, user, "cli", "primary token", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if _, err := tokenService.CreateToken(ctx, user, "worker", "secondary token", nil); err != nil {
		t.Fatalf("create second token: %v", err)
	}

	result, err := tokenService.ListTokensPaged(ctx, user.ID, ListParams{
		Page:           1,
		PageSize:       10,
		Search:         strings.ToUpper(created.TokenInfo.ID),
		SortBy:         "createdAt",
		SortDir:        "desc",
		IncludeRevoked: true,
	})
	if err != nil {
		t.Fatalf("search tokens by id: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("expected one token matching id, got total=%d items=%d", result.Total, len(result.Items))
	}
	if result.Items[0].ID != created.TokenInfo.ID {
		t.Fatalf("expected token %s, got %s", created.TokenInfo.ID, result.Items[0].ID)
	}
}

func TestValidateTokenAcceptsActiveServiceAccountAndRejectsInactive(t *testing.T) {
	tokenService, userService, _, _ := newTokenTestServices(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}
	profile, err := userService.ServiceAccountProfile(ctx, account.ID)
	if err != nil {
		t.Fatalf("load service account profile: %v", err)
	}
	expiresInDays := 365
	created, err := tokenService.CreateToken(ctx, profile, "worker_token", "", &expiresInDays)
	if err != nil {
		t.Fatalf("create service account token: %v", err)
	}

	validated, err := tokenService.ValidateToken(ctx, created.Token, userService)
	if err != nil {
		t.Fatalf("validate active service account token: %v", err)
	}
	if validated.ID != account.ID || validated.Type != auth.UserTypeService {
		t.Fatalf("expected service account profile, got %#v", validated)
	}

	if _, err := userService.UpdateServiceAccount(ctx, account.ID, users.ServiceAccountInput{
		Identifier: account.Identifier,
		Name:       account.Name,
		IsActive:   false,
	}); err != nil {
		t.Fatalf("deactivate service account: %v", err)
	}
	if _, err := tokenService.ValidateToken(ctx, created.Token, userService); !errors.Is(err, ErrAccountInactive) {
		t.Fatalf("expected inactive service account rejection, got %v", err)
	}
}

// TestValidateTokenRejectsRevokedTokenAndInactiveUser verifies revoked tokens and inactive users are rejected.
func TestValidateTokenRejectsRevokedTokenAndInactiveUser(t *testing.T) {
	tokenService, userService, db, user := newTokenTestServices(t)
	ctx := context.Background()

	created, err := tokenService.CreateToken(ctx, user, "cli", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if err := tokenService.RevokeToken(ctx, user.ID, created.TokenInfo.ID); err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	if _, err := tokenService.ValidateToken(ctx, created.Token, userService); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("expected revoked token, got %v", err)
	}

	active, err := tokenService.CreateToken(ctx, user, "cli2", "", nil)
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}
	if err := db.Model(&users.User{}).Where("id = ?", user.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate user: %v", err)
	}
	if _, err := tokenService.ValidateToken(ctx, active.Token, userService); !errors.Is(err, ErrAccountInactive) {
		t.Fatalf("expected inactive account, got %v", err)
	}
}

func TestRevokeTokenRejectsDifferentOwner(t *testing.T) {
	tokenService, _, db, user := newTokenTestServices(t)
	ctx := context.Background()

	created, err := tokenService.CreateToken(ctx, user, "cli", "", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if err := tokenService.RevokeToken(ctx, "22222222-2222-2222-2222-222222222222", created.TokenInfo.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound for different owner, got %v", err)
	}

	var record Token
	if err := db.First(&record, "id = ?", created.TokenInfo.ID).Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if record.RevokedAt != nil {
		t.Fatalf("expected token to remain active, got revoked at %v", record.RevokedAt)
	}
}

func TestUpdateUserRoleNoneRevokesActiveTokens(t *testing.T) {
	tokenService, userService, db, user := newTokenTestServices(t)
	ctx := context.Background()

	first, err := tokenService.CreateToken(ctx, user, "cli", "", nil)
	if err != nil {
		t.Fatalf("create first token: %v", err)
	}
	second, err := tokenService.CreateToken(ctx, user, "cli2", "", nil)
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}

	if _, err := userService.UpdateUser(ctx, user.ID, users.UpdateUserInput{
		Role:     auth.RoleNone,
		IsActive: true,
	}); err != nil {
		t.Fatalf("update user role none: %v", err)
	}

	var revokedCount int64
	if err := db.Model(&Token{}).
		Where("id IN ? AND revoked_at IS NOT NULL", []string{first.TokenInfo.ID, second.TokenInfo.ID}).
		Count(&revokedCount).Error; err != nil {
		t.Fatalf("count revoked tokens: %v", err)
	}
	if revokedCount != 2 {
		t.Fatalf("expected both tokens revoked, got %d", revokedCount)
	}

	if _, err := tokenService.ValidateToken(ctx, first.Token, userService); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("expected first token revoked, got %v", err)
	}
}

// userProfile converts a user model into an auth profile for tests.
func userProfile(user users.User) auth.UserProfile {
	return auth.UserProfile{
		ID:                user.ID,
		Sub:               user.ExternalSub,
		PreferredUsername: user.PreferredUsername,
		Email:             user.Email,
		Name:              user.Name,
		Type:              user.Type,
		Role:              user.Role,
		IsActive:          user.IsActive,
		LastLoginAt:       user.LastLoginAt,
	}
}
