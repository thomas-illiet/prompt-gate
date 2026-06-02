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
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// testUserProfile returns user profile.
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

// TestHandleCurrentUserUsageRejectsInvalidWindow verifies handle current user usage rejects invalid window.
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

// TestParseUsageWindowAcceptsDashboardWindowsAndLegacyDays verifies parse usage window accepts dashboard windows and legacy days.
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

// TestParseUsageWindowRejectsInvalidWindow verifies parse usage window rejects invalid window.
func TestParseUsageWindowRejectsInvalidWindow(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/dashboard/tokens?window=14d", nil)
	if _, err := parseUsageWindow(req); err == nil {
		t.Fatal("expected invalid usage window")
	}
}

// TestHandleCurrentUserGroupsReturnsProfileSafeMemberships verifies handle current user groups returns profile safe memberships.
func TestHandleCurrentUserGroupsReturnsProfileSafeMemberships(t *testing.T) {
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
	providerRecord := provider.Provider{
		ID:          uuid.New(),
		Name:        "openai-main",
		DisplayName: "OpenAI Main",
		Type:        provider.ProviderTypeOpenAI,
		BaseURL:     "https://api.openai.com/v1",
		Enabled:     true,
	}
	if err := db.Create(&providerRecord).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	group, err := groupService.CreateGroup(ctx, groups.CreateGroupInput{
		Name:        "engineering",
		DisplayName: "Engineering",
		Description: "Engineering model access",
		ProviderIDs: []string{providerRecord.ID.String()},
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
	raw := recorder.Body.String()
	for _, field := range []string{"providers", "modelPatterns", "excludedModelPatterns", "members"} {
		if strings.Contains(raw, field) {
			t.Fatalf("profile group response leaked admin field %q: %s", field, raw)
		}
	}
	var response []groups.ProfileGroupResponse
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

// TestHandleCurrentUserPromptsRejectsInvalidPagination verifies handle current user prompts rejects invalid pagination.
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

// TestHandleHelpSetupReturnsUserScopedRedactedProviderMetadata verifies handle help setup returns user scoped redacted provider metadata.
func TestHandleHelpSetupReturnsUserScopedRedactedProviderMetadata(t *testing.T) {
	providerService, groupService, db := newHTTPSetupServices(t)
	ctx := context.Background()
	profile := testUserProfile()
	createHTTPUser(t, db, profile)
	var openAIRequests int
	var anthropicRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/openai/v1/models":
			openAIRequests++
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"},{"id":"gpt-4.1"}]}`))
		case "/anthropic/v1/models":
			anthropicRequests++
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-4"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	openAIProvider, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "openai-main",
		Type:    provider.ProviderTypeOpenAI,
		BaseURL: upstream.URL + "/openai/v1",
		APIKey:  "sk-secret",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "anthropic-main",
		Type:    provider.ProviderTypeAnthropic,
		BaseURL: upstream.URL + "/anthropic",
		APIKey:  "sk-ant-secret",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create denied provider: %v", err)
	}
	group, err := groupService.CreateGroup(ctx, groups.CreateGroupInput{
		Name:          "engineering",
		DisplayName:   "Engineering",
		ProviderIDs:   []string{openAIProvider.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := groupService.AddMember(ctx, group.ID.String(), profile.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/help/setup", nil)
	req = req.WithContext(auth.ContextWithUser(context.Background(), profile))
	recorder := httptest.NewRecorder()

	server{
		config: config.Config{
			ProxyBaseURL: "https://proxy.example.com",
		},
		providers: providerService,
		groups:    groupService,
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
	if openAIRequests != 1 {
		t.Fatalf("expected one authorized provider fetch, got %d", openAIRequests)
	}
	if anthropicRequests != 0 {
		t.Fatalf("expected unauthorized provider not to be fetched, got %d", anthropicRequests)
	}
	if len(body.Providers) != 1 {
		t.Fatalf("expected one provider, got %#v", body.Providers)
	}
	if body.Providers[0].Name != "openai-main" {
		t.Fatalf("unexpected provider: %#v", body.Providers[0])
	}
	if body.Providers[0].OpenAIBaseURL != "https://proxy.example.com/openai-main/v1" {
		t.Fatalf("unexpected OpenAI base URL: %q", body.Providers[0].OpenAIBaseURL)
	}
	if len(body.Providers[0].Models) != 1 || body.Providers[0].Models[0] != "gpt-5-mini" {
		t.Fatalf("unexpected models: %#v", body.Providers[0].Models)
	}
	if strings.Contains(recorder.Body.String(), "sk-secret") || strings.Contains(recorder.Body.String(), "sk-ant-secret") {
		t.Fatalf("response leaked provider API key: %s", recorder.Body.String())
	}
}

// TestHandleHelpSetupReturnsAnthropicWithoutModelFetch verifies handle help setup returns Anthropic without model fetch.
func TestHandleHelpSetupReturnsAnthropicWithoutModelFetch(t *testing.T) {
	providerService, groupService, db := newHTTPSetupServices(t)
	ctx := context.Background()
	profile := testUserProfile()
	createHTTPUser(t, db, profile)
	var upstreamRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests++
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)

	anthropicProvider, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "anthropic-main",
		Type:    provider.ProviderTypeAnthropic,
		BaseURL: upstream.URL + "/anthropic",
		APIKey:  "sk-ant-secret",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	group, err := groupService.CreateGroup(ctx, groups.CreateGroupInput{
		Name:        "engineering",
		DisplayName: "Engineering",
		ProviderIDs: []string{anthropicProvider.ID.String()},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := groupService.AddMember(ctx, group.ID.String(), profile.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/help/setup", nil)
	req = req.WithContext(auth.ContextWithUser(context.Background(), profile))
	recorder := httptest.NewRecorder()

	server{
		config: config.Config{
			ProxyBaseURL: "https://proxy.example.com",
		},
		providers: providerService,
		groups:    groupService,
	}.handleHelpSetup(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if upstreamRequests != 0 {
		t.Fatalf("expected no upstream model requests, got %d", upstreamRequests)
	}

	var body provider.HelpSetupResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Providers) != 1 {
		t.Fatalf("expected one provider, got %#v", body.Providers)
	}
	got := body.Providers[0]
	if got.Name != "anthropic-main" {
		t.Fatalf("unexpected provider: %#v", got)
	}
	if got.AnthropicBaseURL != "https://proxy.example.com/anthropic-main" {
		t.Fatalf("unexpected Anthropic base URL: %q", got.AnthropicBaseURL)
	}
	if got.ModelsError != "" {
		t.Fatalf("expected no models error, got %#v", got)
	}
	if len(got.Models) != 0 {
		t.Fatalf("expected no models, got %#v", got.Models)
	}
	if strings.Contains(recorder.Body.String(), "sk-ant-secret") {
		t.Fatalf("response leaked provider API key: %s", recorder.Body.String())
	}
}

// TestHandleHelpSetupReturnsEmptyProvidersWithoutGroupAccess verifies handle help setup returns empty providers without group access.
func TestHandleHelpSetupReturnsEmptyProvidersWithoutGroupAccess(t *testing.T) {
	providerService, groupService, db := newHTTPSetupServices(t)
	ctx := context.Background()
	profile := testUserProfile()
	createHTTPUser(t, db, profile)
	var upstreamRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"}]}`))
	}))
	t.Cleanup(upstream.Close)

	if _, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:    "openai-main",
		Type:    provider.ProviderTypeOpenAI,
		BaseURL: upstream.URL + "/v1",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/help/setup", nil)
	req = req.WithContext(auth.ContextWithUser(context.Background(), profile))
	recorder := httptest.NewRecorder()

	server{
		config: config.Config{
			ProxyBaseURL: "https://proxy.example.com",
		},
		providers: providerService,
		groups:    groupService,
	}.handleHelpSetup(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if upstreamRequests != 0 {
		t.Fatalf("expected no upstream model requests, got %d", upstreamRequests)
	}
	var body provider.HelpSetupResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Providers) != 0 {
		t.Fatalf("expected no scoped providers, got %#v", body.Providers)
	}
}

// newHTTPSetupServices creates HTTP setup services.
func newHTTPSetupServices(t *testing.T) (*provider.Service, *groups.Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	providerService := provider.NewService(db, cipher)
	if err := db.AutoMigrate(&users.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	if err := providerService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate providers: %v", err)
	}
	groupService := groups.NewService(db)
	if err := groupService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate groups: %v", err)
	}
	return providerService, groupService, db
}

// createHTTPUser creates HTTP user.
func createHTTPUser(t *testing.T, db *gorm.DB, profile auth.UserProfile) {
	t.Helper()

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
}

// newHTTPGroupService creates HTTP group service.
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
