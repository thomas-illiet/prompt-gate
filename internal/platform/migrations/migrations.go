package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/mcp"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
)

// Run applies all database schema migrations in dependency order.
func Run(ctx context.Context, db *gorm.DB) error {
	slog.Info("running database migrations", "models", "users")
	if err := db.WithContext(ctx).AutoMigrate(&users.User{}); err != nil {
		return fmt.Errorf("migrate users: %w", err)
	}

	slog.Info("running database migrations", "models", "tokens")
	if err := db.WithContext(ctx).AutoMigrate(&tokens.Token{}); err != nil {
		return fmt.Errorf("migrate tokens: %w", err)
	}

	slog.Info("running database migrations", "models", "firewall")
	if err := db.WithContext(ctx).AutoMigrate(&firewall.FirewallRule{}); err != nil {
		return fmt.Errorf("migrate firewall: %w", err)
	}

	slog.Info("running database migrations", "models", "providers")
	if err := db.WithContext(ctx).AutoMigrate(&provider.Provider{}); err != nil {
		return fmt.Errorf("migrate providers: %w", err)
	}

	slog.Info("running database migrations", "models", "groups")
	if err := db.WithContext(ctx).AutoMigrate(&groups.Group{}, &groups.GroupProvider{}, &groups.GroupModelPattern{}, &groups.GroupMember{}); err != nil {
		return fmt.Errorf("migrate groups: %w", err)
	}

	slog.Info("running database migrations", "models", "mcp")
	if err := db.WithContext(ctx).AutoMigrate(&mcp.MCPServer{}); err != nil {
		return fmt.Errorf("migrate mcp: %w", err)
	}

	slog.Info("running database migrations", "models", "monitoring")
	if err := db.WithContext(ctx).AutoMigrate(&monitoring.MonitoringService{}); err != nil {
		return fmt.Errorf("migrate monitoring: %w", err)
	}

	slog.Info("running database migrations", "models", "proxy")
	if err := proxy.AutoMigrate(ctx, db); err != nil {
		return fmt.Errorf("migrate proxy: %w", err)
	}

	slog.Info("database migrations completed")
	return nil
}
