package modellab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
)

const maxCatalogResponseBytes = 2 << 20

type cachedCatalog struct {
	Models    []string  `json:"models"`
	FetchedAt time.Time `json:"fetchedAt"`
}

type providerCatalogResult struct {
	provider provider.Provider
	catalog  cachedCatalog
	state    CatalogState
}

func (s *Service) Catalog(ctx context.Context, access AccessFilter) (CatalogResponse, error) {
	records, err := s.providers.ListEnabled(ctx)
	if err != nil {
		return CatalogResponse{}, fmt.Errorf("list model lab providers: %w", err)
	}

	filtered := make([]provider.Provider, 0, len(records))
	for _, record := range records {
		if access.allowsProvider(record.Name) {
			filtered = append(filtered, record)
		}
	}

	results := make([]providerCatalogResult, len(filtered))
	semaphore := make(chan struct{}, s.catalogConcurrency)
	var wg sync.WaitGroup
	for i := range filtered {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[index] = providerCatalogResult{provider: filtered[index], state: CatalogUnavailable}
				return
			}
			catalog, state := s.catalogForProvider(ctx, filtered[index])
			results[index] = providerCatalogResult{provider: filtered[index], catalog: catalog, state: state}
		}(i)
	}
	wg.Wait()

	priceConfig, priceErr := s.priceConfig(ctx)
	providers := make([]CatalogProvider, 0, len(results))
	for _, result := range results {
		protocol, supported := protocolForProvider(result.provider.Type)
		models := make([]CatalogModel, 0, len(result.catalog.Models))
		for _, modelID := range result.catalog.Models {
			if !access.allowsModel(result.provider.Name, modelID) {
				continue
			}
			models = append(models, CatalogModel{
				ID:          modelID,
				DisplayName: nil,
				Pricing:     modelPricing(priceConfig, priceErr, result.provider.Name, modelID),
				Playground: PlaygroundCapability{
					Supported: supported,
					Protocol:  protocol,
				},
				ObservedAvailability: ObservedAvailability{
					State:         AvailabilityUnknown,
					WindowSeconds: int64(time.Hour / time.Second),
				},
			})
		}

		var fetchedAt *time.Time
		if !result.catalog.FetchedAt.IsZero() {
			value := result.catalog.FetchedAt.UTC()
			fetchedAt = &value
		}
		providers = append(providers, CatalogProvider{
			Name:        result.provider.Name,
			DisplayName: result.provider.DisplayName,
			Type:        result.provider.Type,
			Catalog:     CatalogStatus{State: result.state, FetchedAt: fetchedAt},
			Models:      models,
		})
	}

	return CatalogResponse{
		GeneratedAt: s.now().UTC(),
		Limits: ModelLabLimits{
			MaxTargets:      s.maxTargets,
			MaxPromptBytes:  s.maxPromptBytes,
			MaxOutputTokens: s.maxOutputTokens,
		},
		Providers: providers,
	}, nil
}

func (s *Service) catalogForProvider(ctx context.Context, record provider.Provider) (cachedCatalog, CatalogState) {
	key := catalogCacheKey(record)
	var cached cachedCatalog
	hasCached := false
	if s.store != nil && s.store.Enabled() {
		found, err := s.store.GetJSON(ctx, key, &cached)
		if err == nil && found && !cached.FetchedAt.IsZero() {
			cached.Models = normalizedModels(cached.Models)
			hasCached = true
			if s.now().Sub(cached.FetchedAt) <= s.catalogFreshFor {
				return cached, CatalogFresh
			}
		}
	}

	fresh, err := s.discoverWithFlight(ctx, record, key)
	if err == nil {
		return fresh, CatalogFresh
	}
	if configured := configuredModels(record); len(configured) > 0 {
		return cachedCatalog{Models: configured, FetchedAt: s.now().UTC()}, CatalogFresh
	}
	if hasCached {
		return cached, CatalogStale
	}
	return cachedCatalog{Models: []string{}}, CatalogUnavailable
}

func (s *Service) discoverWithFlight(ctx context.Context, record provider.Provider, key string) (cachedCatalog, error) {
	s.flightsMu.Lock()
	if current, ok := s.flights[key]; ok {
		s.flightsMu.Unlock()
		select {
		case <-current.done:
			return current.result, current.err
		case <-ctx.Done():
			return cachedCatalog{}, ctx.Err()
		}
	}
	flight := &catalogFlight{done: make(chan struct{})}
	s.flights[key] = flight
	s.flightsMu.Unlock()

	flight.result.Models, flight.err = s.fetchProviderModels(ctx, record)
	if flight.err == nil {
		flight.result.FetchedAt = s.now().UTC()
		if s.store != nil && s.store.Enabled() {
			_ = s.store.SetJSON(ctx, key, flight.result, s.catalogLastGoodFor)
		}
	}

	s.flightsMu.Lock()
	delete(s.flights, key)
	close(flight.done)
	s.flightsMu.Unlock()
	return flight.result, flight.err
}

func (s *Service) fetchProviderModels(ctx context.Context, record provider.Provider) ([]string, error) {
	endpoint, err := modelCatalogEndpoint(record)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build model catalog request: %w", err)
	}
	key, err := s.providers.DecryptAPIKey(record)
	if err != nil {
		return nil, fmt.Errorf("decrypt provider credential: %w", err)
	}
	if key != "" {
		if record.Type == provider.ProviderTypeAnthropic {
			req.Header.Set("x-api-key", key)
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	if record.Type == provider.ProviderTypeAnthropic {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	client := *s.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch provider model catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("provider model catalog returned status %d", resp.StatusCode)
	}

	var payload struct {
		Data   []modelPayload `json:"data"`
		Models []modelPayload `json:"models"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxCatalogResponseBytes))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode provider model catalog: %w", err)
	}
	models := make([]string, 0, len(payload.Data)+len(payload.Models))
	for _, item := range append(payload.Data, payload.Models...) {
		models = append(models, firstNonEmpty(item.ID, item.Name, item.DisplayName))
	}
	return normalizedModels(models), nil
}

type modelPayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func modelCatalogEndpoint(record provider.Provider) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(record.BaseURL), "/")
	if base == "" {
		return "", provider.ErrInvalidURL
	}
	if record.Type == provider.ProviderTypeAnthropic && !strings.HasSuffix(base, "/v1") {
		return base + "/v1/models", nil
	}
	return base + "/models", nil
}

func protocolForProvider(providerType provider.ProviderType) (Protocol, bool) {
	switch providerType {
	case provider.ProviderTypeOpenAI, provider.ProviderTypeOllama:
		return ProtocolOpenAIChat, true
	case provider.ProviderTypeAnthropic:
		return ProtocolAnthropicMessages, true
	default:
		return "", false
	}
}

func catalogCacheKey(record provider.Provider) string {
	return fmt.Sprintf("promptgate:model-lab:catalog:v1:%s:%d", record.ID.String(), record.UpdatedAt.UTC().UnixNano())
}

func configuredModels(record provider.Provider) []string {
	if record.Config == nil {
		return nil
	}
	value, ok := record.Config["modelLabModels"]
	if !ok {
		return nil
	}
	models := []string{}
	switch typed := value.(type) {
	case []string:
		models = append(models, typed...)
	case []any:
		for _, item := range typed {
			if model, ok := item.(string); ok {
				models = append(models, model)
			}
		}
	}
	return normalizedModels(models)
}

func normalizedModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) priceConfig(ctx context.Context) (pricing.ConfigResponse, error) {
	if s.pricing == nil {
		return pricing.ConfigResponse{}, errorsNewPricingUnavailable
	}
	return s.pricing.Config(ctx)
}

var errorsNewPricingUnavailable = fmt.Errorf("pricing unavailable")

func modelPricing(config pricing.ConfigResponse, configErr error, providerName, model string) ModelPricing {
	price := ModelPricing{Currency: "USD", Unit: "1M_tokens", Source: PriceUnavailable}
	if configErr != nil {
		return price
	}
	price.Input = float64Pointer(config.Fallback.Input)
	price.Output = float64Pointer(config.Fallback.Output)
	price.Source = PriceFallback
	for _, item := range config.Models {
		if item.ProviderName == providerName && item.Model == model {
			price.Input = float64Pointer(item.Input)
			price.Output = float64Pointer(item.Output)
			price.Source = PriceExact
			break
		}
	}
	return price
}

func float64Pointer(value float64) *float64 {
	return &value
}
