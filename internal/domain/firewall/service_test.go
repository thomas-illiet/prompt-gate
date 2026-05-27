package firewall

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newFirewallServiceTestServices(t *testing.T) (*Service, *users.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	userService := users.NewService(db)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate users: %v", err)
	}
	firewallService := NewService(db)
	if err := firewallService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate firewall: %v", err)
	}

	return firewallService, userService, db
}

func TestServiceAccountFirewallRulesAreScopedByAccount(t *testing.T) {
	firewallService, userService, _ := newFirewallServiceTestServices(t)
	ctx := context.Background()

	accountA, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker_a",
		Name:       "Worker A",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create account A: %v", err)
	}
	accountB, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker_b",
		Name:       "Worker B",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create account B: %v", err)
	}

	if _, err := firewallService.CreateServiceAccountRule(ctx, accountA.ID, CreateRuleInput{
		Address:  "10.0.0.10",
		Priority: 1,
		Action:   ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create account A rule: %v", err)
	}
	if _, err := firewallService.CreateServiceAccountRule(ctx, accountB.ID, CreateRuleInput{
		Address:  "10.0.0.11",
		Priority: 1,
		Action:   ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("expected same priority to be allowed across accounts: %v", err)
	}
	if _, err := firewallService.CreateServiceAccountRule(ctx, accountA.ID, CreateRuleInput{
		Address:  "10.0.0.12",
		Priority: 1,
		Action:   ActionAllow,
		Enabled:  true,
	}); err != ErrPriorityConflict {
		t.Fatalf("expected priority conflict in same account, got %v", err)
	}
}

func TestServiceAccountAllowsDefaultsToDeny(t *testing.T) {
	firewallService, userService, _ := newFirewallServiceTestServices(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := firewallService.CreateServiceAccountRule(ctx, account.ID, CreateRuleInput{
		Address:  "10.0.0.10",
		Priority: 1,
		Action:   ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create allow rule: %v", err)
	}

	allowed, matched, err := firewallService.ServiceAccountAllows(ctx, account.ID, "10.0.0.10")
	if err != nil {
		t.Fatalf("evaluate allow rule: %v", err)
	}
	if !allowed || matched == nil || matched.Action != ActionAllow {
		t.Fatalf("expected explicit allow, allowed=%v matched=%#v", allowed, matched)
	}

	allowed, matched, err = firewallService.ServiceAccountAllows(ctx, account.ID, "10.0.0.11")
	if err != nil {
		t.Fatalf("evaluate no match: %v", err)
	}
	if allowed || matched != nil {
		t.Fatalf("expected default deny without match, allowed=%v matched=%#v", allowed, matched)
	}
}

func TestServiceAccountFirewallRulesAreDeletedWithAccount(t *testing.T) {
	firewallService, userService, db := newFirewallServiceTestServices(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := firewallService.CreateServiceAccountRule(ctx, account.ID, CreateRuleInput{
		Address:  "10.0.0.10",
		Priority: 1,
		Action:   ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create scoped rule: %v", err)
	}
	if err := userService.DeleteServiceAccount(ctx, account.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	var count int64
	if err := db.Model(&FirewallRule{}).
		Where("type = ? AND referentiel_id = ?", RuleTypeServiceAccount, account.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count scoped rules: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected scoped rules to be deleted with account, found %d", count)
	}
}

func TestSnapshotUsesServiceAccountOverrideBeforeGlobalRules(t *testing.T) {
	firewallService, userService, _ := newFirewallServiceTestServices(t)
	ctx := context.Background()
	overrideEnabled := true

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier:              "worker",
		Name:                    "Worker",
		IsActive:                true,
		FirewallOverrideEnabled: &overrideEnabled,
	})
	if err != nil {
		t.Fatalf("create override account: %v", err)
	}
	fallbackAccount, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "fallback_worker",
		Name:       "Fallback Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create fallback account: %v", err)
	}
	if _, err := firewallService.CreateRule(ctx, CreateRuleInput{
		Address:  "203.0.113.10",
		Priority: 1,
		Action:   ActionDeny,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create global deny: %v", err)
	}
	if _, err := firewallService.CreateServiceAccountRule(ctx, account.ID, CreateRuleInput{
		Address:  "203.0.113.10",
		Priority: 1,
		Action:   ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create scoped allow: %v", err)
	}

	store := NewSnapshotStore(firewallService)
	if err := store.Refresh(ctx); err != nil {
		t.Fatalf("refresh snapshot: %v", err)
	}
	overrideProfile, err := userService.ServiceAccountProfile(ctx, account.ID)
	if err != nil {
		t.Fatalf("load override profile: %v", err)
	}
	fallbackProfile, err := userService.ServiceAccountProfile(ctx, fallbackAccount.ID)
	if err != nil {
		t.Fatalf("load fallback profile: %v", err)
	}

	allowed, matched, err := store.AllowsUser("203.0.113.10", overrideProfile)
	if err != nil {
		t.Fatalf("evaluate override profile: %v", err)
	}
	if !allowed || matched == nil || matched.ServiceAccountID != account.ID {
		t.Fatalf("expected scoped allow to override global deny, allowed=%v matched=%#v", allowed, matched)
	}

	allowed, matched, err = store.AllowsUser("203.0.113.10", fallbackProfile)
	if err != nil {
		t.Fatalf("evaluate fallback profile: %v", err)
	}
	if allowed || matched == nil || matched.Action != ActionDeny {
		t.Fatalf("expected fallback account to use global deny, allowed=%v matched=%#v", allowed, matched)
	}
}
