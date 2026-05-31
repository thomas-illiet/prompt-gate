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
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func newGroupsTestHandler(t *testing.T) (*Handler, *groups.Service, users.User, provider.Provider, provider.Provider) {
	t.Helper()
	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}, &provider.Provider{}); err != nil {
		t.Fatalf("migrate dependencies: %v", err)
	}
	groupService := groups.NewService(db)
	if err := groupService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate groups: %v", err)
	}

	user := users.User{
		ID:                uuid.NewString(),
		ExternalSub:       "oidc-sub",
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	openai := provider.Provider{
		ID:          uuid.New(),
		Name:        "openai-main",
		DisplayName: "OpenAI Main",
		Type:        provider.ProviderTypeOpenAI,
		BaseURL:     "https://api.openai.com/v1",
		Enabled:     true,
	}
	anthropic := provider.Provider{
		ID:          uuid.New(),
		Name:        "anthropic-main",
		DisplayName: "Anthropic Main",
		Type:        provider.ProviderTypeAnthropic,
		BaseURL:     "https://api.anthropic.com",
		Enabled:     true,
	}
	if err := db.Create(&openai).Error; err != nil {
		t.Fatalf("create openai provider: %v", err)
	}
	if err := db.Create(&anthropic).Error; err != nil {
		t.Fatalf("create anthropic provider: %v", err)
	}

	providerService := provider.NewService(db, nil)
	return NewHandler(nil, nil, nil, groupService, providerService, nil), groupService, user, openai, anthropic
}

func TestHandleAdminGroupsCreateAddMemberAndReplaceUserGroups(t *testing.T) {
	handler, _, user, openai, anthropic := newGroupsTestHandler(t)

	body, err := json.Marshal(groups.CreateGroupInput{
		Name:        "platform",
		DisplayName: "Platform",
		Description: "Two providers with default model access",
		ProviderIDs: []string{openai.ID.String(), anthropic.ID.String()},
	})
	if err != nil {
		t.Fatalf("marshal group: %v", err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups", bytes.NewReader(body))
	createRec := httptest.NewRecorder()
	handler.HandleAdminCreateGroup(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created groups.GroupResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created group: %v", err)
	}
	if created.ProviderCount != 2 || created.ModelPatternCount != 1 {
		t.Fatalf("expected two providers and default model pattern, got %#v", created)
	}
	if len(created.ModelPatterns) != 1 || created.ModelPatterns[0] != ".*" {
		t.Fatalf("expected default all-model pattern, got %#v", created.ModelPatterns)
	}

	addReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/groups/"+created.ID.String()+"/members/"+user.ID, nil)
	addReq.SetPathValue("id", created.ID.String())
	addReq.SetPathValue("userId", user.ID)
	addRec := httptest.NewRecorder()
	handler.HandleAdminAddGroupMember(addRec, addReq)
	if addRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", addRec.Code, addRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+user.ID+"/groups", nil)
	listReq.SetPathValue("id", user.ID)
	listRec := httptest.NewRecorder()
	handler.HandleAdminListUserGroups(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var userGroups []groups.GroupResponse
	if err := json.NewDecoder(listRec.Body).Decode(&userGroups); err != nil {
		t.Fatalf("decode user groups: %v", err)
	}
	if len(userGroups) != 1 || userGroups[0].ID != created.ID {
		t.Fatalf("unexpected user groups: %#v", userGroups)
	}

	replaceBody, err := json.Marshal(groups.ReplaceUserGroupsInput{GroupIDs: []string{}})
	if err != nil {
		t.Fatalf("marshal replacement: %v", err)
	}
	replaceReq := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+user.ID+"/groups", bytes.NewReader(replaceBody))
	replaceReq.SetPathValue("id", user.ID)
	replaceRec := httptest.NewRecorder()
	handler.HandleAdminReplaceUserGroups(replaceRec, replaceReq)
	if replaceRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", replaceRec.Code, replaceRec.Body.String())
	}
	var replaced []groups.GroupResponse
	if err := json.NewDecoder(replaceRec.Body).Decode(&replaced); err != nil {
		t.Fatalf("decode replaced groups: %v", err)
	}
	if len(replaced) != 0 {
		t.Fatalf("expected memberships removed, got %#v", replaced)
	}
}

func TestHandleAdminCreateGroupRejectsInvalidRegex(t *testing.T) {
	handler, _, _, openai, _ := newGroupsTestHandler(t)
	body, err := json.Marshal(groups.CreateGroupInput{
		Name:          "broken",
		DisplayName:   "Broken",
		ProviderIDs:   []string{openai.ID.String()},
		ModelPatterns: []string{"["},
	})
	if err != nil {
		t.Fatalf("marshal group: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdminCreateGroup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_regex") {
		t.Fatalf("expected invalid_regex error, got %s", rec.Body.String())
	}
}

func TestHandleAdminCreateGroupRejectsMissingDisplayNameAndProvider(t *testing.T) {
	handler, _, _, openai, _ := newGroupsTestHandler(t)

	body, err := json.Marshal(groups.CreateGroupInput{
		Name:        "missing-display",
		ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("marshal group: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdminCreateGroup(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_display_name") {
		t.Fatalf("expected invalid_display_name error, got %s", rec.Body.String())
	}

	body, err = json.Marshal(groups.CreateGroupInput{
		Name:        "missing-provider",
		DisplayName: "Missing Provider",
	})
	if err != nil {
		t.Fatalf("marshal group: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.HandleAdminCreateGroup(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "provider_required") {
		t.Fatalf("expected provider_required error, got %s", rec.Body.String())
	}
}

func TestHandleAdminValidateGroupModelPatternsCountsRealMatches(t *testing.T) {
	handler, _, _, _, _ := newGroupsTestHandler(t)
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"},{"id":"gpt-5.1-codex"},{"id":"claude-sonnet-4"}]}`))
	}))
	t.Cleanup(upstream.Close)

	providerRecord, err := handler.providers.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "local-openai",
		Type:    provider.ProviderTypeOpenAI,
		BaseURL: upstream.URL,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	body, err := json.Marshal(groups.ValidateModelPatternsInput{
		ProviderIDs:   []string{providerRecord.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("marshal validation request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups/model-patterns/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdminValidateGroupModelPatterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response groups.ModelPatternValidationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if response.MatchedModelCount != 2 {
		t.Fatalf("expected 2 matched models, got %#v", response)
	}
	if len(response.ProviderResults) != 1 || response.ProviderResults[0].MatchedModelCount != 2 {
		t.Fatalf("unexpected provider results: %#v", response.ProviderResults)
	}
}

func TestHandleAdminValidateGroupModelPatternsMarksDisabledProviderUnavailable(t *testing.T) {
	handler, _, _, _, _ := newGroupsTestHandler(t)
	ctx := context.Background()
	providerRecord, err := handler.providers.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "disabled-openai",
		Type:    provider.ProviderTypeOpenAI,
		BaseURL: "https://disabled.example.com/v1",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	body, err := json.Marshal(groups.ValidateModelPatternsInput{
		ProviderIDs:   []string{providerRecord.ID.String()},
		ModelPatterns: []string{`.*`},
	})
	if err != nil {
		t.Fatalf("marshal validation request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups/model-patterns/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdminValidateGroupModelPatterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response groups.ModelPatternValidationResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if response.MatchedModelCount != 0 || response.UnavailableProviderCount != 1 {
		t.Fatalf("expected disabled provider to be unavailable without matches, got %#v", response)
	}
	if len(response.ProviderResults) != 1 || response.ProviderResults[0].ModelsError == "" {
		t.Fatalf("expected disabled provider result error, got %#v", response.ProviderResults)
	}
}

func TestHandleAdminValidateGroupModelPatternsRejectsInvalidRegex(t *testing.T) {
	handler, _, _, _, _ := newGroupsTestHandler(t)
	body, err := json.Marshal(groups.ValidateModelPatternsInput{
		ModelPatterns: []string{"["},
	})
	if err != nil {
		t.Fatalf("marshal validation request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups/model-patterns/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdminValidateGroupModelPatterns(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_regex") {
		t.Fatalf("expected invalid_regex error, got %s", rec.Body.String())
	}
}
