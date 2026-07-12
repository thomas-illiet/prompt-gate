package runtime

import (
	"context"

	localmcp "promptgate/backend/internal/domain/mcp"
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

func (m *Manager) cacheFirewallSnapshot(ctx context.Context) error {
	if m.opts.Redis == nil || m.opts.FirewallSnapshot == nil {
		return nil
	}
	return m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainFirewall), m.opts.FirewallSnapshot.Snapshot(), m.opts.Redis.TTL())
}

func (m *Manager) cacheAccessSnapshot(ctx context.Context) error {
	if m.opts.Redis == nil || m.opts.AccessSnapshot == nil {
		return nil
	}
	return m.opts.Redis.SetJSON(ctx, redisstore.SnapshotKey(configevents.DomainGroups), m.opts.AccessSnapshot.Snapshot(), m.opts.Redis.TTL())
}
