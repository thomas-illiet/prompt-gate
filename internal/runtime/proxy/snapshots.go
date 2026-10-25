package runtime

import (
	"context"

	localprovider "promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/redisstore"
)

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

func (m *Manager) providerRecordsForBuild(ctx context.Context, authoritative bool) ([]localprovider.Provider, error) {
	if !authoritative {
		return m.providerRecords(ctx)
	}
	return m.opts.Providers.ListEnabled(ctx)
}

func (m *Manager) cacheProviderRecords(ctx context.Context, records []localprovider.Provider) {
	if m.opts.Redis == nil {
		return
	}
	if err := m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainProviders), records, m.opts.Redis.TTL()); err != nil {
		m.opts.Logger.Warn("provider snapshot cache failed", "error", err)
	}
}

func (m *Manager) cacheAccessSnapshot(ctx context.Context) error {
	if m.opts.Redis == nil || m.opts.AccessSnapshot == nil {
		return nil
	}
	return m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), m.opts.AccessSnapshot.Snapshot(), m.opts.Redis.TTL())
}
