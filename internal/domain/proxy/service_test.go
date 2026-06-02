package proxy

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	tokenDomain "promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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

// assertFloatClose asserts float close.
func assertFloatClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000000001 {
		t.Fatalf("got %f want %f", got, want)
	}
}

// TestAutoMigrateDropsModelThoughts verifies auto migrate drops model thoughts.
func TestAutoMigrateDropsModelThoughts(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	if err := db.Exec(`CREATE TABLE model_thoughts (
		id text primary key,
		interception_id text not null,
		content text not null
	)`).Error; err != nil {
		t.Fatalf("create stale model thoughts table: %v", err)
	}

	if err := AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy: %v", err)
	}

	if db.Migrator().HasTable("model_thoughts") {
		t.Fatal("expected stale model_thoughts table to be dropped")
	}
}

// TestAutoMigrateBackfillsEmbeddingTokenTypeAndDropsEndpoint verifies auto migrate backfills embedding token type and drops endpoint.
func TestAutoMigrateBackfillsEmbeddingTokenTypeAndDropsEndpoint(t *testing.T) {
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

	if err := AutoMigrate(context.Background(), db); err != nil {
		t.Fatalf("migrate proxy: %v", err)
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

// TestDashboardTokensDifferentiatesCompletionAndEmbeddingTokens verifies dashboard tokens differentiates completion and embedding tokens.
func TestDashboardTokensDifferentiatesCompletionAndEmbeddingTokens(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	startedAt := now.Add(-time.Hour)

	for _, record := range []Interception{
		{
			ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			InitiatorID:  userID,
			Provider:     "openai-main",
			ProviderType: "openai",
			Model:        "gpt-5",
			StartedAt:    startedAt,
			Metadata:     "{}",
		},
		{
			ID:           "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			InitiatorID:  userID,
			Provider:     "openai-main",
			ProviderType: "openai",
			Model:        "text-embedding-3-small",
			StartedAt:    startedAt,
			Metadata:     "{}",
		},
	} {
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("seed interception: %v", err)
		}
	}
	if err := db.Create(&[]TokenUsage{
		{
			InterceptionID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			ProviderResponseID:    "completion-response",
			InputTokens:           5,
			OutputTokens:          6,
			CacheReadInputTokens:  1,
			CacheWriteInputTokens: 2,
			Metadata:              "{}",
			CreatedAt:             startedAt,
		},
		{
			InterceptionID:     "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			ProviderResponseID: "embedding-response",
			InputTokens:        17,
			Type:               tokenUsageTypeEmbedding,
			Metadata:           `{"type":"embedding"}`,
			CreatedAt:          startedAt,
		},
	}).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}

	tokens, err := service.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.CompletionTokens != 14 || tokens.EmbeddingTokens != 17 || tokens.TotalTokens != 31 {
		t.Fatalf("unexpected dashboard token totals: %#v", tokens)
	}
	if tokens.CompletionInputTokens != 8 || tokens.CompletionOutputTokens != 6 {
		t.Fatalf("unexpected dashboard completion split: %#v", tokens)
	}
	if tokens.EstimatedCost == nil {
		t.Fatal("expected dashboard estimated cost")
	}
	assertFloatClose(t, tokens.EstimatedCost.InputUSD, 0.00004)
	assertFloatClose(t, tokens.EstimatedCost.OutputUSD, 0.00018)
	assertFloatClose(t, tokens.EstimatedCost.EmbeddingUSD, 0.00000034)
	assertFloatClose(t, tokens.EstimatedCost.TotalUSD, 0.00022034)
	if tokens.EstimatedCost.Rates.InputUSDPer1MTokens != 5 || tokens.EstimatedCost.Rates.OutputUSDPer1MTokens != 30 || tokens.EstimatedCost.Rates.EmbeddingUSDPer1MTokens != 0.02 {
		t.Fatalf("unexpected default cost rates: %#v", tokens.EstimatedCost.Rates)
	}

	summary, err := service.UsageSummary(context.Background(), userID, 7, now)
	if err != nil {
		t.Fatalf("usage summary: %v", err)
	}
	if summary.Totals.CompletionTokens != 14 || summary.Totals.EmbeddingTokens != 17 || summary.Totals.TotalTokens != 31 {
		t.Fatalf("unexpected summary token totals: %#v", summary.Totals)
	}
	if summary.Totals.CompletionInputTokens != 8 || summary.Totals.CompletionOutputTokens != 6 {
		t.Fatalf("unexpected summary completion split: %#v", summary.Totals)
	}
	if summary.Totals.EstimatedCost == nil {
		t.Fatal("expected summary estimated cost")
	}

	activity, err := service.DashboardActivity(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard activity: %v", err)
	}
	latest := activity.Daily[len(activity.Daily)-1]
	if latest.CompletionTokens != 14 || latest.EmbeddingTokens != 17 || latest.TotalTokens != 31 {
		t.Fatalf("unexpected daily token totals: %#v", latest)
	}
	if latest.CompletionInputTokens != 8 || latest.CompletionOutputTokens != 6 {
		t.Fatalf("unexpected daily completion split: %#v", latest)
	}
	if latest.EstimatedCost == nil {
		t.Fatal("expected daily estimated cost")
	}
	assertFloatClose(t, latest.EstimatedCost.TotalUSD, 0.00022034)

	topModels, err := service.DashboardTopModels(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard top models: %v", err)
	}
	var embeddingModel *UsageBreakdown
	for i := range topModels.Items {
		if topModels.Items[i].Name == "text-embedding-3-small" {
			embeddingModel = &topModels.Items[i]
			break
		}
	}
	if embeddingModel == nil || embeddingModel.EstimatedCost == nil {
		t.Fatalf("expected embedding model estimated cost: %#v", topModels.Items)
	}
	assertFloatClose(t, embeddingModel.EstimatedCost.EmbeddingUSD, 0.00000034)
}

// TestUsageCostEstimatesCanBeOverriddenAndDisabled verifies usage cost estimates can be overridden and disabled.
func TestUsageCostEstimatesCanBeOverriddenAndDisabled(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	service = NewService(db, WithUsageCost(UsageCostConfig{
		Enabled: true,
		Rates: CostRates{
			InputUSDPer1MTokens:     100,
			OutputUSDPer1MTokens:    200,
			EmbeddingUSDPer1MTokens: 300,
		},
	}))
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	startedAt := now.Add(-time.Hour)

	if err := db.Create(&Interception{
		ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		InitiatorID:  userID,
		Provider:     "openai-main",
		ProviderType: "openai",
		Model:        "gpt-5",
		StartedAt:    startedAt,
		Metadata:     "{}",
	}).Error; err != nil {
		t.Fatalf("seed interception: %v", err)
	}
	if err := db.Create(&TokenUsage{
		InterceptionID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		ProviderResponseID:    "completion-response",
		InputTokens:           5,
		OutputTokens:          6,
		CacheReadInputTokens:  1,
		CacheWriteInputTokens: 2,
		Metadata:              "{}",
		CreatedAt:             startedAt,
	}).Error; err != nil {
		t.Fatalf("seed token usage: %v", err)
	}

	tokens, err := service.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.EstimatedCost == nil {
		t.Fatal("expected overridden estimated cost")
	}
	assertFloatClose(t, tokens.EstimatedCost.InputUSD, 0.0008)
	assertFloatClose(t, tokens.EstimatedCost.OutputUSD, 0.0012)
	assertFloatClose(t, tokens.EstimatedCost.TotalUSD, 0.002)

	disabled := NewService(db, WithUsageCost(UsageCostConfig{Enabled: false}))
	disabledTokens, err := disabled.DashboardTokens(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens disabled: %v", err)
	}
	if disabledTokens.EstimatedCost != nil {
		t.Fatalf("expected no dashboard estimated cost when disabled, got %#v", disabledTokens.EstimatedCost)
	}
	disabledSummary, err := disabled.UsageSummary(context.Background(), userID, 7, now)
	if err != nil {
		t.Fatalf("usage summary disabled: %v", err)
	}
	if disabledSummary.Totals.EstimatedCost != nil {
		t.Fatalf("expected no summary estimated cost when disabled, got %#v", disabledSummary.Totals.EstimatedCost)
	}
	disabledActivity, err := disabled.DashboardActivity(context.Background(), userID, UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard activity disabled: %v", err)
	}
	for _, daily := range disabledActivity.Daily {
		if daily.EstimatedCost != nil {
			t.Fatalf("expected no daily estimated cost when disabled, got %#v", daily.EstimatedCost)
		}
	}
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

// TestListPromptsPaginatesSearchesAndIsolatesUsers verifies list prompts paginates searches and isolates users.
func TestListPromptsPaginatesSearchesAndIsolatesUsers(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Alpha first", "gpt-5", now.Add(-3*time.Hour), 1, 2)
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Beta prompt", "gpt-5", now.Add(-2*time.Hour), 3, 4)
	seedProxyInteraction(t, db, userID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Alpha newest", "gpt-5", now.Add(-1*time.Hour), 5, 6)
	seedProxyInteraction(t, db, "22222222-2222-2222-2222-222222222222", "dddddddd-dddd-dddd-dddd-dddddddddddd", "Alpha hidden", "gpt-5", now, 100, 200)

	result, err := service.ListPrompts(context.Background(), userID, PromptListParams{
		Page:     1,
		PageSize: 1,
		Search:   "alpha",
	})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}

	if result.Total != 2 || len(result.Items) != 1 {
		t.Fatalf("unexpected page result: %#v", result)
	}
	if result.Items[0].Prompt != "Alpha newest" {
		t.Fatalf("expected newest alpha prompt, got %#v", result.Items[0])
	}
	if result.Items[0].InputTokens != 5 || result.Items[0].OutputTokens != 6 || result.Items[0].TotalTokens != 11 {
		t.Fatalf("expected attached token totals, got %#v", result.Items[0])
	}
	if result.Items[0].DurationMs == nil || *result.Items[0].DurationMs != 90000 {
		t.Fatalf("expected attached duration, got %#v", result.Items[0].DurationMs)
	}
}

// TestListPromptsSortsByDuration verifies list prompts sorts by duration.
func TestListPromptsSortsByDuration(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	longStartedAt := now.Add(-3 * time.Hour)
	shortStartedAt := now.Add(-2 * time.Hour)

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Long prompt", "gpt-5", longStartedAt, 1, 2)
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Pending prompt", "gpt-5", now.Add(-1*time.Hour), 3, 4)
	seedProxyInteraction(t, db, userID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Short prompt", "gpt-5", shortStartedAt, 5, 6)

	longEndedAt := longStartedAt.Add(3 * time.Minute)
	shortEndedAt := shortStartedAt.Add(30 * time.Second)
	setInterceptionEndedAt(t, db, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", &longEndedAt)
	setInterceptionEndedAt(t, db, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	setInterceptionEndedAt(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", &shortEndedAt)

	result, err := service.ListPrompts(context.Background(), userID, PromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "durationMs",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("sort prompts by duration: %v", err)
	}

	if got := []string{result.Items[0].Prompt, result.Items[1].Prompt, result.Items[2].Prompt}; fmt.Sprint(got) != "[Short prompt Long prompt Pending prompt]" {
		t.Fatalf("unexpected duration order: %v", got)
	}
	if result.Items[2].DurationMs != nil {
		t.Fatalf("expected pending prompt duration to be nil, got %v", *result.Items[2].DurationMs)
	}
}

// TestListAdminPromptsSearchesFiltersIdentifiesUsersAndSortsTokens verifies list admin prompts searches filters identifies users and sorts tokens.
func TestListAdminPromptsSearchesFiltersIdentifiesUsersAndSortsTokens(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userOne := "11111111-1111-1111-1111-111111111111"
	userTwo := "22222222-2222-2222-2222-222222222222"

	seedProxyInteraction(t, db, userOne, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Alpha first", "gpt-5", now.Add(-3*time.Hour), 10, 2)
	seedProxyInteraction(t, db, userTwo, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Beta prompt", "gpt-4o", now.Add(-2*time.Hour), 5, 1)
	seedProxyInteraction(t, db, userTwo, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Alpha newest", "gpt-5", now.Add(-1*time.Hour), 50, 1)

	result, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "createdAt",
		SortDir:  "desc",
	})
	if err != nil {
		t.Fatalf("list admin prompts: %v", err)
	}

	if result.Total != 3 || len(result.Items) != 3 {
		t.Fatalf("unexpected admin prompt result: %#v", result)
	}
	if result.Items[0].Prompt != "Alpha newest" || result.Items[0].UserID != userTwo {
		t.Fatalf("expected newest prompt for user two, got %#v", result.Items[0])
	}
	if result.Items[0].UserName != "Two" || result.Items[0].UserEmail != "two@example.com" || result.Items[0].UserPreferredUsername != "two" {
		t.Fatalf("expected user identity on prompt row, got %#v", result.Items[0])
	}
	if result.Items[0].InputTokens != 50 || result.Items[0].OutputTokens != 1 || result.Items[0].TotalTokens != 51 {
		t.Fatalf("expected attached token totals, got %#v", result.Items[0])
	}
	if result.Items[0].DurationMs == nil || *result.Items[0].DurationMs != 90000 {
		t.Fatalf("expected attached duration, got %#v", result.Items[0].DurationMs)
	}

	filtered, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		Search:   "alpha",
		UserID:   userOne,
	})
	if err != nil {
		t.Fatalf("filter admin prompts: %v", err)
	}
	if filtered.Total != 1 || filtered.Items[0].Prompt != "Alpha first" {
		t.Fatalf("expected filtered prompt for user one, got %#v", filtered)
	}

	sorted, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "totalTokens",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("sort admin prompts by tokens: %v", err)
	}
	if sorted.Items[0].Prompt != "Beta prompt" || sorted.Items[0].TotalTokens != 6 {
		t.Fatalf("expected lowest token prompt first, got %#v", sorted.Items[0])
	}

	longEndedAt := now.Add(-3 * time.Hour).Add(4 * time.Minute)
	shortEndedAt := now.Add(-2 * time.Hour).Add(20 * time.Second)
	setInterceptionEndedAt(t, db, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", &longEndedAt)
	setInterceptionEndedAt(t, db, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", &shortEndedAt)
	setInterceptionEndedAt(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", nil)

	durationSorted, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "durationMs",
		SortDir:  "desc",
	})
	if err != nil {
		t.Fatalf("sort admin prompts by duration: %v", err)
	}
	if got := []string{durationSorted.Items[0].Prompt, durationSorted.Items[1].Prompt, durationSorted.Items[2].Prompt}; fmt.Sprint(got) != "[Alpha first Beta prompt Alpha newest]" {
		t.Fatalf("unexpected admin duration order: %v", got)
	}
	if durationSorted.Items[2].DurationMs != nil {
		t.Fatalf("expected pending admin prompt duration to be nil, got %v", *durationSorted.Items[2].DurationMs)
	}
}

// TestListAdminPromptsRejectsInvalidSort verifies list admin prompts rejects invalid sort.
func TestListAdminPromptsRejectsInvalidSort(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	_, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "unknown",
		SortDir:  "desc",
	})
	if !errors.Is(err, ErrInvalidSort) {
		t.Fatalf("expected invalid sort error, got %v", err)
	}
}

// TestUsageSummaryRejectsInvalidWindow verifies usage summary rejects invalid window.
func TestUsageSummaryRejectsInvalidWindow(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	if _, err := service.UsageSummary(context.Background(), "11111111-1111-1111-1111-111111111111", 14, time.Now()); err == nil {
		t.Fatal("expected invalid usage window error")
	}
}
