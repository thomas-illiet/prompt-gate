package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/platform/secrets"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestService creates an in-memory provider service with a test cipher.
func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	service := NewService(db, cipher)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return service, db
}

// TestHelpSetupLoadsModelsAndRedactsAPIKeys verifies help setup loads models and redacts API keys.
func TestHelpSetupLoadsModelsAndRedactsAPIKeys(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	var authHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"},{"id":"gpt-5.1-codex"}]}`))
	}))
	t.Cleanup(upstream.Close)

	if _, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:        "openai-main",
		DisplayName: "OpenAI Main",
		Type:        ProviderTypeOpenAI,
		BaseURL:     upstream.URL + "/v1",
		APIKey:      "sk-secret",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	setup, err := service.HelpSetup(ctx, "https://proxy.example.com")
	if err != nil {
		t.Fatalf("help setup: %v", err)
	}

	if authHeader != "Bearer sk-secret" {
		t.Fatalf("expected upstream bearer auth, got %q", authHeader)
	}
	if setup.ProxyBaseURL != "https://proxy.example.com" {
		t.Fatalf("unexpected proxy base URL: %q", setup.ProxyBaseURL)
	}
	if len(setup.Providers) != 1 {
		t.Fatalf("expected one provider, got %#v", setup.Providers)
	}
	provider := setup.Providers[0]
	if provider.OpenAIBaseURL != "https://proxy.example.com/openai-main/v1" {
		t.Fatalf("unexpected OpenAI base URL: %q", provider.OpenAIBaseURL)
	}
	if len(provider.Models) != 2 || provider.Models[0] != "gpt-5-mini" || provider.Models[1] != "gpt-5.1-codex" {
		t.Fatalf("unexpected models: %#v", provider.Models)
	}

	raw, err := json.Marshal(setup)
	if err != nil {
		t.Fatalf("marshal setup: %v", err)
	}
	if strings.Contains(string(raw), "sk-secret") {
		t.Fatalf("setup response leaked API key: %s", raw)
	}
}

// TestHelpSetupSkipsAnthropicModelListing verifies help setup skips Anthropic model listing.
func TestHelpSetupSkipsAnthropicModelListing(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	var upstreamRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests++
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)

	if _, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:    "anthropic-main",
		Type:    ProviderTypeAnthropic,
		BaseURL: upstream.URL,
		APIKey:  "sk-ant-secret",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	setup, err := service.HelpSetup(ctx, "https://proxy.example.com")
	if err != nil {
		t.Fatalf("help setup: %v", err)
	}
	if upstreamRequests != 0 {
		t.Fatalf("expected no upstream model requests, got %d", upstreamRequests)
	}
	if len(setup.Providers) != 1 {
		t.Fatalf("expected one provider, got %#v", setup.Providers)
	}
	provider := setup.Providers[0]
	if provider.AnthropicBaseURL != "https://proxy.example.com/anthropic-main" {
		t.Fatalf("unexpected anthropic base URL: %q", provider.AnthropicBaseURL)
	}
	if provider.ModelsError != "" {
		t.Fatalf("expected no model error, got %#v", provider)
	}
	if len(provider.Models) != 0 {
		t.Fatalf("expected no models, got %#v", provider.Models)
	}
}

// TestHelpSetupForProviderNamesFiltersProvidersAndModels verifies help setup for provider names filters providers and models.
func TestHelpSetupForProviderNamesFiltersProvidersAndModels(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	var allowedFetches int
	var deniedFetches int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/allowed/v1/models":
			allowedFetches++
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"},{"id":"gpt-4.1"}]}`))
		case "/denied/v1/models":
			deniedFetches++
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	if _, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:    "openai-allowed",
		Type:    ProviderTypeOpenAI,
		BaseURL: upstream.URL + "/allowed/v1",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create allowed provider: %v", err)
	}
	if _, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:    "openai-denied",
		Type:    ProviderTypeOpenAI,
		BaseURL: upstream.URL + "/denied/v1",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create denied provider: %v", err)
	}

	setup, err := service.HelpSetupForProviderNames(
		ctx,
		"https://proxy.example.com",
		[]string{"openai-allowed"},
		func(providerName, model string) bool {
			return providerName == "openai-allowed" && strings.HasPrefix(model, "gpt-5")
		},
	)
	if err != nil {
		t.Fatalf("help setup: %v", err)
	}

	if allowedFetches != 1 {
		t.Fatalf("expected one allowed provider fetch, got %d", allowedFetches)
	}
	if deniedFetches != 0 {
		t.Fatalf("expected denied provider not to be fetched, got %d", deniedFetches)
	}
	if len(setup.Providers) != 1 {
		t.Fatalf("expected one provider, got %#v", setup.Providers)
	}
	got := setup.Providers[0].Models
	if len(got) != 1 || got[0] != "gpt-5-mini" {
		t.Fatalf("expected filtered models, got %#v", got)
	}
}

// TestModelCatalogSkipsAnthropicModelListing verifies model catalog skips Anthropic model listing.
func TestModelCatalogSkipsAnthropicModelListing(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	var upstreamRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests++
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)

	provider, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:    "anthropic-main",
		Type:    ProviderTypeAnthropic,
		BaseURL: upstream.URL,
		APIKey:  "sk-ant-secret",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	catalog, err := service.ModelCatalog(ctx, []string{provider.ID.String()})
	if err != nil {
		t.Fatalf("model catalog: %v", err)
	}

	if upstreamRequests != 0 {
		t.Fatalf("expected no upstream model requests, got %d", upstreamRequests)
	}
	if len(catalog) != 1 {
		t.Fatalf("expected one catalog provider, got %#v", catalog)
	}
	if catalog[0].ModelsError != "" {
		t.Fatalf("expected no model error, got %#v", catalog[0])
	}
	if len(catalog[0].Models) != 0 {
		t.Fatalf("expected no models, got %#v", catalog[0].Models)
	}
}

// TestCreateProviderEncryptsAPIKeyAndRedactsResponse verifies provider secrets are encrypted and hidden.
func TestCreateProviderEncryptsAPIKeyAndRedactsResponse(t *testing.T) {
	service, db := newTestService(t)
	ctx := context.Background()

	resp, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:    "openai",
		Type:    ProviderTypeOpenAI,
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "sk-secret",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if !resp.HasAPIKey {
		t.Fatal("expected hasApiKey true")
	}

	var record Provider
	if err := db.First(&record, "id = ?", resp.ID).Error; err != nil {
		t.Fatalf("load provider: %v", err)
	}
	if record.APIKeyCiphertext == "" || record.APIKeyCiphertext == "sk-secret" {
		t.Fatalf("expected encrypted api key, got %q", record.APIKeyCiphertext)
	}
	plain, err := service.DecryptAPIKey(record)
	if err != nil {
		t.Fatalf("decrypt api key: %v", err)
	}
	if plain != "sk-secret" {
		t.Fatalf("expected sk-secret, got %q", plain)
	}
}

// TestUpdateProviderPreservesName verifies provider updates cannot rename route identities.
func TestUpdateProviderPreservesName(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	created, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:        "openai-main",
		DisplayName: "OpenAI Main",
		Type:        ProviderTypeOpenAI,
		BaseURL:     "https://api.openai.com/v1",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	displayName := "OpenAI Primary"
	updated, err := service.UpdateProvider(ctx, created.ID.String(), UpdateProviderInput{
		DisplayName: &displayName,
	})
	if err != nil {
		t.Fatalf("update provider: %v", err)
	}

	if updated.Name != "openai-main" {
		t.Fatalf("expected immutable provider name, got %q", updated.Name)
	}
	if updated.DisplayName != displayName {
		t.Fatalf("expected display name %q, got %q", displayName, updated.DisplayName)
	}
}
