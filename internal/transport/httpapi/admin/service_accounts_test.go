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

	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newServiceAccountsTestHandler(t *testing.T) (*Handler, *users.Service, *tokens.Service) {
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
	tokenService := tokens.NewService(db, "01234567890123456789012345678901")
	userService.SetTokenRevoker(tokenService)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate users table: %v", err)
	}
	if err := tokenService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate tokens table: %v", err)
	}
	firewallService := firewall.NewService(db)
	if err := firewallService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate firewall tables: %v", err)
	}

	return NewHandler(userService, tokenService, firewallService, nil, nil, nil), userService, tokenService
}

func TestHandleAdminCreateServiceAccountRejectsInvalidIdentifier(t *testing.T) {
	handler, _, _ := newServiceAccountsTestHandler(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/service-accounts",
		bytes.NewBufferString(`{"identifier":"Bad Identifier","name":"Bad","isActive":true}`),
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminCreateServiceAccount(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleAdminCreateServiceAccountTokenRejectsTTLAbove365(t *testing.T) {
	handler, userService, _ := newServiceAccountsTestHandler(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/service-accounts/"+account.ID+"/tokens",
		bytes.NewBufferString(`{"name":"worker_token","description":"","expiresInDays":366}`),
	)
	req.SetPathValue("id", account.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminCreateServiceAccountToken(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleAdminListServiceAccountTokensFiltersRevokedByDefault(t *testing.T) {
	handler, userService, tokenService := newServiceAccountsTestHandler(t)
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

	active, err := tokenService.CreateToken(ctx, profile, "active_token", "", nil)
	if err != nil {
		t.Fatalf("create active token: %v", err)
	}
	revoked, err := tokenService.CreateToken(ctx, profile, "revoked_token", "", nil)
	if err != nil {
		t.Fatalf("create revoked token: %v", err)
	}
	if err := tokenService.RevokeToken(ctx, profile.ID, revoked.TokenInfo.ID); err != nil {
		t.Fatalf("revoke token: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/service-accounts/"+account.ID+"/tokens",
		nil,
	)
	req.SetPathValue("id", account.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminListServiceAccountTokens(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var list tokens.ListResult
	if err := json.NewDecoder(recorder.Body).Decode(&list); err != nil {
		t.Fatalf("decode token list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != active.TokenInfo.ID {
		t.Fatalf("expected only active token, got %#v", list)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/service-accounts/"+account.ID+"/tokens?includeRevoked=true",
		nil,
	)
	req.SetPathValue("id", account.ID)
	recorder = httptest.NewRecorder()

	handler.HandleAdminListServiceAccountTokens(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if err := json.NewDecoder(recorder.Body).Decode(&list); err != nil {
		t.Fatalf("decode token list with revoked: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected active and revoked tokens, got %#v", list)
	}
}

func TestHandleAdminServiceAccountFirewallRules(t *testing.T) {
	handler, userService, _ := newServiceAccountsTestHandler(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/service-accounts/"+account.ID+"/firewall/rules",
		bytes.NewBufferString(`{"address":"10.0.0.10","priority":1,"action":"allow","enabled":true,"description":"ci"}`),
	)
	req.SetPathValue("id", account.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminCreateServiceAccountFirewallRule(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var created firewall.RuleResponse
	if err := json.NewDecoder(recorder.Body).Decode(&created); err != nil {
		t.Fatalf("decode created firewall rule: %v", err)
	}
	if created.ServiceAccountID != account.ID {
		t.Fatalf("expected serviceAccountId %q, got %q", account.ID, created.ServiceAccountID)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/service-accounts/"+account.ID+"/firewall/rules",
		nil,
	)
	req.SetPathValue("id", account.ID)
	recorder = httptest.NewRecorder()

	handler.HandleAdminListServiceAccountFirewallRules(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var list firewall.ListResult
	if err := json.NewDecoder(recorder.Body).Decode(&list); err != nil {
		t.Fatalf("decode firewall list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("expected created firewall rule, got %#v", list)
	}
}

func TestHandleAdminServiceAccountFirewallSimulateDefaultsToDeny(t *testing.T) {
	handler, userService, _ := newServiceAccountsTestHandler(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/service-accounts/"+account.ID+"/firewall/simulate",
		bytes.NewBufferString(`{"clientIp":"10.0.0.25"}`),
	)
	req.SetPathValue("id", account.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminSimulateServiceAccountFirewallRule(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response simulateFirewallResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode simulation response: %v", err)
	}
	if response.Allowed || response.MatchedRule != nil {
		t.Fatalf("expected default deny without match, got %#v", response)
	}
}

func TestHandleAdminServiceAccountFirewallRuleConflict(t *testing.T) {
	handler, userService, _ := newServiceAccountsTestHandler(t)
	ctx := context.Background()

	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "worker",
		Name:       "Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	for i, address := range []string{"10.0.0.10", "10.0.0.11"} {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/admin/service-accounts/"+account.ID+"/firewall/rules",
			bytes.NewBufferString(fmt.Sprintf(`{"address":%q,"priority":1,"action":"allow","enabled":true}`, address)),
		)
		req.SetPathValue("id", account.ID)
		recorder := httptest.NewRecorder()

		handler.HandleAdminCreateServiceAccountFirewallRule(recorder, req)

		expected := http.StatusCreated
		if i == 1 {
			expected = http.StatusConflict
		}
		if recorder.Code != expected {
			t.Fatalf("expected %d, got %d: %s", expected, recorder.Code, recorder.Body.String())
		}
	}
}
