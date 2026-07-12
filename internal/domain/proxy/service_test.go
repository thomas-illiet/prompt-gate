package proxy

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	tokenDomain "promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/clientip"

	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type usagePriceCountingLogger struct {
	logger.Interface
	pricingQueries   int
	usageCostQueries int
	usageCostMaxRows int64
}

func (l *usagePriceCountingLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	if strings.Contains(sql, "usage_prices") {
		l.pricingQueries++
	}
	if strings.Contains(sql, "token_usages") && strings.Contains(sql, "SUM") {
		l.usageCostQueries++
		if rows > l.usageCostMaxRows {
			l.usageCostMaxRows = rows
		}
	}
	l.Interface.Trace(ctx, begin, func() (string, int64) {
		return sql, rows
	}, err)
}

func (l *usagePriceCountingLogger) reset() {
	l.pricingQueries = 0
	l.usageCostQueries = 0
	l.usageCostMaxRows = 0
}

// newProxyServiceTestDB creates proxy service test DB.
func newProxyServiceTestDB(t *testing.T) (*gorm.DB, *Service) {
	t.Helper()
	dsn := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)",
		strings.ReplaceAll(t.Name(), "/", "_"),
	)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	if err := AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy: %v", err)
	}
	if err := db.AutoMigrate(&tokenDomain.Token{}); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}

	now := time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC)
	for _, user := range []users.User{
		{
			ID:                "11111111-1111-1111-1111-111111111111",
			ExternalSub:       "sub-1",
			Email:             "one@example.com",
			PreferredUsername: "one",
			Name:              "One",
			Type:              auth.UserTypeUser,
			Role:              auth.RoleUser,
			IsActive:          true,
			LastLoginAt:       now,
		},
		{
			ID:                "22222222-2222-2222-2222-222222222222",
			ExternalSub:       "sub-2",
			Email:             "two@example.com",
			PreferredUsername: "two",
			Name:              "Two",
			Type:              auth.UserTypeUser,
			Role:              auth.RoleUser,
			IsActive:          true,
			LastLoginAt:       now,
		},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	return db, NewService(db)
}

// seedProxyInteraction seeds proxy interaction.
func seedProxyInteraction(t *testing.T, db *gorm.DB, userID, id, prompt, model string, at time.Time, inputTokens, outputTokens int64) {
	t.Helper()
	endedAt := at.Add(90 * time.Second)
	if err := db.Create(&Interception{
		ID:           id,
		InitiatorID:  userID,
		Provider:     "openai",
		ProviderType: "openai",
		Model:        model,
		StartedAt:    at,
		EndedAt:      &endedAt,
		Metadata:     "{}",
	}).Error; err != nil {
		t.Fatalf("seed interception: %v", err)
	}
	if prompt != "" {
		if err := db.Create(&UserPrompt{
			InterceptionID:     id,
			ProviderResponseID: "response-" + id,
			Prompt:             prompt,
			Metadata:           "{}",
			CreatedAt:          at.Add(time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed prompt: %v", err)
		}
	}
	if inputTokens > 0 || outputTokens > 0 {
		if err := db.Create(&TokenUsage{
			InterceptionID:        id,
			ProviderResponseID:    "response-" + id,
			InputTokens:           inputTokens,
			OutputTokens:          outputTokens,
			CacheReadInputTokens:  3,
			CacheWriteInputTokens: 4,
			Metadata:              "{}",
			CreatedAt:             at.Add(2 * time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed token usage: %v", err)
		}
	}
}

// setInterceptionEndedAt sets interception ended at.
func setInterceptionEndedAt(t *testing.T, db *gorm.DB, id string, endedAt *time.Time) {
	t.Helper()
	if err := db.Model(&Interception{}).
		Where("id = ?", id).
		Update("ended_at", endedAt).Error; err != nil {
		t.Fatalf("set interception end: %v", err)
	}
}

// setInterceptionProvider sets interception provider.
func setInterceptionProvider(t *testing.T, db *gorm.DB, id, providerName, providerType string) {
	t.Helper()
	if err := db.Model(&Interception{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"provider":      providerName,
			"provider_type": providerType,
		}).Error; err != nil {
		t.Fatalf("set interception provider: %v", err)
	}
}

// setInterceptionClientIP sets interception client IP.
func setInterceptionClientIP(t *testing.T, db *gorm.DB, id, clientIP string) {
	t.Helper()
	if err := db.Model(&Interception{}).
		Where("id = ?", id).
		Update("client_ip", clientIP).Error; err != nil {
		t.Fatalf("set interception client IP: %v", err)
	}
}

// assertFloatClose asserts float close.
func assertFloatClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000000001 {
		t.Fatalf("got %f want %f", got, want)
	}
}

// mustAggregateUsageKPIs computes dashboard KPI fixtures from raw proxy records.
func mustAggregateUsageKPIs(t *testing.T, service *Service) {
	t.Helper()
	if err := service.db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProxyDailyUsageBreakdown{}).Error; err != nil {
			return fmt.Errorf("clear usage breakdown kpis: %w", err)
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ProxyDailyUsageKPI{}).Error; err != nil {
			return fmt.Errorf("clear usage kpis: %w", err)
		}

		var interceptions []Interception
		if err := tx.Find(&interceptions).Error; err != nil {
			return fmt.Errorf("load fixture interceptions: %w", err)
		}
		interceptionsByID := make(map[string]Interception, len(interceptions))
		for _, interception := range interceptions {
			interceptionsByID[interception.ID] = interception
			if err := aggregateInterceptionStarted(tx, interception); err != nil {
				return err
			}
			if err := aggregateInterceptionDuration(tx, interception); err != nil {
				return err
			}
		}

		var prompts []UserPrompt
		if err := tx.Find(&prompts).Error; err != nil {
			return fmt.Errorf("load fixture prompts: %w", err)
		}
		for _, prompt := range prompts {
			interception, ok := interceptionsByID[prompt.InterceptionID]
			if !ok {
				continue
			}
			if err := aggregatePromptUsage(tx, interception, prompt); err != nil {
				return err
			}
		}

		var tokenUsages []TokenUsage
		if err := tx.Find(&tokenUsages).Error; err != nil {
			return fmt.Errorf("load fixture token usage: %w", err)
		}
		for _, usage := range tokenUsages {
			interception, ok := interceptionsByID[usage.InterceptionID]
			if !ok {
				continue
			}
			if err := aggregateTokenUsage(tx, interception, usage); err != nil {
				return err
			}
		}

		var toolUsages []ToolUsage
		if err := tx.Find(&toolUsages).Error; err != nil {
			return fmt.Errorf("load fixture tool usage: %w", err)
		}
		for _, usage := range toolUsages {
			interception, ok := interceptionsByID[usage.InterceptionID]
			if !ok {
				continue
			}
			if err := aggregateToolUsage(tx, interception, usage); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		t.Fatalf("aggregate usage kpis: %v", err)
	}
}

// TestAutoMigrateDoesNotDropLegacyArtifacts verifies auto migrate is not destructive.
func TestAutoMigrateDoesNotDropLegacyArtifacts(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	if err := db.Exec(`CREATE TABLE model_thoughts (
		id text primary key,
		interception_id text not null,
		content text not null
	)`).Error; err != nil {
		t.Fatalf("create stale model thoughts table: %v", err)
	}
	if err := db.Exec(`ALTER TABLE token_usages ADD COLUMN endpoint text NOT NULL DEFAULT ''`).Error; err != nil {
		t.Fatalf("add legacy endpoint column: %v", err)
	}

	if err := AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy: %v", err)
	}

	if !db.Migrator().HasTable("model_thoughts") {
		t.Fatal("expected stale model_thoughts table to remain after AutoMigrate")
	}
	if !db.Migrator().HasColumn(&tokenUsageEndpointMigration{}, "endpoint") {
		t.Fatal("expected legacy endpoint column to remain after AutoMigrate")
	}
}

// TestMigrateLegacySchemaDropsModelThoughts verifies explicit legacy migration drops model thoughts.
func TestMigrateLegacySchemaDropsModelThoughts(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	if err := db.Exec(`CREATE TABLE model_thoughts (
		id text primary key,
		interception_id text not null,
		content text not null
	)`).Error; err != nil {
		t.Fatalf("create stale model thoughts table: %v", err)
	}

	if err := MigrateLegacySchema(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy legacy schema: %v", err)
	}
	if err := MigrateLegacySchema(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy legacy schema second run: %v", err)
	}

	if db.Migrator().HasTable("model_thoughts") {
		t.Fatal("expected stale model_thoughts table to be dropped")
	}
}

// TestMigrateLegacySchemaBackfillsEmbeddingTokenTypeAndDropsEndpoint verifies explicit legacy migration drops endpoint.
func TestMigrateLegacySchemaBackfillsEmbeddingTokenTypeAndDropsEndpoint(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	seedProxyInteraction(t, db, "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "", "text-embedding-3-small", now, 0, 0)
	if err := db.Create(&TokenUsage{
		InterceptionID:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		ProviderResponseID: "embedding-response",
		InputTokens:        17,
		Metadata:           `{"endpoint":"/embeddings"}`,
		CreatedAt:          now,
	}).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}
	if err := db.Exec(`ALTER TABLE token_usages ADD COLUMN endpoint text NOT NULL DEFAULT ''`).Error; err != nil {
		t.Fatalf("add legacy endpoint column: %v", err)
	}
	if err := db.Table("token_usages").
		Where("provider_response_id = ?", "embedding-response").
		Update("endpoint", tokenUsageEndpointEmbeddings).Error; err != nil {
		t.Fatalf("seed legacy endpoint: %v", err)
	}

	if err := MigrateLegacySchema(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy legacy schema: %v", err)
	}
	if err := MigrateLegacySchema(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy legacy schema second run: %v", err)
	}

	var usage TokenUsage
	if err := db.Where("provider_response_id = ?", "embedding-response").First(&usage).Error; err != nil {
		t.Fatalf("load token usage: %v", err)
	}
	if usage.Type != tokenUsageTypeEmbedding {
		t.Fatalf("expected embedding token type backfilled, got %q", usage.Type)
	}
	if db.Migrator().HasColumn(&tokenUsageEndpointMigration{}, "endpoint") {
		t.Fatal("expected legacy endpoint column to be dropped")
	}
}

// TestRecordInterceptionPersistsClientIP verifies recorder stores the resolved request IP.
func TestRecordInterceptionPersistsClientIP(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	rec := NewRecorder(db)
	ctx := clientip.ContextWithClientIP(context.Background(), "198.51.100.7")
	interceptionID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	if err := rec.RecordInterception(ctx, &aibrecorder.InterceptionRecord{
		ID:           interceptionID,
		InitiatorID:  "11111111-1111-1111-1111-111111111111",
		Provider:     "openai",
		ProviderName: "openai-main",
		Model:        "gpt-5",
		Metadata:     aibrecorder.Metadata{},
	}); err != nil {
		t.Fatalf("record interception: %v", err)
	}

	var record Interception
	if err := db.First(&record, "id = ?", interceptionID).Error; err != nil {
		t.Fatalf("load interception: %v", err)
	}
	if record.ClientIP != "198.51.100.7" {
		t.Fatalf("expected client IP persisted, got %q", record.ClientIP)
	}
}

// TestUsageSummaryAggregatesOnlyCurrentUser verifies usage summary aggregates only current user.
func TestUsageSummaryAggregatesOnlyCurrentUser(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	interactionAt := now.AddDate(0, 0, -1)

	seedProxyInteraction(t, db, "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Alpha prompt", "gpt-5", interactionAt, 11, 13)
	seedProxyInteraction(t, db, "22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Other prompt", "gpt-5", interactionAt, 101, 103)

	if err := db.Create(&ToolUsage{
		InterceptionID:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		ProviderResponseID: "response-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Tool:               "linear.list",
		Input:              "{}",
		Metadata:           "{}",
		CreatedAt:          interactionAt.Add(3 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed tool usage: %v", err)
	}
	mustAggregateUsageKPIs(t, service)

	summary, err := service.UsageSummary(context.Background(), "11111111-1111-1111-1111-111111111111", 7, now)
	if err != nil {
		t.Fatalf("usage summary: %v", err)
	}

	if summary.Totals.Requests != 1 || summary.Totals.Prompts != 1 || summary.Totals.ToolCalls != 1 {
		t.Fatalf("unexpected totals: %#v", summary.Totals)
	}
	if summary.Totals.InputTokens != 11 || summary.Totals.OutputTokens != 13 || summary.Totals.TotalTokens != 31 {
		t.Fatalf("unexpected token totals: %#v", summary.Totals)
	}
	if len(summary.RecentPrompts) != 1 || summary.RecentPrompts[0].Prompt != "Alpha prompt" {
		t.Fatalf("unexpected recent prompts: %#v", summary.RecentPrompts)
	}
	if len(summary.TopModels) != 1 || summary.TopModels[0].Name != "gpt-5" {
		t.Fatalf("unexpected top models: %#v", summary.TopModels)
	}
}

// TestDashboardWidgetsAggregateByWindowAndDimension verifies dashboard widgets aggregate by window and dimension.
func TestDashboardWidgetsAggregateByWindowAndDimension(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	otherUserID := "22222222-2222-2222-2222-222222222222"
	oldAt := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	recentAt := now.AddDate(0, 0, -1)
	latestAt := now.AddDate(0, 0, -2)

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Old prompt", "gpt-old", oldAt, 100, 200)
	setInterceptionProvider(t, db, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "legacy-provider", "ollama")
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Recent prompt", "gpt-5", recentAt, 11, 13)
	setInterceptionProvider(t, db, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "openai-main", "openai")
	seedProxyInteraction(t, db, userID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Latest prompt", "claude-4", latestAt, 50, 10)
	setInterceptionProvider(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", "anthropic-main", "anthropic")
	seedProxyInteraction(t, db, otherUserID, "dddddddd-dddd-dddd-dddd-dddddddddddd", "Hidden prompt", "gpt-5", latestAt, 1000, 1000)

	pendingEndedAt := (*time.Time)(nil)
	setInterceptionEndedAt(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", pendingEndedAt)
	mustAggregateUsageKPIs(t, service)

	tokens, err := service.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.Window != UsageWindow7Days || tokens.TotalTokens != 98 {
		t.Fatalf("unexpected 7 day token response: %#v", tokens)
	}
	if tokens.CompletionTokens != 98 || tokens.EmbeddingTokens != 0 {
		t.Fatalf("unexpected token type totals: %#v", tokens)
	}

	messages, err := service.DashboardMessages(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard messages: %v", err)
	}
	if messages.Messages != 2 {
		t.Fatalf("expected two messages, got %#v", messages)
	}

	duration, err := service.DashboardDuration(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard duration: %v", err)
	}
	if duration.TotalDurationMs != 90000 {
		t.Fatalf("expected only completed duration, got %#v", duration)
	}

	activity, err := service.DashboardActivity(context.Background(), userID, UsageWindowAll, now)
	if err != nil {
		t.Fatalf("dashboard activity: %v", err)
	}
	if activity.Window != UsageWindowAll || len(activity.Daily) != 29 || activity.Daily[0].Date != "2026-01-02" {
		t.Fatalf("unexpected all time activity window: %#v", activity)
	}
	if activity.Daily[0].Requests != 1 || activity.Daily[len(activity.Daily)-2].Requests != 1 {
		t.Fatalf("unexpected daily request buckets: %#v", activity.Daily)
	}

	topModels, err := service.DashboardTopModels(context.Background(), userID, UsageWindowAll, now)
	if err != nil {
		t.Fatalf("dashboard top models: %v", err)
	}
	if len(topModels.Items) != 3 || topModels.Items[0].Name != "gpt-old" || topModels.Items[0].TotalTokens != 307 {
		t.Fatalf("unexpected top models: %#v", topModels.Items)
	}
	if topModels.Items[0].EstimatedCost == nil {
		t.Fatal("expected top model estimated cost")
	}
	assertFloatClose(t, topModels.Items[0].EstimatedCost.TotalUSD, 0.006535)

	topProviderNames, err := service.DashboardTopProviderNames(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard top provider names: %v", err)
	}
	if len(topProviderNames.Items) != 2 || topProviderNames.Items[0].Name != "anthropic-main" {
		t.Fatalf("unexpected top provider names: %#v", topProviderNames.Items)
	}

	topProviderTypes, err := service.DashboardTopProviderTypes(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard top provider types: %v", err)
	}
	if len(topProviderTypes.Items) != 2 || topProviderTypes.Items[0].Name != "anthropic" {
		t.Fatalf("unexpected top provider types: %#v", topProviderTypes.Items)
	}
}

// TestAdminDashboardWidgetsAggregateGlobally verifies admin dashboard widgets aggregate globally.
func TestAdminDashboardWidgetsAggregateGlobally(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	otherUserID := "22222222-2222-2222-2222-222222222222"
	serviceAccountID := "33333333-3333-3333-3333-333333333333"
	oldAt := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	recentAt := now.AddDate(0, 0, -1)
	latestAt := now.AddDate(0, 0, -2)

	if err := db.Create(&users.User{
		ID:                serviceAccountID,
		ExternalSub:       "service:bot",
		PreferredUsername: "service-bot",
		Name:              "Service Bot",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed service account: %v", err)
	}

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Current user", "gpt-5", recentAt, 10, 20)
	seedProxyInteraction(t, db, otherUserID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Other user", "gpt-5", latestAt, 30, 40)
	seedProxyInteraction(t, db, serviceAccountID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Service account", "claude-4", latestAt, 50, 60)
	seedProxyInteraction(t, db, otherUserID, "dddddddd-dddd-dddd-dddd-dddddddddddd", "Old global", "gpt-old", oldAt, 1, 1)

	revokedAt := now.Add(-time.Hour)
	expiredAt := now.Add(-30 * time.Minute)
	if err := db.Create(&[]tokenDomain.Token{
		{
			UserID:      userID,
			Name:        "active-user",
			Description: "",
			TokenHash:   "active-user-hash",
			ExpiresAt:   now.Add(time.Hour),
		},
		{
			UserID:      serviceAccountID,
			Name:        "active-service",
			Description: "",
			TokenHash:   "active-service-hash",
			ExpiresAt:   now.Add(time.Hour),
		},
		{
			UserID:      otherUserID,
			Name:        "revoked",
			Description: "",
			TokenHash:   "revoked-hash",
			ExpiresAt:   now.Add(time.Hour),
			RevokedAt:   &revokedAt,
		},
		{
			UserID:      otherUserID,
			Name:        "expired",
			Description: "",
			TokenHash:   "expired-hash",
			ExpiresAt:   now.Add(-time.Hour),
			ExpiredAt:   &expiredAt,
		},
	}).Error; err != nil {
		t.Fatalf("seed tokens: %v", err)
	}
	mustAggregateUsageKPIs(t, service)

	userTokens, err := service.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("user dashboard tokens: %v", err)
	}
	if userTokens.TotalTokens != 37 {
		t.Fatalf("expected current user tokens to stay isolated, got %#v", userTokens)
	}

	globalTokens, err := service.AdminDashboardTokens(context.Background(), UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("global dashboard tokens: %v", err)
	}
	if globalTokens.TotalTokens != 231 {
		t.Fatalf("unexpected global token total: %#v", globalTokens)
	}

	messages, err := service.AdminDashboardMessages(context.Background(), UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("global dashboard messages: %v", err)
	}
	if messages.Messages != 3 {
		t.Fatalf("expected three global messages, got %#v", messages)
	}

	activity, err := service.AdminDashboardActivity(context.Background(), UsageWindowAll, now)
	if err != nil {
		t.Fatalf("global dashboard activity: %v", err)
	}
	if activity.Window != UsageWindowAll || len(activity.Daily) != 29 || activity.Daily[0].Date != "2026-01-02" {
		t.Fatalf("unexpected global all-time activity window: %#v", activity)
	}

	adoption, err := service.AdminDashboardAdoption(context.Background(), UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("global dashboard adoption: %v", err)
	}
	if adoption.ActiveUsers != 2 || adoption.ActiveServiceAccounts != 1 || adoption.ActiveVirtualKeys != 2 {
		t.Fatalf("unexpected adoption totals: %#v", adoption)
	}

	topIdentities, err := service.AdminDashboardTopIdentities(context.Background(), UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("global dashboard top identities: %v", err)
	}
	if len(topIdentities.Items) != 3 || topIdentities.Items[0].Name != "Service Bot" || topIdentities.Items[0].TotalTokens != 117 {
		t.Fatalf("unexpected top identities: %#v", topIdentities.Items)
	}
	if topIdentities.Items[0].EstimatedCost == nil {
		t.Fatal("expected top identity estimated cost")
	}
	assertFloatClose(t, topIdentities.Items[0].EstimatedCost.TotalUSD, 0.002085)
}

// TestDashboardAllTimeWithoutActivityReturnsEmptyDaily verifies dashboard all time without activity returns empty daily.
func TestDashboardAllTimeWithoutActivityReturnsEmptyDaily(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)

	activity, err := service.DashboardActivity(context.Background(), "22222222-2222-2222-2222-222222222222", UsageWindowAll, now)
	if err != nil {
		t.Fatalf("dashboard activity: %v", err)
	}
	if activity.Window != UsageWindowAll || len(activity.Daily) != 0 || !activity.StartsAt.Equal(activity.EndsAt) {
		t.Fatalf("unexpected empty all time activity: %#v", activity)
	}
}

// TestUsageSummaryRejectsInvalidWindow verifies usage summary rejects invalid window.
func TestUsageSummaryRejectsInvalidWindow(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	if _, err := service.UsageSummary(context.Background(), "11111111-1111-1111-1111-111111111111", 14, time.Now()); err == nil {
		t.Fatal("expected invalid usage window error")
	}
}
