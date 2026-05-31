package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/secrets"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testUserProfile() auth.UserProfile {
	return auth.UserProfile{
		ID:                "11111111-1111-1111-1111-111111111111",
		Sub:               "sub",
		PreferredUsername: "user",
		Email:             "user@example.com",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC),
	}
}

func TestHandleCurrentUserUsageRejectsInvalidWindow(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/usage?days=14", nil)
	req = req.WithContext(auth.ContextWithUser(context.Background(), testUserProfile()))
	recorder := httptest.NewRecorder()

	server{}.handleCurrentUserUsage(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "invalid_usage_window" {
		t.Fatalf("expected invalid_usage_window, got %#v", body)
	}
}

func TestParseUsageWindowAcceptsDashboardWindowsAndLegacyDays(t *testing.T) {
	for _, test := range []struct {
		path string
		want proxy.UsageWindow
	}{
		{path: "/api/v1/me/dashboard/tokens", want: proxy.UsageWindow30Days},
		{path: "/api/v1/me/dashboard/tokens?window=7d", want: proxy.UsageWindow7Days},
		{path: "/api/v1/me/dashboard/tokens?window=30d", want: proxy.UsageWindow30Days},
		{path: "/api/v1/me/dashboard/tokens?window=all", want: proxy.UsageWindowAll},
		{path: "/api/v1/me/dashboard/tokens?days=7", want: proxy.UsageWindow7Days},
	} {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		got, err := parseUsageWindow(req)
		if err != nil {
			t.Fatalf("parse %s: %v", test.path, err)
		}
		if got != test.want {
			t.Fatalf("parse %s: got %q want %q", test.path, got, test.want)
		}
	}
}

func TestParseUsageWindowRejectsInvalidWindow(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/dashboard/tokens?window=14d", nil)
	if _, err := parseUsageWindow(req); err == nil {
		t.Fatal("expected invalid usage window")
	}
}

func TestHandleCurrentUserGroupsReturnsMemberships(t *testing.T) {
	groupService, db := newHTTPGroupService(t)
	profile := testUserProfile()
	ctx := context.Background()
	user := users.User{
		ID:                profile.ID,
		ExternalSub:       profile.Sub,
		Email:             profile.Email,
		PreferredUsername: profile.PreferredUsername,
		Name:              profile.Name,
		Type:              profile.Type,
		Role:              profile.Role,
		IsActive:          profile.IsActive,
		LastLoginAt:       profile.LastLoginAt,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	group, err := groupService.CreateGroup(ctx, groups.CreateGroupInput{
		Name:        "engineering",
		DisplayName: "Engineering",
		Description: "Engineering model access",
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := groupService.AddMember(ctx, group.ID.String(), profile.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/groups", nil)
	req = req.WithContext(auth.ContextWithUser(context.Background(), profile))
	recorder := httptest.NewRecorder()

	server{groups: groupService}.handleCurrentUserGroups(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response []groups.GroupResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("expected one group, got %#v", response)
	}
	if response[0].Name != "engineering" || response[0].Description != "Engineering model access" {
		t.Fatalf("unexpected group response: %#v", response[0])
	}
}

func TestHandleCurrentUserPromptsRejectsInvalidPagination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/prompts?page=0", nil)
	req = req.WithContext(auth.ContextWithUser(context.Background(), testUserProfile()))
	recorder := httptest.NewRecorder()

	server{}.handleCurrentUserPrompts(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "invalid_pagination" {
		t.Fatalf("expected invalid_pagination, got %#v", body)
	}
}

func TestHandleHelpSetupReturnsRedactedProviderMetadata(t *testing.T) {
	providerService := newHTTPProviderService(t)
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"}]}`))
	}))
	t.Cleanup(upstream.Close)

	if _, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "openai-main",
		Type:    provider.ProviderTypeOpenAI,
		BaseURL: upstream.URL + "/v1",
		APIKey:  "sk-secret",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/help/setup", nil)
	req = req.WithContext(auth.ContextWithUser(context.Background(), testUserProfile()))
	recorder := httptest.NewRecorder()

	server{
		config: config.Config{
			ProxyBaseURL: "https://proxy.example.com",
		},
		providers: providerService,
	}.handleHelpSetup(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var body provider.HelpSetupResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.ProxyBaseURL != "https://proxy.example.com" {
		t.Fatalf("unexpected proxy base URL: %q", body.ProxyBaseURL)
	}
	if len(body.Providers) != 1 {
		t.Fatalf("expected one provider, got %#v", body.Providers)
	}
	if body.Providers[0].OpenAIBaseURL != "https://proxy.example.com/openai-main/v1" {
		t.Fatalf("unexpected OpenAI base URL: %q", body.Providers[0].OpenAIBaseURL)
	}
	if len(body.Providers[0].Models) != 1 || body.Providers[0].Models[0] != "gpt-5-mini" {
		t.Fatalf("unexpected models: %#v", body.Providers[0].Models)
	}
	if strings.Contains(recorder.Body.String(), "sk-secret") {
		t.Fatalf("response leaked provider API key: %s", recorder.Body.String())
	}
}

func newHTTPProviderService(t *testing.T) *provider.Service {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	service := provider.NewService(db, cipher)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate providers: %v", err)
	}
	return service
}

func newHTTPGroupService(t *testing.T) (*groups.Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-groups?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}, &provider.Provider{}); err != nil {
		t.Fatalf("migrate group dependencies: %v", err)
	}
	service := groups.NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate groups: %v", err)
	}
	return service, db
}
