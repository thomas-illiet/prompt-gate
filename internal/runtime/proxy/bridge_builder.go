package runtime

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	coderbridge "github.com/coder/aibridge"
	codermcp "github.com/coder/aibridge/mcp"

	localprovider "promptgate/backend/internal/domain/provider"
)

type managedBridge interface {
	http.Handler
	Shutdown(context.Context) error
}

type bridgeBuilder func(context.Context, []coderbridge.Provider, codermcp.ServerProxier) (managedBridge, error)

func (m *Manager) newBridge(ctx context.Context, providers []coderbridge.Provider, mcpProxy codermcp.ServerProxier) (managedBridge, error) {
	return coderbridge.NewRequestBridge(
		ctx,
		providers,
		m.opts.Recorder,
		mcpProxy,
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

// buildMCPProxy converts enabled MCP records into an MCP server proxier.
func (m *Manager) buildMCPProxy(ctx context.Context) (codermcp.ServerProxier, error) {
	records, err := m.mcpRecords(ctx)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	proxies := make(map[string]codermcp.ServerProxier, len(records))
	for _, record := range records {
		headers, err := m.opts.MCP.HeadersForProxy(record)
		if err != nil {
			return nil, err
		}
		allow, err := compileOptionalRegex(record.AllowPattern)
		if err != nil {
			return nil, err
		}
		deny, err := compileOptionalRegex(record.DenyPattern)
		if err != nil {
			return nil, err
		}
		proxy, err := codermcp.NewStreamableHTTPServerProxy(
			record.Name, record.URL, headers, allow, deny,
			m.opts.BridgeLogger.Named("mcp."+record.Name), m.opts.Tracer,
		)
		if err != nil {
			return nil, fmt.Errorf("create mcp proxy %q: %w", record.Name, err)
		}
		proxies[record.Name] = proxy
	}
	manager := codermcp.NewServerProxyManager(proxies, m.opts.Tracer)
	if err := manager.Init(ctx); err != nil {
		m.opts.Logger.Warn("mcp init warning; proxy will continue with available tools", "error", err)
	}
	return manager, nil
}

func compileOptionalRegex(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	return regexp.Compile(pattern)
}
