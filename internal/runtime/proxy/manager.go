package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	cdrslog "cdr.dev/slog/v3"
	aibrecorder "github.com/coder/aibridge/recorder"
	"go.opentelemetry.io/otel/trace"

	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	localmcp "promptgate/backend/internal/domain/mcp"
	localprovider "promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

type Options struct {
	Providers                *localprovider.Service
	MCP                      *localmcp.Service
	Recorder                 aibrecorder.Recorder
	FirewallSnapshot         *firewall.SnapshotStore
	AccessSnapshot           *groups.SnapshotStore
	AuthCache                tokens.AuthCache
	Redis                    *redisstore.Store
	HTTPClient               *http.Client
	Logger                   *slog.Logger
	BridgeLogger             cdrslog.Logger
	Tracer                   trace.Tracer
	ReloadDebounce           time.Duration
	MaxBufferedRequestBytes  int64
	MaxBufferedResponseBytes int64
}

type Manager struct {
	opts        Options
	buildBridge bridgeBuilder
	current     atomic.Value
}

type bridgeEntry struct {
	bridge managedBridge
}

// NewManager creates a swappable proxy runtime and builds the initial bridge.
func NewManager(ctx context.Context, opts Options) (*Manager, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ReloadDebounce <= 0 {
		opts.ReloadDebounce = 250 * time.Millisecond
	}
	opts.HTTPClient = normalizeProviderRuntimeOptions(providerRuntimeOptions{
		httpClient:               opts.HTTPClient,
		maxBufferedRequestBytes:  opts.MaxBufferedRequestBytes,
		maxBufferedResponseBytes: opts.MaxBufferedResponseBytes,
	}).httpClient
	manager := &Manager{opts: opts}
	manager.buildBridge = manager.newBridge
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
	builder := m.buildBridge
	if builder == nil {
		builder = m.newBridge
	}
	bridge, err := builder(ctx, providers, mcpProxy)
	if err != nil {
		return fmt.Errorf("create request bridge: %w", err)
	}
	m.installBridge(bridge)
	return nil
}

func (m *Manager) installBridge(bridge managedBridge) {
	previous, _ := m.current.Swap(&bridgeEntry{bridge: bridge}).(*bridgeEntry)
	if previous != nil && previous.bridge != nil {
		go shutdownBridge(previous.bridge, m.opts.Logger)
	}
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
			if err := m.opts.AccessSnapshot.SetSnapshot(snapshot); err == nil {
				return nil
			} else {
				m.opts.Logger.Warn("cached group access snapshot ignored", "error", err)
			}
		} else if err != nil {
			m.opts.Logger.Warn("cached group access snapshot load failed", "error", err)
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

// Shutdown closes the active bridge if it supports graceful shutdown.
func (m *Manager) Shutdown(ctx context.Context) error {
	entry, _ := m.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge == nil {
		return nil
	}
	return entry.bridge.Shutdown(ctx)
}

// shutdownBridge closes a replaced bridge outside the atomic swap path.
func shutdownBridge(bridge managedBridge, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := bridge.Shutdown(ctx); err != nil {
		logger.Error("failed to shut down previous request bridge", "error", err)
	}
}
