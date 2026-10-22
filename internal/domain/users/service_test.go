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

// RevokeUserTokensTx records revoked user ids for assertions.
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

// createUsageTables creates usage tables.
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
		`CREATE TABLE proxy_daily_usage_kpis (
			day date NOT NULL,
			initiator_id text NOT NULL,
			requests integer NOT NULL DEFAULT 0,
			PRIMARY KEY (day, initiator_id)
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create usage table: %v", err)
		}
	}
}

// seedTokenConsumption seeds token consumption.
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

// adminUserByID returns admin user by ID.
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

// serviceAccountByID returns service account by ID.
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

// TestSyncUserMatchesPreferredUsernameAndPreservesManagedFields verifies SSO matching ignores subject changes.
func TestSyncUserMatchesPreferredUsernameAndPreservesManagedFields(t *testing.T) {
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
		Role:     auth.RoleLimited,
		IsActive: false,
	})
	if err != nil {
		t.Fatalf("update user before re-sync: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	resynced, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "replacement-sub",
		PreferredUsername: "promptgate-user",
		Email:             "manager@example.com",
		Name:              "PromptGate Limited",
	})
	if err != nil {
		t.Fatalf("re-sync user: %v", err)
	}

	if resynced.Role != auth.RoleLimited {
		t.Fatalf("expected limited role to be preserved, got %q", resynced.Role)
	}

	if resynced.IsActive {
		t.Fatal("expected inactive status to be preserved")
	}

	if resynced.ID != created.ID {
		t.Fatalf("expected preferred username to match the existing user, got ids %q and %q", created.ID, resynced.ID)
	}

	if resynced.Sub != "replacement-sub" || resynced.Email != "manager@example.com" || resynced.Name != "PromptGate Limited" || resynced.PreferredUsername != "promptgate-user" {
		t.Fatalf("expected identity fields to be refreshed, got %#v", resynced)
	}

	if updated.LastLoginAt == nil || !resynced.LastLoginAt.After(*updated.LastLoginAt) {
		t.Fatal("expected last login timestamp to move forward on re-sync")
	}
}

// TestSyncUserDoesNotMatchServiceAccountIdentifier verifies SSO matching is restricted to human users.
func TestSyncUserDoesNotMatchServiceAccountIdentifier(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	account, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "shared-identifier",
		Name:       "Shared Identifier Bot",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	profile, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "human-sub",
		PreferredUsername: "shared-identifier",
		Email:             "human@example.com",
		Name:              "Human User",
	})
	if err != nil {
		t.Fatalf("sync human user: %v", err)
	}

	if profile.ID == account.ID || profile.Type != auth.UserTypeUser {
		t.Fatalf("expected a distinct human user, got %#v", profile)
	}

	storedAccount, err := service.GetServiceAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("load service account: %v", err)
	}
	if storedAccount.Name != "Shared Identifier Bot" {
		t.Fatalf("expected service account to remain unchanged, got %#v", storedAccount)
	}
}

// TestSyncUserRequiresPreferredUsername verifies the SSO match key cannot be empty.
func TestSyncUserRequiresPreferredUsername(t *testing.T) {
	service := newTestService(t)

	_, err := service.SyncUser(context.Background(), auth.Identity{Sub: "sub-without-username"})
	if !errors.Is(err, ErrPreferredUsernameRequired) {
		t.Fatalf("expected preferred username validation error, got %v", err)
	}
}

func TestCreateUserCreatesPendingAccountFromExactTrimmedUID(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.CreateUser(ctx, "  Alice.Admin  ")
	if err != nil {
		t.Fatalf("create pending user: %v", err)
	}
	if created.PreferredUsername != "Alice.Admin" {
		t.Fatalf("expected trimmed case-preserved UID, got %q", created.PreferredUsername)
	}
	if created.Sub != nil || created.LastLoginAt != nil {
		t.Fatalf("expected pending SSO fields to be null, got %#v", created)
	}
	if created.Email != "" || created.Name != "" || created.Role != auth.RoleNone || !created.IsActive {
		t.Fatalf("unexpected pending user defaults: %#v", created)
	}
	if created.ActiveInLast30Days {
		t.Fatal("expected a newly pre-created user to be inactive on the platform")
	}

	if _, err := service.CreateUser(ctx, "Alice.Admin"); !errors.Is(err, ErrPreferredUsernameConflict) {
		t.Fatalf("expected exact UID conflict, got %v", err)
	}
	if _, err := service.CreateUser(ctx, "alice.admin"); err != nil {
		t.Fatalf("expected case-distinct UID to be accepted: %v", err)
	}
	if _, err := service.CreateUser(ctx, " \t "); !errors.Is(err, ErrPreferredUsernameRequired) {
		t.Fatalf("expected blank UID rejection, got %v", err)
	}
}

func TestCreateUserAllowsServiceAccountWithSameIdentifier(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	account, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "shared-uid",
		Name:       "Shared UID worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}
	person, err := service.CreateUser(ctx, "shared-uid")
	if err != nil {
		t.Fatalf("create human with shared identifier: %v", err)
	}
	if account.ID == person.ID || person.Type != auth.UserTypeUser {
		t.Fatalf("expected distinct service and human accounts, got account=%#v person=%#v", account, person)
	}
}

func TestCreateUserConcurrentConflictLeavesOneAccount(t *testing.T) {
	service := newTestService(t)
	sqlDB, err := service.db.DB()
	if err != nil {
		t.Fatalf("expose database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	start := make(chan struct{})
	errorsCh := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, createErr := service.CreateUser(context.Background(), "concurrent-user")
			errorsCh <- createErr
		}()
	}
	close(start)

	var successes, conflicts int
	for range 2 {
		switch err := <-errorsCh; {
		case err == nil:
			successes++
		case errors.Is(err, ErrPreferredUsernameConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent create error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one success and one conflict, got successes=%d conflicts=%d", successes, conflicts)
	}

	var count int64
	if err := service.db.Model(&User{}).
		Where("type = ? AND preferred_username = ?", auth.UserTypeUser, "concurrent-user").
		Count(&count).Error; err != nil {
		t.Fatalf("count concurrent user: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one persisted account, got %d", count)
	}
}

func TestCreateUserRacingWithSyncUserConvergesOnClaimedAccount(t *testing.T) {
	service := newTestService(t)
	sqlDB, err := service.db.DB()
	if err != nil {
		t.Fatalf("expose database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	type createResult struct {
		user AdminUser
		err  error
	}
	type syncResult struct {
		profile auth.UserProfile
		err     error
	}
	start := make(chan struct{})
	createdCh := make(chan createResult, 1)
	syncedCh := make(chan syncResult, 1)
	go func() {
		<-start
		user, createErr := service.CreateUser(context.Background(), "racing-user")
		createdCh <- createResult{user: user, err: createErr}
	}()
	go func() {
		<-start
		profile, syncErr := service.SyncUser(context.Background(), auth.Identity{
			Sub:               "racing-subject",
			PreferredUsername: "racing-user",
			Email:             "racing@example.com",
			Name:              "Racing User",
		})
		syncedCh <- syncResult{profile: profile, err: syncErr}
	}()
	close(start)

	created := <-createdCh
	if created.err != nil && !errors.Is(created.err, ErrPreferredUsernameConflict) {
		t.Fatalf("unexpected create side result: %v", created.err)
	}
	synced := <-syncedCh
	if synced.err != nil {
		t.Fatalf("sync side failed: %v", synced.err)
	}

	var records []User
	if err := service.db.Where(
		"type = ? AND preferred_username = ?",
		auth.UserTypeUser,
		"racing-user",
	).Find(&records).Error; err != nil {
		t.Fatalf("load racing user: %v", err)
	}
	if len(records) != 1 || records[0].ID != synced.profile.ID {
		t.Fatalf("expected one stable claimed account, records=%#v profile=%#v", records, synced.profile)
	}
	if created.err == nil && created.user.ID != synced.profile.ID {
		t.Fatalf("expected pre-created id %q to be claimed as %q", created.user.ID, synced.profile.ID)
	}
	if records[0].ExternalSub == nil || *records[0].ExternalSub != "racing-subject" ||
		records[0].LastLoginAt == nil || records[0].Email != "racing@example.com" || records[0].Name != "Racing User" {
		t.Fatalf("expected SSO fields on converged account, got %#v", records[0])
	}
}

func TestSyncUserClaimsPendingAccountAndPreservesManagedFields(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	pending, err := service.CreateUser(ctx, "pending-user")
	if err != nil {
		t.Fatalf("pre-create user: %v", err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := service.UpdateUser(ctx, pending.ID, UpdateUserInput{
		Role:      auth.RoleLimited,
		IsActive:  false,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("grant pending user access: %v", err)
	}
	if _, err := service.UpdateUserNote(ctx, pending.ID, UpdateAccountNoteInput{Note: "managed note"}); err != nil {
		t.Fatalf("set pending user note: %v", err)
	}
	planID := "11111111-1111-1111-1111-111111111111"
	if err := service.db.Model(&User{}).Where("id = ?", pending.ID).
		Update("subscription_plan_id", planID).Error; err != nil {
		t.Fatalf("assign pending subscription: %v", err)
	}

	profile, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "oidc-subject",
		PreferredUsername: "pending-user",
		Email:             "pending@example.com",
		Name:              "Pending User",
	})
	if err != nil {
		t.Fatalf("claim pending user through SSO: %v", err)
	}
	if profile.ID != pending.ID || profile.Role != auth.RoleLimited || profile.IsActive {
		t.Fatalf("expected local identity and access to be preserved, got %#v", profile)
	}
	if profile.Sub != "oidc-subject" || profile.Email != "pending@example.com" || profile.Name != "Pending User" || profile.LastLoginAt.IsZero() {
		t.Fatalf("expected SSO fields to be populated, got %#v", profile)
	}

	claimed, err := service.GetUser(ctx, pending.ID)
	if err != nil {
		t.Fatalf("load claimed user: %v", err)
	}
	if claimed.Note != "managed note" || claimed.SubscriptionPlanID == nil || *claimed.SubscriptionPlanID != planID || claimed.ExpiresAt == nil {
		t.Fatalf("expected managed fields to survive SSO claim, got %#v", claimed)
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

// TestCreateServiceAccountForcesServiceTypeAndUserRole verifies service account creation forces service type and user role.
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
	if record.ExternalSub == nil || !strings.HasPrefix(*record.ExternalSub, "service:") {
		t.Fatalf("expected service external sub, got %v", record.ExternalSub)
	}
}

func TestDeleteServiceAccountDoesNotRequireLegacyFirewallTable(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	account, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "ephemeral-worker",
		Name:       "Ephemeral Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	if err := service.DeleteServiceAccount(ctx, account.ID); err != nil {
		t.Fatalf("delete service account without legacy firewall table: %v", err)
	}
	if _, err := service.GetServiceAccount(ctx, account.ID); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected deleted service account to be absent, got %v", err)
	}
}

// TestListUsersIncludesHistoricalTokenConsumption verifies list users includes historical token consumption.
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

func TestAdminUserActivityUsesThirtyUTCDaysAndIsIndependentFromAccess(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()
	fixedNow := time.Date(2026, time.October, 23, 21, 45, 0, 0, time.FixedZone("test", 2*60*60))
	service.now = func() time.Time { return fixedNow }
	todayUTC := time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)

	create := func(uid string) AdminUser {
		t.Helper()
		user, err := service.CreateUser(ctx, uid)
		if err != nil {
			t.Fatalf("create %s: %v", uid, err)
		}
		return user
	}
	todayUser := create("today-user")
	boundaryUser := create("boundary-user")
	oldUser := create("old-user")
	zeroUser := create("zero-user")
	quietUser := create("quiet-user")

	seedActivity := func(userID string, day time.Time, requests int64) {
		t.Helper()
		if err := service.db.Exec(
			"INSERT INTO proxy_daily_usage_kpis (day, initiator_id, requests) VALUES (?, ?, ?)",
			day,
			userID,
			requests,
		).Error; err != nil {
			t.Fatalf("seed activity for %s: %v", userID, err)
		}
	}
	seedActivity(todayUser.ID, todayUTC, 1)
	seedActivity(boundaryUser.ID, todayUTC.AddDate(0, 0, -29), 2)
	seedActivity(oldUser.ID, todayUTC.AddDate(0, 0, -30), 3)
	seedActivity(zeroUser.ID, todayUTC.AddDate(0, 0, -5), 0)

	updated, err := service.UpdateUser(ctx, todayUser.ID, UpdateUserInput{
		Role:     auth.RoleNone,
		IsActive: false,
	})
	if err != nil {
		t.Fatalf("disable recently active user: %v", err)
	}
	if updated.IsActive || !updated.ActiveInLast30Days {
		t.Fatalf("expected disabled access and independent recent activity, got %#v", updated)
	}

	result, err := service.ListUsers(ctx, ListParams{Page: 1, PageSize: 10, Type: auth.UserTypeUser})
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	assertActivity := func(user AdminUser, want bool) {
		t.Helper()
		got := adminUserByID(t, result.Items, user.ID).ActiveInLast30Days
		if got != want {
			t.Fatalf("activity for %s = %t, want %t", user.PreferredUsername, got, want)
		}
	}
	assertActivity(todayUser, true)
	assertActivity(boundaryUser, true)
	assertActivity(oldUser, false)
	assertActivity(zeroUser, false)
	assertActivity(quietUser, false)

	detail, err := service.GetUser(ctx, boundaryUser.ID)
	if err != nil {
		t.Fatalf("get boundary user: %v", err)
	}
	if !detail.ActiveInLast30Days {
		t.Fatal("expected detail response to include recent activity")
	}
	noted, err := service.UpdateUserNote(ctx, boundaryUser.ID, UpdateAccountNoteInput{Note: "still active"})
	if err != nil {
		t.Fatalf("update active user note: %v", err)
	}
	if !noted.ActiveInLast30Days {
		t.Fatal("expected note response to include recent activity")
	}
}

func TestLastLoginSortAlwaysPlacesPendingUsersLast(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	pending, err := service.CreateUser(ctx, "pending")
	if err != nil {
		t.Fatalf("create pending user: %v", err)
	}
	linked, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "linked-sub",
		PreferredUsername: "linked",
		Email:             "linked@example.com",
		Name:              "Linked",
	})
	if err != nil {
		t.Fatalf("sync linked user: %v", err)
	}

	for _, direction := range []string{"asc", "desc"} {
		result, err := service.ListUsers(ctx, ListParams{
			Page:     1,
			PageSize: 10,
			Type:     auth.UserTypeUser,
			SortBy:   "lastLoginAt",
			SortDir:  direction,
		})
		if err != nil {
			t.Fatalf("sort last login %s: %v", direction, err)
		}
		if len(result.Items) != 2 || result.Items[0].ID != linked.ID || result.Items[1].ID != pending.ID {
			t.Fatalf("expected pending user last for %s sort, got %#v", direction, result.Items)
		}
	}
}

// TestListServiceAccountsIncludesHistoricalTokenConsumption verifies list service accounts includes historical token consumption.
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

// TestAccountListsSearchByTechnicalID verifies dashboard deep links can filter
// both account kinds by their stable technical identifier.
func TestAccountListsSearchByTechnicalID(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	person, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "searchable-person",
		PreferredUsername: "person",
		Email:             "person@example.com",
		Name:              "Searchable Person",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "searchable-worker",
		Name:       "Searchable Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	usersResult, err := service.ListUsers(ctx, ListParams{
		Page:     1,
		PageSize: 10,
		Search:   person.ID,
		Type:     auth.UserTypeUser,
	})
	if err != nil {
		t.Fatalf("search users by id: %v", err)
	}
	if len(usersResult.Items) != 1 || usersResult.Items[0].ID != person.ID {
		t.Fatalf("unexpected user search result: %#v", usersResult.Items)
	}

	accountsResult, err := service.ListServiceAccountsPaged(ctx, ServiceAccountListParams{
		Page:     1,
		PageSize: 10,
		Search:   account.ID,
	})
	if err != nil {
		t.Fatalf("search service accounts by id: %v", err)
	}
	if len(accountsResult.Items) != 1 || accountsResult.Items[0].ID != account.ID {
		t.Fatalf("unexpected service account search result: %#v", accountsResult.Items)
	}
}

// TestServiceAccountIdentifierValidationAndConflict verifies service account identifier validation and conflict.
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

// TestUpdateUserValidatesExpirationAndRevokesTokensWhenRoleBecomesNone verifies update user validates expiration and revokes tokens when role becomes none.
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

// TestUpdateAccountNotesPersistClearValidateAndRespectAccountType verifies account notes persist, clear, validate, and respect account type.
func TestUpdateAccountNotesPersistClearValidateAndRespectAccountType(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	user, err := service.SyncUser(ctx, auth.Identity{
		Sub:               "sub-1",
		PreferredUsername: "promptgate-user",
		Email:             "user@example.com",
		Name:              "PromptGate User",
	})
	if err != nil {
		t.Fatalf("sync user: %v", err)
	}
	account, err := service.CreateServiceAccount(ctx, ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	updatedUser, err := service.UpdateUserNote(ctx, user.ID, UpdateAccountNoteInput{
		Note: "VIP access approved by security.",
	})
	if err != nil {
		t.Fatalf("update user note: %v", err)
	}
	if updatedUser.Note != "VIP access approved by security." {
		t.Fatalf("expected updated user note, got %q", updatedUser.Note)
	}
	reloadedUser, err := service.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloadedUser.Note != updatedUser.Note {
		t.Fatalf("expected persisted user note %q, got %q", updatedUser.Note, reloadedUser.Note)
	}

	clearedUser, err := service.UpdateUserNote(ctx, user.ID, UpdateAccountNoteInput{})
	if err != nil {
		t.Fatalf("clear user note: %v", err)
	}
	if clearedUser.Note != "" {
		t.Fatalf("expected cleared user note, got %q", clearedUser.Note)
	}

	updatedAccount, err := service.UpdateServiceAccountNote(ctx, account.ID, UpdateAccountNoteInput{
		Note: "Rotation owner: platform.",
	})
	if err != nil {
		t.Fatalf("update service account note: %v", err)
	}
	if updatedAccount.Note != "Rotation owner: platform." {
		t.Fatalf("expected updated service account note, got %q", updatedAccount.Note)
	}
	reloadedAccount, err := service.GetServiceAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("reload service account: %v", err)
	}
	if reloadedAccount.Note != updatedAccount.Note {
		t.Fatalf("expected persisted service account note %q, got %q", updatedAccount.Note, reloadedAccount.Note)
	}

	if _, err := service.UpdateUserNote(ctx, user.ID, UpdateAccountNoteInput{Note: strings.Repeat("a", maxAccountNoteLength+1)}); !errors.Is(err, ErrInvalidNote) {
		t.Fatalf("expected ErrInvalidNote, got %v", err)
	}
	if _, err := service.UpdateUserNote(ctx, account.ID, UpdateAccountNoteInput{Note: "wrong type"}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected service account to be hidden from user note update, got %v", err)
	}
	if _, err := service.UpdateServiceAccountNote(ctx, user.ID, UpdateAccountNoteInput{Note: "wrong type"}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected user to be hidden from service account note update, got %v", err)
	}
}

// TestExpireAccessClearsExpiredRolesAndRevokesTokens verifies expire access clears expired roles and revokes tokens.
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
