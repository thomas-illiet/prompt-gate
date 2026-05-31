package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	cdrslog "cdr.dev/slog/v3"
	coderbridge "github.com/coder/aibridge"
	codermcp "github.com/coder/aibridge/mcp"
	"go.opentelemetry.io/otel/trace"

	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	localmcp "promptgate/backend/internal/domain/mcp"
	localprovider "promptgate/backend/internal/domain/provider"
	localproxy "promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

type Options struct {
	Providers        *localprovider.Service
	MCP              *localmcp.Service
	Recorder         *localproxy.Recorder
	FirewallSnapshot *firewall.SnapshotStore
	AccessSnapshot   *groups.SnapshotStore
	AuthCache        tokens.AuthCache
	Redis            *redisstore.Store
	Logger           *slog.Logger
	BridgeLogger     cdrslog.Logger
	Tracer           trace.Tracer
	ReloadDebounce   time.Duration
}

type Manager struct {
	opts    Options
	current atomic.Value
}

type bridgeEntry struct {
	bridge *coderbridge.RequestBridge
}

// NewManager creates a swappable proxy runtime and builds the initial bridge.
func NewManager(ctx context.Context, opts Options) (*Manager, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ReloadDebounce <= 0 {
		opts.ReloadDebounce = 250 * time.Millisecond
	}
	manager := &Manager{opts: opts}
	if opts.FirewallSnapshot != nil {
		if err := opts.FirewallSnapshot.Refresh(ctx); err != nil {
			return nil, fmt.Errorf("load firewall snapshot: %w", err)
		}
		_ = manager.cacheFirewallSnapshot(ctx)
	}
	if opts.AccessSnapshot != nil {
		if err := manager.RefreshAccessGroups(ctx); err != nil {
			return nil, fmt.Errorf("load group access snapshot: %w", err)
		}
	}
	if err := manager.Rebuild(ctx); err != nil {
		return nil, err
	}
	return manager, nil
}

// ServeHTTP delegates requests to the current bridge handler.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	entry, _ := m.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge == nil {
		http.Error(w, "proxy not ready", http.StatusServiceUnavailable)
		return
	}
	entry.bridge.ServeHTTP(w, r)
}

// Rebuild reconstructs the bridge from the latest provider and MCP config.
func (m *Manager) Rebuild(ctx context.Context) error {
	providers, err := m.buildProviders(ctx)
	if err != nil {
		return err
	}
	if len(providers) == 0 {
		return fmt.Errorf("no supported enabled providers configured")
	}
	mcpProxy, err := m.buildMCPProxy(ctx)
	if err != nil {
		return err
	}
	bridge, err := coderbridge.NewRequestBridge(ctx, providers, m.opts.Recorder, mcpProxy, m.opts.BridgeLogger, nil, m.opts.Tracer)
	if err != nil {
		return fmt.Errorf("create request bridge: %w", err)
	}
	next := &bridgeEntry{bridge: bridge}
	previous, _ := m.current.Swap(next).(*bridgeEntry)
	if previous != nil && previous.bridge != nil {
		go shutdownBridge(previous.bridge, m.opts.Logger)
	}
	return nil
}

// RefreshFirewall reloads the firewall snapshot without rebuilding the bridge.
func (m *Manager) RefreshFirewall(ctx context.Context) error {
	if m.opts.FirewallSnapshot == nil {
		return nil
	}
	if err := m.opts.FirewallSnapshot.Refresh(ctx); err != nil {
		return err
	}
	return m.cacheFirewallSnapshot(ctx)
}

// RefreshAccessGroups reloads group access rules without rebuilding the bridge.
func (m *Manager) RefreshAccessGroups(ctx context.Context) error {
	if m.opts.AccessSnapshot == nil {
		return nil
	}
	var snapshot groups.Snapshot
	if m.opts.Redis != nil {
		if ok, err := m.opts.Redis.GetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), &snapshot); err == nil && ok {
			return m.opts.AccessSnapshot.SetSnapshot(snapshot)
		}
	}
	if err := m.opts.AccessSnapshot.Refresh(ctx); err != nil {
		return err
	}
	return m.cacheAccessSnapshot(ctx)
}

// Reload refreshes runtime-backed configuration without restarting the process.
func (m *Manager) Reload(ctx context.Context) error {
	if err := m.RefreshFirewall(ctx); err != nil {
		return fmt.Errorf("refresh firewall snapshot: %w", err)
	}
	if err := m.RefreshAccessGroups(ctx); err != nil {
		return fmt.Errorf("refresh group access snapshot: %w", err)
	}
	if err := m.Rebuild(ctx); err != nil {
		return fmt.Errorf("rebuild proxy bridge: %w", err)
	}
	return nil
}

// Watch subscribes to config events and hot-reloads affected proxy domains.
func (m *Manager) Watch(ctx context.Context) {
	if m.opts.Redis == nil || !m.opts.Redis.Enabled() {
		return
	}
	events := m.opts.Redis.Subscribe(ctx)
	m.opts.Logger.Info("proxy config reload watcher started")
	var timer *time.Timer
	var timerC <-chan time.Time
	scheduleBridgeReload := func() {
		if timer == nil {
			timer = time.NewTimer(m.opts.ReloadDebounce)
			timerC = timer.C
			m.opts.Logger.Info("proxy bridge reload scheduled", "debounce", m.opts.ReloadDebounce)
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(m.opts.ReloadDebounce)
		timerC = timer.C
		m.opts.Logger.Info("proxy bridge reload rescheduled", "debounce", m.opts.ReloadDebounce)
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-timerC:
			timerC = nil
			if err := m.Rebuild(ctx); err != nil {
				m.opts.Logger.Error("proxy bridge reload failed; keeping previous bridge", "error", err)
			} else {
				m.opts.Logger.Info("proxy bridge reloaded")
			}
		case event, ok := <-events:
			if !ok {
				m.opts.Logger.Info("proxy config reload watcher stopped")
				return
			}
			m.opts.Logger.Info("config reload event received", "domain", event.Domain, "version", event.Version)
			switch event.Domain {
			case configevents.DomainFirewall:
				if err := m.RefreshFirewall(ctx); err != nil {
					m.opts.Logger.Error("firewall snapshot reload failed", "error", err)
				} else {
					m.opts.Logger.Info("firewall snapshot reloaded", "version", event.Version)
				}
			case configevents.DomainGroups:
				if err := m.RefreshAccessGroups(ctx); err != nil {
					m.opts.Logger.Error("group access snapshot reload failed", "error", err)
				} else {
					m.opts.Logger.Info("group access snapshot reloaded", "version", event.Version)
				}
			case configevents.DomainProviders, configevents.DomainMCP:
				scheduleBridgeReload()
			case configevents.DomainAuth:
				if m.opts.AuthCache != nil {
					m.opts.AuthCache.SetVersion(event.Version)
					m.opts.Logger.Info("auth cache version reloaded", "version", event.Version)
				}
			default:
				m.opts.Logger.Warn("unknown config reload domain ignored", "domain", event.Domain, "version", event.Version)
			}
		}
	}
}

// Shutdown closes the active bridge if it supports graceful shutdown.
func (m *Manager) Shutdown(ctx context.Context) error {
	entry, _ := m.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge == nil {
		return nil
	}
	return entry.bridge.Shutdown(ctx)
}

// buildProviders converts enabled provider records into promptgate providers.
func (m *Manager) buildProviders(ctx context.Context) ([]coderbridge.Provider, error) {
	records, err := m.providerRecords(ctx)
	if err != nil {
		return nil, err
	}
	providers := make([]coderbridge.Provider, 0, len(records))
	for _, record := range records {
		apiKey, err := m.opts.Providers.DecryptAPIKey(record)
		if err != nil {
			return nil, fmt.Errorf("decrypt provider %q api key: %w", record.Name, err)
		}
		switch record.Type {
		case localprovider.ProviderTypeOpenAI:
			providers = append(providers, newOpenAIProvider(record.Name, record.BaseURL, apiKey))
		case localprovider.ProviderTypeAnthropic:
			providers = append(providers, coderbridge.NewAnthropicProvider(coderbridge.AnthropicConfig{
				Name:    record.Name,
				BaseURL: record.BaseURL,
				Key:     apiKey,
			}, nil))
		case localprovider.ProviderTypeOllama:
			providers = append(providers, newOllamaProvider(record.Name, record.BaseURL, apiKey))
		default:
			m.opts.Logger.Warn("unsupported provider ignored", "name", record.Name, "type", record.Type)
		}
	}
	return providers, nil
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
			record.Name,
			record.URL,
			headers,
			allow,
			deny,
			m.opts.BridgeLogger.Named("mcp."+record.Name),
			m.opts.Tracer,
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

// providerRecords loads providers from Redis snapshots or the database.
func (m *Manager) providerRecords(ctx context.Context) ([]localprovider.Provider, error) {
	var records []localprovider.Provider
	if m.opts.Redis != nil {
		if ok, err := m.opts.Redis.GetJSON(ctx, redisstore.SnapshotKey(configevents.DomainProviders), &records); err == nil && ok {
			return records, nil
		}
	}
	records, err := m.opts.Providers.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if m.opts.Redis != nil {
		_ = m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainProviders), records, m.opts.Redis.TTL())
	}
	return records, nil
}

// mcpRecords loads MCP servers from Redis snapshots or the database.
func (m *Manager) mcpRecords(ctx context.Context) ([]localmcp.MCPServer, error) {
	var records []localmcp.MCPServer
	if m.opts.Redis != nil {
		if ok, err := m.opts.Redis.GetJSON(ctx, redisstore.SnapshotKey(configevents.DomainMCP), &records); err == nil && ok {
			return records, nil
		}
	}
	records, err := m.opts.MCP.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if m.opts.Redis != nil {
		_ = m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainMCP), records, m.opts.Redis.TTL())
	}
	return records, nil
}

// cacheFirewallSnapshot refreshes and caches the firewall snapshot.
func (m *Manager) cacheFirewallSnapshot(ctx context.Context) error {
	if m.opts.Redis == nil || m.opts.FirewallSnapshot == nil {
		return nil
	}
	return m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainFirewall), m.opts.FirewallSnapshot.Snapshot(), m.opts.Redis.TTL())
}

// cacheAccessSnapshot refreshes and caches the group access snapshot.
func (m *Manager) cacheAccessSnapshot(ctx context.Context) error {
	if m.opts.Redis == nil || m.opts.AccessSnapshot == nil {
		return nil
	}
	return m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), m.opts.AccessSnapshot.Snapshot(), m.opts.Redis.TTL())
}

// compileOptionalRegex compiles a regex only when a pattern is configured.
func compileOptionalRegex(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	return regexp.Compile(pattern)
}

// shutdownBridge closes a bridge when the upstream implementation exposes Close.
func shutdownBridge(bridge *coderbridge.RequestBridge, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := bridge.Shutdown(ctx); err != nil {
		logger.Error("failed to shut down previous request bridge", "error", err)
	}
}
