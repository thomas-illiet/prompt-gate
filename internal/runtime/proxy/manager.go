package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	cdrslog "cdr.dev/slog/v3"
	aibrecorder "github.com/coder/aibridge/recorder"
	"go.opentelemetry.io/otel/trace"

	"promptgate/backend/internal/domain/groups"
	localprovider "promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

type Options struct {
	Providers                *localprovider.Service
	Recorder                 aibrecorder.Recorder
	AccessSnapshot           *groups.SnapshotStore
	AuthCache                tokens.AuthCache
	Redis                    *redisstore.Store
	HTTPClient               *http.Client
	Logger                   *slog.Logger
	BridgeLogger             cdrslog.Logger
	Tracer                   trace.Tracer
	ReloadDebounce           time.Duration
	ConfigReconcileInterval  time.Duration
	ConfigSubscribeRetry     time.Duration
	ConfigLoadTimeout        time.Duration
	MaxBufferedRequestBytes  int64
	MaxBufferedResponseBytes int64
}

type Manager struct {
	opts            Options
	buildBridge     bridgeBuilder
	subscribeEvents func(context.Context) <-chan redisstore.Event
	current         atomic.Value
	reloadMu        sync.Mutex

	versionsInitialized atomic.Bool
	providerVersion     atomic.Int64
	groupVersion        atomic.Int64
	appliedProviders    []localprovider.Provider
	providerConfigValid bool
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
	if opts.ConfigReconcileInterval <= 0 {
		opts.ConfigReconcileInterval = 30 * time.Second
	}
	if opts.ConfigSubscribeRetry <= 0 {
		opts.ConfigSubscribeRetry = time.Second
	}
	if opts.ConfigLoadTimeout <= 0 {
		opts.ConfigLoadTimeout = 10 * time.Second
	}
	opts.HTTPClient = normalizeProviderRuntimeOptions(providerRuntimeOptions{
		httpClient:               opts.HTTPClient,
		maxBufferedRequestBytes:  opts.MaxBufferedRequestBytes,
		maxBufferedResponseBytes: opts.MaxBufferedResponseBytes,
	}).httpClient
	manager := &Manager{opts: opts}
	manager.initializeVersions()
	manager.buildBridge = manager.newBridge
	manager.reloadMu.Lock()
	defer manager.reloadMu.Unlock()
	if opts.AccessSnapshot != nil {
		if err := manager.refreshAccessGroupsLocked(ctx, true); err != nil {
			return nil, fmt.Errorf("load group access snapshot: %w", err)
		}
	}
	if err := manager.rebuildLocked(ctx, true, false); err != nil {
		return nil, err
	}
	manager.syncAppliedVersions(ctx)
	return manager, nil
}

// ServeHTTP delegates requests to the current bridge handler.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	entry, _ := m.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge == nil {
		http.Error(w, "proxy not ready", http.StatusServiceUnavailable)
		return
	}
	upstreamRequest := r.Clone(withProviderTransport(r.Context(), m.opts.HTTPClient))
	upstreamRequest.Header = sanitizeUpstreamHeaders(r.Header)
	upstreamRequest.Trailer = nil
	entry.bridge.ServeHTTP(w, upstreamRequest)
}

// Rebuild reconstructs the bridge from the latest provider config.
func (m *Manager) Rebuild(ctx context.Context) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	return m.rebuildLocked(ctx, false, false)
}

func (m *Manager) rebuildLocked(ctx context.Context, authoritative, allowEmpty bool) error {
	bridge, records, err := m.prepareBridge(ctx, authoritative)
	if err != nil {
		if authoritative && records != nil {
			// The database read succeeded but the resulting provider set could not
			// be built (for example, an API key could not be decrypted). Do not keep
			// serving a bridge that may contain a provider revoked by that DB state.
			m.providerConfigValid = false
			m.installBridge(nil)
		}
		return err
	}
	if bridge == nil && !allowEmpty {
		return fmt.Errorf("no supported enabled providers configured")
	}
	if authoritative {
		m.cacheProviderRecords(ctx, records)
	}
	m.installBridge(bridge)
	m.rememberProviderRecords(records)
	return nil
}

func (m *Manager) prepareBridge(ctx context.Context, authoritative bool) (managedBridge, []localprovider.Provider, error) {
	records, err := m.providerRecordsForBuild(ctx, authoritative)
	if err != nil {
		return nil, nil, err
	}
	bridge, err := m.prepareBridgeFromRecords(ctx, records)
	return bridge, records, err
}

func (m *Manager) prepareBridgeFromRecords(ctx context.Context, records []localprovider.Provider) (managedBridge, error) {
	providers, err := m.buildProviderRecords(records)
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, nil
	}
	builder := m.buildBridge
	if builder == nil {
		builder = m.newBridge
	}
	bridge, err := builder(ctx, providers)
	if err != nil {
		return nil, fmt.Errorf("create request bridge: %w", err)
	}
	return bridge, nil
}

func (m *Manager) installBridge(bridge managedBridge) {
	previous, _ := m.current.Swap(&bridgeEntry{bridge: bridge}).(*bridgeEntry)
	if previous != nil && previous.bridge != nil {
		go shutdownBridge(previous.bridge, m.opts.Logger)
	}
}

// RefreshAccessGroups reloads group access rules without rebuilding the bridge.
func (m *Manager) RefreshAccessGroups(ctx context.Context) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	return m.refreshAccessGroupsLocked(ctx, false)
}

func (m *Manager) refreshAccessGroupsLocked(ctx context.Context, authoritative bool) error {
	if m.opts.AccessSnapshot == nil {
		return nil
	}
	var snapshot groups.Snapshot
	if !authoritative && m.opts.Redis != nil {
		if ok, err := m.opts.Redis.GetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), &snapshot); err == nil && ok {
			if snapshot.UserGroups == nil {
				m.opts.Logger.Warn("cached group access snapshot ignored", "error", groups.ErrLegacySnapshot)
			} else if err := m.opts.AccessSnapshot.SetSnapshot(snapshot); err == nil {
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
	if err := m.cacheAccessSnapshot(ctx); err != nil {
		m.opts.Logger.Warn("group access snapshot cache failed", "error", err)
	}
	return nil
}

func (m *Manager) providerRecordsApplied(records []localprovider.Provider) bool {
	return m.providerConfigValid && m.appliedProviders != nil && reflect.DeepEqual(m.appliedProviders, records)
}

func (m *Manager) rememberProviderRecords(records []localprovider.Provider) {
	m.appliedProviders = append(make([]localprovider.Provider, 0, len(records)), records...)
	m.providerConfigValid = true
}

// Reload refreshes runtime-backed configuration without restarting the process.
func (m *Manager) Reload(ctx context.Context) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	if err := m.refreshAccessGroupsLocked(ctx, true); err != nil {
		return fmt.Errorf("refresh group access snapshot: %w", err)
	}
	if err := m.rebuildLocked(ctx, true, true); err != nil {
		return fmt.Errorf("rebuild proxy bridge: %w", err)
	}
	m.syncAppliedVersions(ctx)
	return nil
}

// Shutdown closes the active bridge if it supports graceful shutdown.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	entry, _ := m.current.Load().(*bridgeEntry)
	if entry == nil || entry.bridge == nil {
		return nil
	}
	return entry.bridge.Shutdown(ctx)
}

func (m *Manager) initializeVersions() {
	if m.versionsInitialized.CompareAndSwap(false, true) {
		m.providerVersion.Store(-1)
		m.groupVersion.Store(-1)
	}
}

func (m *Manager) syncAppliedVersions(ctx context.Context) {
	if m.opts.Redis == nil {
		return
	}
	if version, err := m.opts.Redis.Version(ctx, configevents.DomainProviders); err == nil {
		m.providerVersion.Store(version)
	}
	if version, err := m.opts.Redis.Version(ctx, configevents.DomainGroups); err == nil {
		m.groupVersion.Store(version)
	}
}

// shutdownBridge closes a replaced bridge outside the atomic swap path.
func shutdownBridge(bridge managedBridge, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := bridge.Shutdown(ctx); err != nil {
		logger.Error("failed to shut down previous request bridge", "error", err)
	}
}
