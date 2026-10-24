package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"

	coderbridge "github.com/coder/aibridge"

	localprovider "promptgate/backend/internal/domain/provider"
)

type managedBridge interface {
	http.Handler
	Shutdown(context.Context) error
}

type bridgeBuilder func(context.Context, []coderbridge.Provider) (managedBridge, error)

func (m *Manager) newBridge(ctx context.Context, providers []coderbridge.Provider) (managedBridge, error) {
	return coderbridge.NewRequestBridge(
		ctx,
		providers,
		m.opts.Recorder,
		nil,
		m.opts.BridgeLogger,
		nil,
		m.opts.Tracer,
	)
}

// buildProviders converts enabled provider records into bridge providers.
func (m *Manager) buildProviders(ctx context.Context) ([]coderbridge.Provider, error) {
	records, err := m.providerRecords(ctx)
	if err != nil {
		return nil, err
	}
	runtimeOpts := m.providerRuntimeOptions()
	providers := make([]coderbridge.Provider, 0, len(records))
	for _, record := range records {
		provider, err := m.buildProvider(record, runtimeOpts)
		if err != nil {
			return nil, err
		}
		if provider != nil {
			providers = append(providers, provider)
		}
	}
	return providers, nil
}

func (m *Manager) buildProvider(record localprovider.Provider, opts providerRuntimeOptions) (coderbridge.Provider, error) {
	// AI Bridge historically enables raw request/response dumps from this
	// environment variable when no explicit directory is supplied. Prompt Gate
	// never exposes raw API dumps, so clear the legacy upstream switch before
	// every initial build and hot reload.
	if err := os.Unsetenv("BRIDGE_DUMP_DIR"); err != nil {
		return nil, fmt.Errorf("disable AI Bridge API dumps: %w", err)
	}

	apiKey, err := m.opts.Providers.DecryptAPIKey(record)
	if err != nil {
		return nil, fmt.Errorf("decrypt provider %q api key: %w", record.Name, err)
	}
	switch record.Type {
	case localprovider.ProviderTypeOpenAI:
		return newOpenAIProvider(record.Name, record.BaseURL, apiKey, opts), nil
	case localprovider.ProviderTypeAnthropic:
		return coderbridge.NewAnthropicProvider(coderbridge.AnthropicConfig{
			Name: record.Name, BaseURL: record.BaseURL, Key: apiKey,
		}, nil), nil
	case localprovider.ProviderTypeOllama:
		return newOllamaProvider(record.Name, record.BaseURL, apiKey, opts), nil
	default:
		m.opts.Logger.Warn("unsupported provider ignored", "name", record.Name, "type", record.Type)
		return nil, nil
	}
}

func (m *Manager) providerRuntimeOptions() providerRuntimeOptions {
	return normalizeProviderRuntimeOptions(providerRuntimeOptions{
		httpClient:               m.opts.HTTPClient,
		maxBufferedRequestBytes:  m.opts.MaxBufferedRequestBytes,
		maxBufferedResponseBytes: m.opts.MaxBufferedResponseBytes,
	})
}
