package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
	aibrecorder "github.com/coder/aibridge/recorder"
)

func TestRedisRecorderEnqueuesUsageEventWithClientIP(t *testing.T) {
	srv := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	rec := NewRedisRecorder(store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	ctx := clientip.ContextWithClientIP(context.Background(), "198.51.100.9")
	if err := rec.RecordInterception(ctx, &aibrecorder.InterceptionRecord{
		ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		InitiatorID:  "11111111-1111-1111-1111-111111111111",
		Provider:     "openai",
		ProviderName: "openai-main",
		Model:        "gpt-5",
		Metadata:     aibrecorder.Metadata{"route": "/v1/chat/completions"},
	}); err != nil {
		t.Fatalf("record interception: %v", err)
	}

	entries, err := srv.Stream(UsageEventsStream)
	if err != nil {
		t.Fatalf("load stream: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one stream entry, got %d", len(entries))
	}
	payload := streamValue(entries[0].Values, UsageEventPayloadField)
	if payload == "" {
		t.Fatal("expected usage payload field")
	}
	var event UsageEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatalf("decode usage event: %v", err)
	}
	if event.Type != UsageEventInterceptionStarted || event.InterceptionStarted == nil {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.InterceptionStarted.ClientIP != "198.51.100.9" || event.InterceptionStarted.Provider != "openai-main" || event.InterceptionStarted.ProviderType != "openai" {
		t.Fatalf("unexpected interception payload: %#v", event.InterceptionStarted)
	}
}

func TestRedisRecorderBestEffortLogsRedisErrors(t *testing.T) {
	var logs bytes.Buffer
	rec := NewRedisRecorder(nil, slog.New(slog.NewTextHandler(&logs, nil)))

	err := rec.RecordTokenUsage(context.Background(), &aibrecorder.TokenUsageRecord{
		InterceptionID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		MsgID:          "response-1",
		Input:          10,
		Output:         20,
		Metadata:       aibrecorder.Metadata{},
	})
	if err != nil {
		t.Fatalf("expected best effort nil error, got %v", err)
	}
	if !strings.Contains(logs.String(), "level=ERROR") || !strings.Contains(logs.String(), "eventType=token_usage") {
		t.Fatalf("expected error log for failed enqueue, got %q", logs.String())
	}
}

func TestWorkerProcessesEventsIdempotentlyAcrossInstances(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	workerOne := NewWorker(db, nil, WorkerOptions{ConsumerName: "one"}, nil)
	workerTwo := NewWorker(db, nil, WorkerOptions{ConsumerName: "two"}, nil)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	startedAt := now.Add(-time.Hour)
	endedAt := startedAt.Add(2 * time.Second)

	started := UsageEvent{
		EventID:   "event-started",
		Type:      UsageEventInterceptionStarted,
		CreatedAt: startedAt,
		InterceptionStarted: &InterceptionStartedEvent{
			ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			InitiatorID:  "11111111-1111-1111-1111-111111111111",
			Provider:     "openai-main",
			ProviderType: "openai",
			Model:        "gpt-5",
			ClientIP:     "198.51.100.8",
			StartedAt:    startedAt,
			Metadata:     "{}",
		},
	}
	tokenUsage := UsageEvent{
		EventID:   "event-token",
		Type:      UsageEventTokenUsage,
		CreatedAt: startedAt,
		TokenUsage: &TokenUsageEvent{
			InterceptionID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			ProviderResponseID:    "response-1",
			InputTokens:           10,
			OutputTokens:          5,
			CacheReadInputTokens:  1,
			CacheWriteInputTokens: 2,
			Type:                  tokenUsageTypeCompletion,
			Metadata:              "{}",
			CreatedAt:             startedAt,
		},
	}
	promptUsage := UsageEvent{
		EventID:   "event-prompt",
		Type:      UsageEventPromptUsage,
		CreatedAt: startedAt,
		PromptUsage: &PromptUsageEvent{
			InterceptionID:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			ProviderResponseID: "response-1",
			Prompt:             "Hello",
			Metadata:           "{}",
			CreatedAt:          startedAt,
		},
	}
	ended := UsageEvent{
		EventID:   "event-ended",
		Type:      UsageEventInterceptionEnded,
		CreatedAt: endedAt,
		InterceptionEnded: &InterceptionEndedEvent{
			ID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			EndedAt: endedAt,
		},
	}

	for _, event := range []UsageEvent{started, tokenUsage, promptUsage, ended} {
		if err := workerOne.ProcessUsageEvent(context.Background(), event, event.EventID+"-redis"); err != nil {
			t.Fatalf("process %s: %v", event.Type, err)
		}
		if err := workerTwo.ProcessUsageEvent(context.Background(), event, event.EventID+"-redis-duplicate"); !errors.Is(err, errUsageEventAlreadyProcessed) {
			t.Fatalf("expected duplicate event to be ignored, got %v", err)
		}
	}

	tokens, err := service.DashboardTokens(context.Background(), "11111111-1111-1111-1111-111111111111", UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard tokens: %v", err)
	}
	if tokens.TotalTokens != 18 || tokens.CompletionInputTokens != 13 || tokens.CompletionOutputTokens != 5 {
		t.Fatalf("unexpected token totals: %#v", tokens)
	}
	messages, err := service.DashboardMessages(context.Background(), "11111111-1111-1111-1111-111111111111", UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard messages: %v", err)
	}
	if messages.Messages != 1 {
		t.Fatalf("expected one message, got %#v", messages)
	}
	duration, err := service.DashboardDuration(context.Background(), "11111111-1111-1111-1111-111111111111", UsageWindow7Days, now)
	if err != nil {
		t.Fatalf("dashboard duration: %v", err)
	}
	if duration.TotalDurationMs != 2000 {
		t.Fatalf("expected 2000ms duration, got %#v", duration)
	}
	prompts, err := service.ListPrompts(context.Background(), "11111111-1111-1111-1111-111111111111", PromptListParams{})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	if prompts.Total != 1 || prompts.Items[0].Prompt != "Hello" {
		t.Fatalf("unexpected prompts: %#v", prompts)
	}
}

func TestWorkerLeavesDependencyMissingEventUnprocessed(t *testing.T) {
	db, _ := newProxyServiceTestDB(t)
	worker := NewWorker(db, nil, WorkerOptions{}, nil)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	tokenUsage := UsageEvent{
		EventID:   "event-token-before-start",
		Type:      UsageEventTokenUsage,
		CreatedAt: now,
		TokenUsage: &TokenUsageEvent{
			InterceptionID:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			ProviderResponseID: "response-1",
			InputTokens:        10,
			Type:               tokenUsageTypeCompletion,
			Metadata:           "{}",
			CreatedAt:          now,
		},
	}

	err := worker.ProcessUsageEvent(context.Background(), tokenUsage, "1-0")
	if !errors.Is(err, ErrUsageEventDependencyMissing) {
		t.Fatalf("expected missing dependency, got %v", err)
	}
	var processed int64
	if err := db.Model(&ProcessedUsageEvent{}).Where("event_id = ?", tokenUsage.EventID).Count(&processed).Error; err != nil {
		t.Fatalf("count processed events: %v", err)
	}
	if processed != 0 {
		t.Fatalf("expected dependency failure to rollback processed marker, got %d", processed)
	}
}

func TestRawUsageCleanupKeepsAggregatedKPIs(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	userID := "11111111-1111-1111-1111-111111111111"
	oldAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	newAt := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Old prompt", "gpt-5", oldAt, 10, 20)
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "New prompt", "gpt-5", newAt, 30, 40)
	mustAggregateUsageKPIs(t, service)

	deleted, err := service.DeleteRawUsageBefore(context.Background(), time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("delete raw usage: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected one raw interception deleted, got %d", deleted)
	}

	activity, err := service.DashboardActivity(context.Background(), userID, UsageWindowAll, time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("dashboard activity: %v", err)
	}
	if activity.Window != UsageWindowAll || len(activity.Daily) != 30 || activity.Daily[0].Date != "2026-01-01" {
		t.Fatalf("expected durable KPI activity to keep old day, got %#v", activity)
	}
	prompts, err := service.ListPrompts(context.Background(), userID, PromptListParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}
	if prompts.Total != 1 || prompts.Items[0].Prompt != "New prompt" {
		t.Fatalf("expected raw prompt exploration to only keep new prompt, got %#v", prompts)
	}
}

func streamValue(values []string, key string) string {
	for i := 0; i+1 < len(values); i += 2 {
		if values[i] == key {
			return values[i+1]
		}
	}
	return ""
}
