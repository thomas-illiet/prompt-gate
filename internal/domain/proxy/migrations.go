package proxy

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type tokenUsageEndpointMigration struct {
	TokenUsage
	Endpoint string `gorm:"column:endpoint"`
}

// TableName returns the legacy token usage table used during endpoint-column migration.
func (tokenUsageEndpointMigration) TableName() string {
	return "token_usages"
}

// AutoMigrate migrates proxy recorder tables.
func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(
		&Interception{},
		&TokenUsage{},
		&ToolUsage{},
		&ProxyDailyUsageKPI{},
		&ProxyDailyUsageBreakdown{},
		&ProcessedUsageEvent{},
	); err != nil {
		return err
	}

	if err := db.WithContext(ctx).
		Model(&TokenUsage{}).
		Where("metadata LIKE ? OR metadata LIKE ?", `%"type":"embedding"%`, `%"endpoint":"/embeddings"%`).
		Update("type", tokenUsageTypeEmbedding).Error; err != nil {
		return fmt.Errorf("backfill token usage type from metadata: %w", err)
	}
	return nil
}

// MigrateLegacySchema applies explicit destructive cleanup for legacy proxy schema artifacts.
func MigrateLegacySchema(ctx context.Context, db *gorm.DB) error {
	migrator := db.WithContext(ctx).Migrator()
	if migrator.HasTable("user_prompts") {
		if err := migrator.DropTable("user_prompts"); err != nil {
			return fmt.Errorf("drop user prompts table: %w", err)
		}
	}
	if migrator.HasColumn("proxy_daily_usage_kpis", "prompts") {
		if err := db.WithContext(ctx).Exec("ALTER TABLE proxy_daily_usage_kpis DROP COLUMN prompts").Error; err != nil {
			return fmt.Errorf("drop prompt count column: %w", err)
		}
	}
	if err := db.WithContext(ctx).
		Where("type = ?", legacyPromptUsageType).
		Delete(&ProcessedUsageEvent{}).Error; err != nil {
		return fmt.Errorf("delete prompt usage event markers: %w", err)
	}
	if migrator.HasTable("model_thoughts") {
		if err := migrator.DropTable("model_thoughts"); err != nil {
			return fmt.Errorf("drop model thoughts table: %w", err)
		}
	}
	if migrator.HasColumn(&tokenUsageEndpointMigration{}, "endpoint") {
		if err := db.WithContext(ctx).
			Table("token_usages").
			Where("endpoint = ?", tokenUsageEndpointEmbeddings).
			Update("type", tokenUsageTypeEmbedding).Error; err != nil {
			return fmt.Errorf("backfill token usage type from endpoint: %w", err)
		}
		if err := db.WithContext(ctx).Exec("ALTER TABLE token_usages DROP COLUMN endpoint").Error; err != nil {
			return fmt.Errorf("drop token usage endpoint column: %w", err)
		}
	}
	return nil
}
