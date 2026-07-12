package runtime

import (
	"context"
	"time"

	"promptgate/backend/internal/platform/configevents"
)

// Watch subscribes to configuration events until the context is canceled.
func (m *Manager) Watch(ctx context.Context) {
	if m.opts.Redis == nil || !m.opts.Redis.Enabled() {
		return
	}

	events := m.opts.Redis.Subscribe(ctx)
	m.opts.Logger.Info("proxy config reload watcher started")
	reload := newReloadDebouncer(m.opts.ReloadDebounce)
	defer reload.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reload.C():
			reload.Clear()
			m.rebuildAfterEvent(ctx)
		case event, ok := <-events:
			if !ok {
				m.opts.Logger.Info("proxy config reload watcher stopped")
				return
			}
			m.handleConfigEvent(ctx, event.Domain, event.Version, reload)
		}
	}
}

func (m *Manager) handleConfigEvent(ctx context.Context, domain string, version int64, reload *reloadDebouncer) {
	m.opts.Logger.Info("config reload event received", "domain", domain, "version", version)
	switch domain {
	case configevents.DomainFirewall:
		if err := m.RefreshFirewall(ctx); err != nil {
			m.opts.Logger.Error("firewall snapshot reload failed", "error", err)
			return
		}
		m.opts.Logger.Info("firewall snapshot reloaded", "version", version)
	case configevents.DomainGroups:
		if err := m.RefreshAccessGroups(ctx); err != nil {
			m.opts.Logger.Error("group access snapshot reload failed", "error", err)
			return
		}
		m.opts.Logger.Info("group access snapshot reloaded", "version", version)
	case configevents.DomainProviders, configevents.DomainMCP:
		rescheduled := reload.Schedule()
		message := "proxy bridge reload scheduled"
		if rescheduled {
			message = "proxy bridge reload rescheduled"
		}
		m.opts.Logger.Info(message, "debounce", m.opts.ReloadDebounce)
	case configevents.DomainAuth:
		if m.opts.AuthCache != nil {
			m.opts.AuthCache.SetVersion(version)
			m.opts.Logger.Info("auth cache version reloaded", "version", version)
		}
	default:
		m.opts.Logger.Warn("unknown config reload domain ignored", "domain", domain, "version", version)
	}
}

func (m *Manager) rebuildAfterEvent(ctx context.Context) {
	if err := m.Rebuild(ctx); err != nil {
		m.opts.Logger.Error("proxy bridge reload failed; keeping previous bridge", "error", err)
		return
	}
	m.opts.Logger.Info("proxy bridge reloaded")
}

type reloadDebouncer struct {
	delay time.Duration
	timer *time.Timer
	ch    <-chan time.Time
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

func (d *reloadDebouncer) C() <-chan time.Time {
	return d.ch
}

func (d *reloadDebouncer) Clear() {
	d.ch = nil
}

func (d *reloadDebouncer) Stop() {
	if d.timer != nil {
		d.timer.Stop()
	}
}
