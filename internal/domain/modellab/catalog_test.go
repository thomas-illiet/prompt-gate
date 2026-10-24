package modellab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

type fakeProviderRepository struct {
	records []provider.Provider
	keys    map[string]string
	err     error
}

func (f *fakeProviderRepository) ListEnabled(context.Context) ([]provider.Provider, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]provider.Provider{}, f.records...), nil
}

func (f *fakeProviderRepository) DecryptAPIKey(record provider.Provider) (string, error) {
	return f.keys[record.Name], nil
}

type fakePricingRepository struct {
	config pricing.ConfigResponse
	err    error
}

func (f *fakePricingRepository) Config(context.Context) (pricing.ConfigResponse, error) {
	return f.config, f.err
}

func newModelLabTestStore(t *testing.T) *redisstore.Store {
	t.Helper()
	server := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+server.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newModelLabTestService(t *testing.T, providers ProviderRepository, prices PricingRepository, store RedisStore, now func() time.Time, client *http.Client) *Service {
	t.Helper()
	service, err := NewService(providers, prices, store, Options{
		TicketSecret:       "0123456789abcdef0123456789abcdef",
		ProxyBaseURL:       "https://proxy.example.test",
		TicketTTL:          time.Minute,
		LeaseTTL:           10 * time.Minute,
		CatalogFreshFor:    5 * time.Minute,
		CatalogLastGoodFor: time.Hour,
		HTTPClient:         client,
		Now:                now,
	})
	if err != nil {
		t.Fatalf("new model lab service: %v", err)
	}
	return service
}

func TestCatalogFiltersAccessAndReportsPriceSource(t *testing.T) {
	var requests atomic.Int64
	var openAIAuth string
	var anthropicKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/openai/v1/models":
			openAIAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5"},{"id":"gpt-4.1"}]}`))
		case "/anthropic/v1/models":
			anthropicKey = r.Header.Get("x-api-key")
			if r.Header.Get("anthropic-version") == "" {
				t.Error("missing anthropic-version header")
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	providers := &fakeProviderRepository{
		records: []provider.Provider{
			{ID: uuid.New(), Name: "anthropic-main", DisplayName: "Anthropic", Type: provider.ProviderTypeAnthropic, BaseURL: upstream.URL + "/anthropic", Enabled: true, UpdatedAt: now},
			{ID: uuid.New(), Name: "openai-main", DisplayName: "OpenAI", Type: provider.ProviderTypeOpenAI, BaseURL: upstream.URL + "/openai/v1", Enabled: true, UpdatedAt: now},
			{ID: uuid.New(), Name: "denied", DisplayName: "Denied", Type: provider.ProviderTypeOpenAI, BaseURL: upstream.URL + "/openai/v1", Enabled: true, UpdatedAt: now},
		},
		keys: map[string]string{"openai-main": "sk-openai", "anthropic-main": "sk-ant"},
	}
	prices := &fakePricingRepository{config: pricing.ConfigResponse{
		Fallback: pricing.PriceRates{Input: 1, Output: 2},
		Models:   []pricing.ModelPriceRecord{{ProviderName: "openai-main", Model: "gpt-5", Input: 3, Output: 4}},
	}}
	service := newModelLabTestService(t, providers, prices, newModelLabTestStore(t), func() time.Time { return now }, upstream.Client())
	access := AccessFilter{
		ProviderAllowed: func(name string) bool { return name != "denied" },
		ModelAllowed: func(providerName, model string) bool {
			return model != "gpt-4.1"
		},
	}

	catalog, err := service.Catalog(context.Background(), access)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(catalog.Providers) != 2 {
		t.Fatalf("expected two allowed providers, got %#v", catalog.Providers)
	}
	if requests.Load() != 2 {
		t.Fatalf("expected two discovery requests, got %d", requests.Load())
	}
	if openAIAuth != "Bearer sk-openai" || anthropicKey != "sk-ant" {
		t.Fatalf("unexpected credentials: openai=%q anthropic=%q", openAIAuth, anthropicKey)
	}

	var openAI CatalogProvider
	for _, item := range catalog.Providers {
		if item.Name == "openai-main" {
			openAI = item
		}
	}
	if openAI.Catalog.State != CatalogFresh || len(openAI.Models) != 1 || openAI.Models[0].ID != "gpt-5" {
		t.Fatalf("unexpected openai catalog: %#v", openAI)
	}
	if openAI.Models[0].Pricing.Source != PriceExact || openAI.Models[0].Pricing.Input == nil || *openAI.Models[0].Pricing.Input != 3 {
		t.Fatalf("unexpected exact pricing: %#v", openAI.Models[0].Pricing)
	}

	// A second call inside the freshness window must use Redis, not the provider.
	if _, err := service.Catalog(context.Background(), access); err != nil {
		t.Fatalf("cached catalog: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("expected cached catalog, got %d discovery requests", requests.Load())
	}
}

func TestCatalogUsesStaleLastKnownGoodAndConfiguredFallback(t *testing.T) {
	var fail atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5"}]}`))
	}))
	t.Cleanup(upstream.Close)

	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	record := provider.Provider{ID: uuid.New(), Name: "openai-main", Type: provider.ProviderTypeOpenAI, BaseURL: upstream.URL + "/v1", UpdatedAt: now}
	providers := &fakeProviderRepository{records: []provider.Provider{record}}
	service := newModelLabTestService(t, providers, nil, newModelLabTestStore(t), func() time.Time { return now }, upstream.Client())

	first, err := service.Catalog(context.Background(), AccessFilter{})
	if err != nil || first.Providers[0].Catalog.State != CatalogFresh {
		t.Fatalf("initial catalog: %#v err=%v", first, err)
	}
	fail.Store(true)
	now = now.Add(10 * time.Minute)
	stale, err := service.Catalog(context.Background(), AccessFilter{})
	if err != nil {
		t.Fatalf("stale catalog: %v", err)
	}
	if stale.Providers[0].Catalog.State != CatalogStale || len(stale.Providers[0].Models) != 1 {
		t.Fatalf("expected stale LKG, got %#v", stale.Providers[0])
	}
	if stale.Providers[0].Models[0].Pricing.Source != PriceUnavailable {
		t.Fatalf("expected unavailable pricing, got %#v", stale.Providers[0].Models[0].Pricing)
	}
	if stale.Providers[0].Models[0].Pricing.Input != nil || stale.Providers[0].Models[0].Pricing.Output != nil {
		t.Fatalf("unavailable pricing must stay unknown, got %#v", stale.Providers[0].Models[0].Pricing)
	}

	record.ID = uuid.New()
	record.Config = provider.ProviderConfig{"modelLabModels": []any{"configured-model"}}
	providers.records = []provider.Provider{record}
	configured, err := service.Catalog(context.Background(), AccessFilter{})
	if err != nil {
		t.Fatalf("configured fallback catalog: %v", err)
	}
	if configured.Providers[0].Catalog.State != CatalogFresh || configured.Providers[0].Models[0].ID != "configured-model" {
		t.Fatalf("unexpected configured fallback: %#v", configured.Providers[0])
	}
}

func TestCatalogReturnsRepositoryError(t *testing.T) {
	service := newModelLabTestService(
		t,
		&fakeProviderRepository{err: errors.New("database unavailable")},
		nil,
		newModelLabTestStore(t),
		time.Now,
		http.DefaultClient,
	)
	if _, err := service.Catalog(context.Background(), AccessFilter{}); err == nil {
		t.Fatal("expected provider repository error")
	}
}

func TestCatalogDiscoveryDoesNotFollowCredentialedRedirects(t *testing.T) {
	var redirectedRequests atomic.Int64
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		redirectedRequests.Add(1)
		if r.Header.Get("x-api-key") != "" || r.Header.Get("Authorization") != "" {
			t.Errorf("redirect target received provider credentials: %#v", r.Header)
		}
	}))
	t.Cleanup(redirectTarget.Close)

	redirectSource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/stolen", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirectSource.Close)

	record := provider.Provider{
		ID:        uuid.New(),
		Name:      "anthropic-redirect",
		Type:      provider.ProviderTypeAnthropic,
		BaseURL:   redirectSource.URL,
		Enabled:   true,
		UpdatedAt: time.Now().UTC(),
	}
	service := newModelLabTestService(
		t,
		&fakeProviderRepository{
			records: []provider.Provider{record},
			keys:    map[string]string{record.Name: "provider-secret-canary"},
		},
		nil,
		newModelLabTestStore(t),
		time.Now,
		redirectSource.Client(),
	)

	if _, err := service.fetchProviderModels(context.Background(), record); err == nil {
		t.Fatal("expected credentialed catalog redirect to be rejected")
	}
	if redirectedRequests.Load() != 0 {
		t.Fatalf("redirect target was contacted %d times", redirectedRequests.Load())
	}
}
