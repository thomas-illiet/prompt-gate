package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"promptgate/backend/internal/domain/faq"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/domain/notifications"
	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/setupguide"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
)

// Run applies all database schema migrations in dependency order.
func Run(ctx context.Context, db *gorm.DB) error {
	slog.Info("running database migrations", "models", "users")
	if err := users.NewService(db).AutoMigrate(ctx); err != nil {
		return fmt.Errorf("migrate users: %w", err)
	}

	slog.Info("running database migrations", "models", "subscriptions")
	if err := subscriptions.NewService(db).AutoMigrate(ctx); err != nil {
		return fmt.Errorf("migrate subscriptions: %w", err)
	}

	slog.Info("running database migrations", "models", "tokens")
	if err := db.WithContext(ctx).AutoMigrate(&tokens.Token{}); err != nil {
		return fmt.Errorf("migrate tokens: %w", err)
	}

	slog.Info("running database migrations", "models", "faq")
	if err := faq.NewService(db).AutoMigrate(ctx); err != nil {
		return fmt.Errorf("migrate faq: %w", err)
	}

	slog.Info("running database migrations", "models", "providers")
	if err := db.WithContext(ctx).AutoMigrate(&provider.Provider{}); err != nil {
		return fmt.Errorf("migrate providers: %w", err)
	}

	slog.Info("running database migrations", "models", "setup guides")
	if err := setupguide.NewService(db).AutoMigrate(ctx); err != nil {
		return fmt.Errorf("migrate setup guides: %w", err)
	}

	slog.Info("running database migrations", "models", "pricing")
	if err := pricing.NewService(db, nil).AutoMigrate(ctx); err != nil {
		return fmt.Errorf("migrate pricing: %w", err)
	}

	slog.Info("running database migrations", "models", "groups")
	if err := groups.NewService(db).AutoMigrate(ctx); err != nil {
		return fmt.Errorf("migrate groups: %w", err)
	}

	slog.Info("running database migrations", "models", "monitoring")
	if err := db.WithContext(ctx).AutoMigrate(&monitoring.MonitoringService{}); err != nil {
		return fmt.Errorf("migrate monitoring: %w", err)
	}

	slog.Info("running database migrations", "models", "notifications")
	if err := notifications.NewService(db, false, "").AutoMigrate(ctx); err != nil {
		return fmt.Errorf("migrate notifications: %w", err)
	}

	slog.Info("running database migrations", "models", "proxy")
	if err := proxy.AutoMigrate(ctx, db); err != nil {
		return fmt.Errorf("migrate proxy: %w", err)
	}
	if err := proxy.MigrateLegacySchema(ctx, db); err != nil {
		return fmt.Errorf("migrate proxy legacy schema: %w", err)
	}
	if err := removeLegacyFirewall(ctx, db); err != nil {
		return fmt.Errorf("remove legacy firewall schema: %w", err)
	}
	if err := removeLegacyMCP(ctx, db); err != nil {
		return fmt.Errorf("remove legacy mcp schema: %w", err)
	}

	slog.Info("database migrations completed")
	return nil
}

// removeLegacyMCP removes persisted configuration for the retired MCP feature.
func removeLegacyMCP(ctx context.Context, db *gorm.DB) error {
	migrator := db.WithContext(ctx).Migrator()
	if migrator.HasTable("mcp_servers") {
		return migrator.DropTable("mcp_servers")
	}
	return nil
}

func removeLegacyFirewall(ctx context.Context, db *gorm.DB) error {
	migrator := db.WithContext(ctx).Migrator()
	if migrator.HasTable("firewall_rules") {
		if err := migrator.DropTable("firewall_rules"); err != nil {
			return err
		}
	}
	if migrator.HasColumn(&users.User{}, "firewall_override_enabled") {
		if err := migrator.DropColumn(&users.User{}, "firewall_override_enabled"); err != nil {
			return err
		}
	}
	return nil
}
