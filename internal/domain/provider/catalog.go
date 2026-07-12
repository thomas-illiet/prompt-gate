package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Service) HelpSetup(ctx context.Context, proxyBaseURL string) (HelpSetupResponse, error) {
	records, err := s.ListEnabled(ctx)
	if err != nil {
		return HelpSetupResponse{}, err
	}
	return s.helpSetupFromRecords(ctx, proxyBaseURL, records, nil), nil
}

func (s *Service) HelpSetupForProviderNames(ctx context.Context, proxyBaseURL string, providerNames []string, modelAllowed HelpSetupModelAllowedFunc) (HelpSetupResponse, error) {
	records, err := s.ListEnabledByNames(ctx, providerNames)
	if err != nil {
		return HelpSetupResponse{}, err
	}
	return s.helpSetupFromRecords(ctx, proxyBaseURL, records, modelAllowed), nil
}

func (s *Service) helpSetupFromRecords(ctx context.Context, proxyBaseURL string, records []Provider, modelAllowed HelpSetupModelAllowedFunc) HelpSetupResponse {
	proxyBaseURL = strings.TrimRight(strings.TrimSpace(proxyBaseURL), "/")
	out := HelpSetupResponse{ProxyBaseURL: proxyBaseURL, Providers: make([]HelpSetupProvider, 0, len(records))}
	for _, record := range records {
		item := HelpSetupProvider{
			Name: record.Name, DisplayName: record.DisplayName, Type: record.Type,
			RoutePrefix: routePrefix(record), Models: []string{},
		}
		switch record.Type {
		case ProviderTypeOpenAI, ProviderTypeOllama:
			item.OpenAIBaseURL = proxyBaseURL + item.RoutePrefix
		case ProviderTypeAnthropic:
			item.AnthropicBaseURL = proxyBaseURL + item.RoutePrefix
		}
		if !providerSupportsModelListing(record.Type) {
			out.Providers = append(out.Providers, item)
			continue
		}
		models, err := s.fetchProviderModels(ctx, record)
		if err != nil {
			item.ModelsError = err.Error()
		} else {
			if modelAllowed != nil {
				models = filterAllowedModels(record.Name, models, modelAllowed)
			}
			item.Models = models
		}
		out.Providers = append(out.Providers, item)
	}
	return out
}

func (s *Service) ModelCatalog(ctx context.Context, providerIDs []string) ([]ModelCatalogProvider, error) {
	records, err := s.modelCatalogRecords(ctx, providerIDs)
	if err != nil {
		return nil, err
	}

	out := make([]ModelCatalogProvider, 0, len(records))
	for _, record := range records {
		item := ModelCatalogProvider{ID: record.ID, Name: record.Name, DisplayName: record.DisplayName, Models: []string{}}
		if !record.Enabled {
			item.ModelsError = "provider is disabled"
			out = append(out, item)
			continue
		}
		if !providerSupportsModelListing(record.Type) {
			out = append(out, item)
			continue
		}
		models, err := s.fetchProviderModels(ctx, record)
		if err != nil {
			item.ModelsError = err.Error()
		} else {
			item.Models = models
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) modelCatalogRecords(ctx context.Context, providerIDs []string) ([]Provider, error) {
	if len(providerIDs) == 0 {
		return s.ListEnabled(ctx)
	}
	seen := map[string]struct{}{}
	records := make([]Provider, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		providerID = strings.TrimSpace(providerID)
		if providerID == "" {
			continue
		}
		if _, ok := seen[providerID]; ok {
			continue
		}
		record, err := s.getProvider(ctx, s.db, providerID)
		if err != nil {
			return nil, err
		}
		seen[providerID] = struct{}{}
		records = append(records, record)
	}
	return records, nil
}

func filterAllowedModels(providerName string, models []string, modelAllowed HelpSetupModelAllowedFunc) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		if modelAllowed(providerName, model) {
			out = append(out, model)
		}
	}
	return out
}

func providerSupportsModelListing(providerType ProviderType) bool {
	return providerType == ProviderTypeOpenAI || providerType == ProviderTypeOllama
}

func (s *Service) DecryptAPIKey(record Provider) (string, error) {
	if strings.TrimSpace(record.APIKeyCiphertext) == "" {
		return "", nil
	}
	return s.cipher.Decrypt(record.APIKeyCiphertext)
}

func (s *Service) fetchProviderModels(ctx context.Context, record Provider) ([]string, error) {
	endpoint, err := modelsEndpoint(record)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build model request: %w", err)
	}
	key, err := s.DecryptAPIKey(record)
	if err != nil {
		return nil, fmt.Errorf("decrypt provider key: %w", err)
	}
	if key != "" {
		if record.Type == ProviderTypeAnthropic {
			req.Header.Set("x-api-key", key)
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	if record.Type == ProviderTypeAnthropic {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	client := s.modelHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch models returned %d", resp.StatusCode)
	}

	var payload struct {
		Data   []modelPayload `json:"data"`
		Models []modelPayload `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	models := make([]string, 0, len(payload.Data)+len(payload.Models))
	for _, model := range append(payload.Data, payload.Models...) {
		models = appendModelID(models, model.ID, model.Name, model.DisplayName)
	}
	return models, nil
}

type modelPayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func appendModelID(models []string, values ...string) []string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return append(models, value)
		}
	}
	return models
}

func routePrefix(record Provider) string {
	if record.Type == ProviderTypeOpenAI || record.Type == ProviderTypeOllama {
		return "/" + record.Name + "/v1"
	}
	return "/" + record.Name
}

func modelsEndpoint(record Provider) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(record.BaseURL), "/")
	if base == "" {
		return "", ErrInvalidURL
	}
	if record.Type == ProviderTypeAnthropic && !strings.HasSuffix(base, "/v1") {
		return base + "/v1/models", nil
	}
	return base + "/models", nil
}
