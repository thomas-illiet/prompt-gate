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
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newFirewallTestHandler creates an admin handler wired to an in-memory firewall service.
func newFirewallTestHandler(t *testing.T) (*Handler, *firewall.Service) {
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

	service := firewall.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("auto-migrate firewall table: %v", err)
	}

	return NewHandler(nil, nil, service, nil, nil, nil), service
}

// TestHandleAdminCreateFirewallRuleRejectsInvalidBody verifies malformed JSON is rejected.
func TestHandleAdminCreateFirewallRuleRejectsInvalidBody(t *testing.T) {
	handler, _ := newFirewallTestHandler(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/firewall/rules",
		bytes.NewBufferString(`{"address":"192.168.1.10","priority":1,"action":"allow","enabled":true,"extra":true}`),
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminCreateFirewallRule(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

// TestHandleAdminGetFirewallRuleReturnsNotFound verifies missing firewall rules return 404.
func TestHandleAdminGetFirewallRuleReturnsNotFound(t *testing.T) {
	handler, _ := newFirewallTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/firewall/rules/missing-id", nil)
	req.SetPathValue("id", "missing-id")
	recorder := httptest.NewRecorder()

	handler.HandleAdminGetFirewallRule(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

// TestHandleAdminCreateFirewallRuleReturnsPriorityConflict verifies duplicate priorities return a conflict.
func TestHandleAdminCreateFirewallRuleReturnsPriorityConflict(t *testing.T) {
	handler, service := newFirewallTestHandler(t)
	ctx := context.Background()

	if _, err := service.CreateRule(ctx, firewall.CreateRuleInput{
		Address:  "192.168.1.10",
		Priority: 1,
		Action:   firewall.ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed firewall rule: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/firewall/rules",
		bytes.NewBufferString(`{"address":"192.168.1.11","priority":1,"action":"deny","enabled":true}`),
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminCreateFirewallRule(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", recorder.Code)
	}
}

// TestHandleAdminUpdateFirewallRulePersistsDescription verifies descriptions can be edited or cleared.
func TestHandleAdminUpdateFirewallRulePersistsDescription(t *testing.T) {
	handler, service := newFirewallTestHandler(t)
	ctx := context.Background()

	created, err := service.CreateRule(ctx, firewall.CreateRuleInput{
		Address:     "192.168.1.10",
		Description: "Office gateway",
		Priority:    1,
		Action:      firewall.ActionAllow,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("seed firewall rule: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/firewall/rules/"+created.ID,
		bytes.NewBufferString(`{"description":" Updated office gateway "}`),
	)
	req.SetPathValue("id", created.ID)
	recorder := httptest.NewRecorder()

	handler.HandleAdminUpdateFirewallRule(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var response firewall.RuleResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Description != "Updated office gateway" {
		t.Fatalf("expected trimmed description, got %q", response.Description)
	}
}

// TestHandleAdminSimulateFirewallRuleUsesCIDRRules verifies simulations use service matching logic.
func TestHandleAdminSimulateFirewallRuleUsesCIDRRules(t *testing.T) {
	handler, service := newFirewallTestHandler(t)
	ctx := context.Background()

	if _, err := service.CreateRule(ctx, firewall.CreateRuleInput{
		Address:  "10.0.0.10",
		Priority: 1,
		Action:   firewall.ActionAllow,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed exact allow rule: %v", err)
	}
	if _, err := service.CreateRule(ctx, firewall.CreateRuleInput{
		Address:  "10.0.0.0/24",
		Priority: 2,
		Action:   firewall.ActionDeny,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("seed cidr deny rule: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/firewall/simulate",
		bytes.NewBufferString(`{"clientIp":"10.0.0.25"}`),
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminSimulateFirewallRule(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var response struct {
		Allowed     bool                   `json:"allowed"`
		MatchedRule *firewall.RuleResponse `json:"matchedRule"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Allowed {
		t.Fatal("expected simulated IP to be denied")
	}
	if response.MatchedRule == nil || response.MatchedRule.Address != "10.0.0.0/24" {
		t.Fatalf("expected cidr rule match, got %#v", response.MatchedRule)
	}
}

// TestHandleAdminSimulateFirewallRuleRejectsInvalidIP verifies invalid simulation input returns 400.
func TestHandleAdminSimulateFirewallRuleRejectsInvalidIP(t *testing.T) {
	handler, _ := newFirewallTestHandler(t)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/firewall/simulate",
		bytes.NewBufferString(`{"clientIp":"not-an-ip"}`),
	)
	recorder := httptest.NewRecorder()

	handler.HandleAdminSimulateFirewallRule(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}
