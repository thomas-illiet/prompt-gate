package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"promptgate/backend/internal/domain/groups"
	localprovider "promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

// Watch keeps runtime configuration synchronized until the context is canceled.
// Redis Pub/Sub is only a low-latency hint: durable version counters and periodic
// reconciliation recover missed publications and connection interruptions.
func (m *Manager) Watch(ctx context.Context) {
	if m.opts.Redis == nil || !m.opts.Redis.Enabled() {
		return
	}
	m.initializeVersions()

	reconcileInterval := m.opts.ConfigReconcileInterval
	if reconcileInterval <= 0 {
		reconcileInterval = 30 * time.Second
	}
	subscribeRetry := m.opts.ConfigSubscribeRetry
	if subscribeRetry <= 0 {
		subscribeRetry = time.Second
	}
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	lastAuthoritative := time.Time{}

	for ctx.Err() == nil {
		events := m.configEvents(ctx)
		if ctx.Err() != nil {
			return
		}

		// Subscribe is established synchronously. Reconcile only after that point,
		// so a mutation racing startup is either observed as an event or by version.
		// Track authoritative attempts by wall clock as well as ticker delivery: when
		// subscriptions repeatedly close, the closed channel must not starve ticks.
		force := lastAuthoritative.IsZero() || time.Since(lastAuthoritative) >= reconcileInterval
		if err := m.reconcileConfig(ctx, force); err != nil {
			m.opts.Logger.Error("proxy config reconciliation failed", "error", err)
		}
		if force {
			lastAuthoritative = time.Now()
		}
		m.opts.Logger.Info("proxy config reload watcher started")

		if !m.watchConfigSubscription(ctx, events, ticker.C, &lastAuthoritative) {
			return
		}
		m.opts.Logger.Warn("proxy config event subscription closed; reconnecting")
		if !waitWithContext(ctx, subscribeRetry) {
			return
		}
	}
}

func (m *Manager) configEvents(ctx context.Context) <-chan redisstore.Event {
	if m.subscribeEvents != nil {
		return m.subscribeEvents(ctx)
	}
	return m.opts.Redis.Subscribe(ctx)
}

func (m *Manager) watchConfigSubscription(
	ctx context.Context,
	events <-chan redisstore.Event,
	ticks <-chan time.Time,
	lastAuthoritative *time.Time,
) bool {
	reloadDelay := m.opts.ReloadDebounce
	if reloadDelay <= 0 {
		reloadDelay = 250 * time.Millisecond
	}
	reload := newReloadDebouncer(reloadDelay)
	defer reload.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticks:
			if err := m.reconcileConfig(ctx, true); err != nil {
				m.opts.Logger.Error("periodic proxy config reconciliation failed", "error", err)
			}
			*lastAuthoritative = time.Now()
		case <-reload.C():
			hintedVersion, hinted := reload.Version()
			reload.Clear()
			var hint *int64
			if hinted {
				hint = &hintedVersion
			}
			if err := m.reconcileProviders(ctx, hint, false); err != nil {
				m.opts.Logger.Error("proxy bridge reconciliation failed", "error", err)
			} else {
				m.opts.Logger.Info("proxy bridge reloaded")
			}
		case event, ok := <-events:
			if !ok {
				return true
			}
			m.handleConfigEvent(ctx, event.Domain, event.Version, reload)
		}
	}
}

func (m *Manager) handleConfigEvent(ctx context.Context, domain string, version int64, reload *reloadDebouncer) {
	m.opts.Logger.Info("config reload event received", "domain", domain, "version", version)
	switch domain {
	case configevents.DomainGroups:
		if err := m.reconcileGroups(ctx, &version, false); err != nil {
			m.opts.Logger.Error("group access snapshot reload failed", "error", err)
			return
		}
		m.opts.Logger.Info("group access snapshot reloaded", "version", version)
	case configevents.DomainProviders:
		reload.Hint(version)
		rescheduled := reload.Schedule()
		message := "proxy bridge reload scheduled"
		if rescheduled {
			message = "proxy bridge reload rescheduled"
		}
		m.opts.Logger.Info(message, "debounce", reload.delay)
	case configevents.DomainAuth:
		if m.opts.AuthCache != nil {
			m.opts.AuthCache.SetVersion(version)
			m.opts.Logger.Info("auth cache version reloaded", "version", version)
		}
	default:
		m.opts.Logger.Warn("unknown config reload domain ignored", "domain", domain, "version", version)
	}
}

func (m *Manager) reconcileConfig(ctx context.Context, force bool) error {
	groupErr := m.reconcileGroups(ctx, nil, force)
	providerErr := m.reconcileProviders(ctx, nil, force)
	return errors.Join(groupErr, providerErr)
}

func (m *Manager) reconcileGroups(ctx context.Context, hintedVersion *int64, force bool) error {
	if m.opts.AccessSnapshot == nil {
		return nil
	}
	m.initializeVersions()
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	version, versionKnown, versionErr := m.readConfigVersion(ctx, configevents.DomainGroups, hintedVersion)
	if versionErr != nil && !force {
		return versionErr
	}
	if versionKnown && !force && version == m.groupVersion.Load() {
		return nil
	}
	knownChange := configVersionChanged(m.groupVersion.Load(), version, versionKnown, hintedVersion)
	loadCtx, cancel := m.configLoadContext(ctx)
	defer cancel()

	for {
		if err := m.refreshAccessGroupsLocked(loadCtx, true); err != nil {
			if knownChange {
				m.installDenyAllGroups()
			}
			return err
		}
		if !versionKnown {
			return versionErr
		}

		latest, err := m.opts.Redis.Version(ctx, configevents.DomainGroups)
		if err != nil {
			if hintedVersion == nil {
				return fmt.Errorf("verify group config version: %w", err)
			}
			latest = *hintedVersion
		} else if hintedVersion != nil && *hintedVersion > latest {
			latest = *hintedVersion
		}
		if latest != version {
			version = latest
			knownChange = configVersionChanged(m.groupVersion.Load(), version, true, hintedVersion)
			continue
		}
		m.groupVersion.Store(version)
		return nil
	}
}

func (m *Manager) reconcileProviders(ctx context.Context, hintedVersion *int64, force bool) error {
	if m.opts.Providers == nil {
		return nil
	}
	m.initializeVersions()
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	version, versionKnown, versionErr := m.readConfigVersion(ctx, configevents.DomainProviders, hintedVersion)
	if versionErr != nil && !force {
		return versionErr
	}
	if versionKnown && !force && version == m.providerVersion.Load() {
		return nil
	}
	knownChange := configVersionChanged(m.providerVersion.Load(), version, versionKnown, hintedVersion)
	loadCtx, cancel := m.configLoadContext(ctx)
	defer cancel()

	for {
		records, err := m.providerRecordsForBuild(loadCtx, true)
		if err != nil {
			if knownChange {
				m.providerConfigValid = false
				m.installBridge(nil)
			}
			return err
		}
		changed := !m.providerRecordsApplied(records)
		var bridge managedBridge
		if changed {
			bridge, err = m.prepareBridgeFromRecords(loadCtx, records)
			if err != nil {
				// The authoritative database read succeeded, so retaining a bridge
				// containing stale credentials or a revoked provider would be unsafe.
				// Disable new proxy traffic and retry on the next reconciliation.
				m.providerConfigValid = false
				m.installBridge(nil)
				return err
			}
		}
		if !versionKnown {
			m.cacheProviderRecords(ctx, records)
			if changed {
				m.installBridge(bridge)
				m.rememberProviderRecords(records)
			}
			return versionErr
		}

		latest, verifyErr := m.opts.Redis.Version(ctx, configevents.DomainProviders)
		if verifyErr != nil {
			if hintedVersion == nil {
				// The database read is authoritative even when Redis is unavailable.
				// Apply it so a revoked provider is not kept active merely because the
				// durable version counter could not be re-read. Returning the error keeps
				// the watcher retrying reconciliation on the next tick/reconnect.
				m.cacheProviderRecords(ctx, records)
				if changed {
					m.installBridge(bridge)
					m.rememberProviderRecords(records)
				}
				return fmt.Errorf("verify provider config version: %w", verifyErr)
			}
			latest = *hintedVersion
		} else if hintedVersion != nil && *hintedVersion > latest {
			latest = *hintedVersion
		}
		if latest != version {
			if bridge != nil {
				shutdownBridge(bridge, m.opts.Logger)
			}
			version = latest
			knownChange = configVersionChanged(m.providerVersion.Load(), version, true, hintedVersion)
			continue
		}

		m.cacheProviderRecords(ctx, records)
		if changed {
			m.installBridge(bridge)
			m.rememberProviderRecords(records)
		}
		m.providerVersion.Store(version)
		return nil
	}
}

func configVersionChanged(applied, observed int64, observedKnown bool, hinted *int64) bool {
	if hinted != nil && (applied < 0 || *hinted != applied) {
		return true
	}
	// A negative applied version means the startup DB state could not be tied to
	// a durable Redis generation. Once any durable version becomes observable,
	// a failed authoritative reload cannot prove the installed state is current.
	return observedKnown && (applied < 0 || observed != applied)
}

func (m *Manager) configLoadContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := m.opts.ConfigLoadTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (m *Manager) installDenyAllGroups() {
	if m.opts.AccessSnapshot == nil {
		return
	}
	if err := m.opts.AccessSnapshot.SetSnapshot(groups.Snapshot{
		KnownProviders: []string{},
		ProviderTypes:  map[string]localprovider.ProviderType{},
		Users:          map[string]groups.UserAccess{},
		UserGroups:     map[string][]string{},
	}); err != nil {
		m.opts.Logger.Error("failed to install deny-all group snapshot", "error", err)
	}
}

func (m *Manager) readConfigVersion(ctx context.Context, domain string, hintedVersion *int64) (int64, bool, error) {
	version, err := m.opts.Redis.Version(ctx, domain)
	if err == nil {
		if hintedVersion != nil && *hintedVersion > version {
			return *hintedVersion, true, nil
		}
		return version, true, nil
	}
	if hintedVersion != nil {
		return *hintedVersion, true, nil
	}
	return 0, false, fmt.Errorf("load %s config version: %w", domain, err)
}

func waitWithContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type reloadDebouncer struct {
	delay      time.Duration
	timer      *time.Timer
	ch         <-chan time.Time
	version    int64
	hasVersion bool
}

func newReloadDebouncer(delay time.Duration) *reloadDebouncer {
	return &reloadDebouncer{delay: delay}
}

// Schedule starts or resets the timer and reports whether an existing timer was reset.
func (d *reloadDebouncer) Schedule() bool {
	if d.timer == nil {
		d.timer = time.NewTimer(d.delay)
		d.ch = d.timer.C
		return false
	}
	if !d.timer.Stop() {
		select {
		case <-d.timer.C:
		default:
		}
	}
	d.timer.Reset(d.delay)
	d.ch = d.timer.C
	return true
}

func (d *reloadDebouncer) Hint(version int64) {
	if !d.hasVersion || version > d.version {
		d.version = version
		d.hasVersion = true
	}
}

func (d *reloadDebouncer) Version() (int64, bool) {
	return d.version, d.hasVersion
}

func (d *reloadDebouncer) C() <-chan time.Time {
	return d.ch
}

func (d *reloadDebouncer) Clear() {
	d.ch = nil
	d.version = 0
	d.hasVersion = false
}

func (d *reloadDebouncer) Stop() {
	if d.timer != nil {
		d.timer.Stop()
	}
}
