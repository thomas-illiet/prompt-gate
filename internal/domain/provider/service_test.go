package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/secrets"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// newTestService creates an in-memory provider service with a test cipher.
func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
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

type recordingNotifier struct {
	domains []string
}

func (n *recordingNotifier) Notify(_ context.Context, domain string) {
	n.domains = append(n.domains, domain)
}

func TestProviderCRUDListingAndNotifications(t *testing.T) {
	service, db := newTestService(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	service.SetNotifier(notifier)
	service.SetModelHTTPClient(nil)

	alpha, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:        "  OPENAI-MAIN  ",
		DisplayName: " OpenAI Main ",
		Type:        ProviderTypeOpenAI,
		BaseURL:     "https://api.openai.example/v1/",
		APIKey:      " first-secret ",
		Config:      ProviderConfig{"models": []interface{}{"gpt-5"}},
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create first provider: %v", err)
	}
	if alpha.Name != "openai-main" || alpha.DisplayName != "OpenAI Main" || alpha.BaseURL != "https://api.openai.example/v1" {
		t.Fatalf("provider was not normalized: %#v", alpha)
	}

	beta, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:        "ollama-local",
		DisplayName: "Ollama Local",
		Type:        ProviderTypeOllama,
		BaseURL:     "http://ollama.example:11434",
		Enabled:     false,
	})
	if err != nil {
		t.Fatalf("create second provider: %v", err)
	}
	_, err = service.CreateProvider(ctx, CreateProviderInput{
		Name:        "anthropic-main",
		DisplayName: "Anthropic Main",
		Type:        ProviderTypeAnthropic,
		BaseURL:     "https://api.anthropic.example",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create third provider: %v", err)
	}
	disabled := false
	if _, err := service.UpdateProvider(ctx, beta.ID.String(), UpdateProviderInput{Enabled: &disabled}); err != nil {
		t.Fatalf("disable second provider: %v", err)
	}

	for name, want := range map[string]bool{
		"":               false,
		"missing":        false,
		"openai-main":    true,
		"  openai-main ": true,
	} {
		got, existsErr := service.ProviderNameExists(ctx, name)
		if existsErr != nil {
			t.Fatalf("provider name exists %q: %v", name, existsErr)
		}
		if got != want {
			t.Fatalf("provider name exists %q = %v, want %v", name, got, want)
		}
	}

	all, err := service.ListProviders(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if got := []string{all[0].Name, all[1].Name, all[2].Name}; !reflect.DeepEqual(got, []string{"anthropic-main", "ollama-local", "openai-main"}) {
		t.Fatalf("unexpected provider order: %#v", got)
	}

	page, err := service.ListProvidersPaged(ctx, ListParams{Page: 1, PageSize: 2, SortBy: "displayName", SortDir: "desc"})
	if err != nil {
		t.Fatalf("list provider page: %v", err)
	}
	if page.Total != 3 || page.Page != 1 || page.PageSize != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected page metadata: %#v", page)
	}
	if page.Items[0].Name != "openai-main" || page.Items[1].Name != "ollama-local" {
		t.Fatalf("unexpected display-name ordering: %#v", page.Items)
	}

	enabled, err := service.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("list enabled providers: %v", err)
	}
	if got := []string{enabled[0].Name, enabled[1].Name}; !reflect.DeepEqual(got, []string{"anthropic-main", "openai-main"}) {
		t.Fatalf("unexpected enabled providers: %#v", got)
	}
	if empty, emptyErr := service.ListEnabledByNames(ctx, []string{"", "  "}); emptyErr != nil || len(empty) != 0 {
		t.Fatalf("empty named provider lookup = %#v, %v", empty, emptyErr)
	}
	selected, err := service.ListEnabledByNames(ctx, []string{" OPENAI-MAIN ", "openai-main", "ollama-local", "anthropic-main"})
	if err != nil {
		t.Fatalf("list enabled providers by name: %v", err)
	}
	if got := []string{selected[0].Name, selected[1].Name}; !reflect.DeepEqual(got, []string{"anthropic-main", "openai-main"}) {
		t.Fatalf("unexpected selected providers: %#v", got)
	}

	displayName := " Updated Ollama "
	providerType := ProviderTypeOllama
	baseURL := "http://new-ollama.example:11434/"
	apiKey := " replacement-secret "
	config := ProviderConfig{"keep_alive": "5m"}
	enabledValue := false
	updated, err := service.UpdateProvider(ctx, alpha.ID.String(), UpdateProviderInput{
		DisplayName: &displayName,
		Type:        &providerType,
		BaseURL:     &baseURL,
		APIKey:      &apiKey,
		Config:      &config,
		Enabled:     &enabledValue,
	})
	if err != nil {
		t.Fatalf("update provider: %v", err)
	}
	if updated.Name != alpha.Name || updated.DisplayName != "Updated Ollama" || updated.Type != ProviderTypeOllama || updated.BaseURL != "http://new-ollama.example:11434" || updated.Enabled || !updated.HasAPIKey {
		t.Fatalf("unexpected updated provider: %#v", updated)
	}
	loaded, err := service.GetProvider(ctx, alpha.ID.String())
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	if !reflect.DeepEqual(loaded.Config, config) {
		t.Fatalf("unexpected provider config: %#v", loaded.Config)
	}
	var stored Provider
	if err := db.First(&stored, "id = ?", alpha.ID).Error; err != nil {
		t.Fatalf("load updated provider: %v", err)
	}
	decrypted, err := service.DecryptAPIKey(stored)
	if err != nil || decrypted != "replacement-secret" {
		t.Fatalf("decrypt replacement key = %q, %v", decrypted, err)
	}

	nilConfig := ProviderConfig(nil)
	updated, err = service.UpdateProvider(ctx, alpha.ID.String(), UpdateProviderInput{Config: &nilConfig})
	if err != nil {
		t.Fatalf("clear provider config: %v", err)
	}
	if updated.Config == nil || len(updated.Config) != 0 {
		t.Fatalf("expected normalized empty config, got %#v", updated.Config)
	}

	if err := service.DeleteProvider(ctx, beta.ID.String()); err != nil {
		t.Fatalf("delete provider: %v", err)
	}
	if _, err := service.GetProvider(ctx, beta.ID.String()); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("get deleted provider error = %v", err)
	}
	if err := service.DeleteProvider(ctx, beta.ID.String()); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("delete missing provider error = %v", err)
	}

	wantNotifications := []string{
		configevents.DomainProviders, configevents.DomainGroups,
		configevents.DomainProviders, configevents.DomainGroups,
		configevents.DomainProviders, configevents.DomainGroups,
		configevents.DomainProviders, configevents.DomainGroups,
		configevents.DomainProviders, configevents.DomainGroups,
		configevents.DomainProviders, configevents.DomainGroups,
		configevents.DomainProviders, configevents.DomainGroups,
	}
	if !reflect.DeepEqual(notifier.domains, wantNotifications) {
		t.Fatalf("unexpected config notifications: %#v", notifier.domains)
	}
	service.SetNotifier(nil)
}

func TestProviderValidationSortingAndFailureCases(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	for name, input := range map[string]CreateProviderInput{
		"invalid name": {Name: "bad/name", Type: ProviderTypeOpenAI, BaseURL: "https://example.com"},
		"invalid type": {Name: "valid-name", Type: ProviderType("unknown"), BaseURL: "https://example.com"},
		"empty URL":    {Name: "valid-name", Type: ProviderTypeOpenAI},
		"relative URL": {Name: "valid-name", Type: ProviderTypeOpenAI, BaseURL: "/v1"},
		"invalid scheme": {Name: "valid-name", Type: ProviderTypeOpenAI,
			BaseURL: "ftp://example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.CreateProvider(ctx, input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	created, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:    "duplicate",
		Type:    ProviderTypeOpenAI,
		BaseURL: "https://example.com/v1",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := service.CreateProvider(ctx, CreateProviderInput{
		Name:    "duplicate",
		Type:    ProviderTypeOpenAI,
		BaseURL: "https://example.com/v1",
	}); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("duplicate provider error = %v", err)
	}

	invalidType := ProviderType("unknown")
	if _, err := service.UpdateProvider(ctx, created.ID.String(), UpdateProviderInput{Type: &invalidType}); !errors.Is(err, ErrInvalidType) {
		t.Fatalf("invalid update type error = %v", err)
	}
	invalidURL := "mailto:test@example.com"
	if _, err := service.UpdateProvider(ctx, created.ID.String(), UpdateProviderInput{BaseURL: &invalidURL}); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("invalid update URL error = %v", err)
	}
	if _, err := service.UpdateProvider(ctx, "missing", UpdateProviderInput{}); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("missing update error = %v", err)
	}
	if _, err := service.GetProvider(ctx, "missing"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("missing get error = %v", err)
	}

	for _, params := range []ListParams{
		{SortBy: "unknown", SortDir: "asc"},
		{SortBy: "name", SortDir: "sideways"},
	} {
		if _, err := service.ListProvidersPaged(ctx, params); !errors.Is(err, ErrInvalidSort) {
			t.Fatalf("invalid sort %#v error = %v", params, err)
		}
	}

	params := ListParams{Page: -1, PageSize: 500}
	normalizeListParams(&params)
	if params.Page != 1 || params.PageSize != 100 || params.SortBy != "name" || params.SortDir != "asc" {
		t.Fatalf("unexpected normalized params: %#v", params)
	}
	if got, err := normalizeSortDir(" DESC "); err != nil || got != "DESC" {
		t.Fatalf("normalize descending sort = %q, %v", got, err)
	}
	if got := normalizeProviderNames([]string{" Beta ", "alpha", "BETA", ""}); !reflect.DeepEqual(got, []string{"beta", "alpha"}) {
		t.Fatalf("unexpected normalized names: %#v", got)
	}
	if encrypted, err := service.encryptOptional("   "); err != nil || encrypted != "" {
		t.Fatalf("encrypt empty optional = %q, %v", encrypted, err)
	}
	response := (&Provider{Name: "empty-config"}).toResponse()
	if response.Config == nil || len(response.Config) != 0 {
		t.Fatalf("expected non-nil empty response config, got %#v", response.Config)
	}
}
