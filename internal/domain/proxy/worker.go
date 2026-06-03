package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"promptgate/backend/internal/platform/redisstore"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errUsageEventAlreadyProcessed = errors.New("usage event already processed")

type WorkerOptions struct {
	ConsumerName       string
	BatchSize          int64
	BlockTimeout       time.Duration
	PendingIdleTimeout time.Duration
}

type Worker struct {
	db     *gorm.DB
	store  *redisstore.Store
	logger *slog.Logger
	opts   WorkerOptions
}

// NewWorker creates a Redis Stream consumer for promptgate background work.
func NewWorker(db *gorm.DB, store *redisstore.Store, opts WorkerOptions, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	if opts.BlockTimeout <= 0 {
		opts.BlockTimeout = 5 * time.Second
	}
	if opts.PendingIdleTimeout <= 0 {
		opts.PendingIdleTimeout = 30 * time.Second
	}
	if strings.TrimSpace(opts.ConsumerName) == "" {
		opts.ConsumerName = defaultUsageConsumerName()
	}
	return &Worker{db: db, store: store, logger: logger, opts: opts}
}

// Run consumes background events until the context is canceled.
func (w *Worker) Run(ctx context.Context) error {
	client := w.redisClient()
	if client == nil {
		return errors.New("redis store unavailable")
	}
	if err := ensureUsageConsumerGroup(ctx, client); err != nil {
		return err
	}
	w.logger.Info("promptgate worker started", "consumer", w.opts.ConsumerName, "batch_size", w.opts.BatchSize)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if messages, err := w.claimPending(ctx, client); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("failed to claim pending usage events", "error", err)
			sleepWithContext(ctx, time.Second)
		} else if len(messages) > 0 {
			w.handleMessages(ctx, client, messages)
			continue
		}

		streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    UsageEventsConsumerGroup,
			Consumer: w.opts.ConsumerName,
			Streams:  []string{UsageEventsStream, ">"},
			Count:    w.opts.BatchSize,
			Block:    w.opts.BlockTimeout,
		}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("failed to read usage events", "error", err)
			sleepWithContext(ctx, time.Second)
			continue
		}
		for _, stream := range streams {
			w.handleMessages(ctx, client, stream.Messages)
		}
	}
}

func (w *Worker) claimPending(ctx context.Context, client *redis.Client) ([]redis.XMessage, error) {
	messages, _, err := client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   UsageEventsStream,
		Group:    UsageEventsConsumerGroup,
		Consumer: w.opts.ConsumerName,
		MinIdle:  w.opts.PendingIdleTimeout,
		Start:    "0-0",
		Count:    w.opts.BatchSize,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return messages, err
}

func (w *Worker) handleMessages(ctx context.Context, client *redis.Client, messages []redis.XMessage) {
	for _, message := range messages {
		event, err := usageEventFromMessage(message)
		if err != nil {
			w.logger.Error("dropping invalid usage event", "redisMessageId", message.ID, "error", err)
			w.ackAndDelete(ctx, client, message.ID)
			continue
		}

		err = w.ProcessUsageEvent(ctx, event, message.ID)
		switch {
		case err == nil:
			w.ackAndDelete(ctx, client, message.ID)
		case errors.Is(err, errUsageEventAlreadyProcessed):
			w.ackAndDelete(ctx, client, message.ID)
		case errors.Is(err, ErrUsageEventDependencyMissing):
			w.logger.Warn(
				"usage event dependency missing; leaving message pending",
				"eventType", event.Type,
				"eventId", event.EventID,
				"interceptionId", eventInterceptionID(event),
				"redisMessageId", message.ID,
			)
		default:
			w.logger.Error(
				"failed to process usage event",
				"eventType", event.Type,
				"eventId", event.EventID,
				"interceptionId", eventInterceptionID(event),
				"redisMessageId", message.ID,
				"error", err,
			)
		}
	}
}

// ProcessUsageEvent persists one decoded usage event and updates aggregate KPI tables.
func (w *Worker) ProcessUsageEvent(ctx context.Context, event UsageEvent, redisMessageID string) error {
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = redisMessageID
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		processed := ProcessedUsageEvent{
			EventID:        event.EventID,
			RedisMessageID: redisMessageID,
			Type:           string(event.Type),
			CreatedAt:      event.CreatedAt,
			ProcessedAt:    time.Now().UTC(),
		}
		if processed.CreatedAt.IsZero() {
			processed.CreatedAt = processed.ProcessedAt
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&processed)
		if result.Error != nil {
			return fmt.Errorf("mark usage event processed: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return errUsageEventAlreadyProcessed
		}
		return processUsageEvent(tx, event)
	})
}

func processUsageEvent(tx *gorm.DB, event UsageEvent) error {
	switch event.Type {
	case UsageEventInterceptionStarted:
		if event.InterceptionStarted == nil {
			return errors.New("missing interception_started payload")
		}
		payload := event.InterceptionStarted
		record := Interception{
			ID:           payload.ID,
			InitiatorID:  payload.InitiatorID,
			Provider:     payload.Provider,
			ProviderType: payload.ProviderType,
			Model:        payload.Model,
			ClientIP:     payload.ClientIP,
			StartedAt:    timestamp(payload.StartedAt),
			Metadata:     payload.Metadata,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
		if result.Error != nil {
			return fmt.Errorf("record interception: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return aggregateInterceptionStarted(tx, record)
	case UsageEventInterceptionEnded:
		if event.InterceptionEnded == nil {
			return errors.New("missing interception_ended payload")
		}
		payload := event.InterceptionEnded
		interception, err := loadUsageInterception(tx, payload.ID)
		if err != nil {
			return err
		}
		if interception.EndedAt != nil {
			return nil
		}
		endedAt := timestamp(payload.EndedAt)
		if err := tx.Model(&Interception{}).Where("id = ?", payload.ID).Update("ended_at", endedAt).Error; err != nil {
			return fmt.Errorf("record interception end: %w", err)
		}
		interception.EndedAt = &endedAt
		return aggregateInterceptionDuration(tx, interception)
	case UsageEventTokenUsage:
		if event.TokenUsage == nil {
			return errors.New("missing token_usage payload")
		}
		payload := event.TokenUsage
		interception, err := loadUsageInterception(tx, payload.InterceptionID)
		if err != nil {
			return err
		}
		record := TokenUsage{
			ID:                    uuid.NewString(),
			InterceptionID:        payload.InterceptionID,
			ProviderResponseID:    payload.ProviderResponseID,
			InputTokens:           payload.InputTokens,
			OutputTokens:          payload.OutputTokens,
			CacheReadInputTokens:  payload.CacheReadInputTokens,
			CacheWriteInputTokens: payload.CacheWriteInputTokens,
			Type:                  payload.Type,
			Metadata:              payload.Metadata,
			CreatedAt:             timestamp(payload.CreatedAt),
		}
		if record.Type == "" {
			record.Type = tokenUsageTypeCompletion
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("record token usage: %w", err)
		}
		return aggregateTokenUsage(tx, interception, record)
	case UsageEventPromptUsage:
		if event.PromptUsage == nil {
			return errors.New("missing prompt_usage payload")
		}
		payload := event.PromptUsage
		interception, err := loadUsageInterception(tx, payload.InterceptionID)
		if err != nil {
			return err
		}
		record := UserPrompt{
			ID:                 uuid.NewString(),
			InterceptionID:     payload.InterceptionID,
			ProviderResponseID: payload.ProviderResponseID,
			Prompt:             payload.Prompt,
			Metadata:           payload.Metadata,
			CreatedAt:          timestamp(payload.CreatedAt),
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("record prompt usage: %w", err)
		}
		return aggregatePromptUsage(tx, interception, record)
	case UsageEventToolUsage:
		if event.ToolUsage == nil {
			return errors.New("missing tool_usage payload")
		}
		payload := event.ToolUsage
		interception, err := loadUsageInterception(tx, payload.InterceptionID)
		if err != nil {
			return err
		}
		record := ToolUsage{
			ID:                 uuid.NewString(),
			InterceptionID:     payload.InterceptionID,
			ProviderResponseID: payload.ProviderResponseID,
			ServerURL:          payload.ServerURL,
			Tool:               payload.Tool,
			Input:              payload.Input,
			Injected:           payload.Injected,
			InvocationError:    payload.InvocationError,
			Metadata:           payload.Metadata,
			CreatedAt:          timestamp(payload.CreatedAt),
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("record tool usage: %w", err)
		}
		return aggregateToolUsage(tx, interception, record)
	default:
		return fmt.Errorf("unknown usage event type %q", event.Type)
	}
}

func (w *Worker) ackAndDelete(ctx context.Context, client *redis.Client, messageID string) {
	if err := client.XAck(ctx, UsageEventsStream, UsageEventsConsumerGroup, messageID).Err(); err != nil {
		w.logger.Error("failed to ack usage event", "redisMessageId", messageID, "error", err)
		return
	}
	if err := client.XDel(ctx, UsageEventsStream, messageID).Err(); err != nil {
		w.logger.Error("failed to delete usage event", "redisMessageId", messageID, "error", err)
	}
}

func (w *Worker) redisClient() *redis.Client {
	if w.store == nil {
		return nil
	}
	return w.store.Client()
}

func ensureUsageConsumerGroup(ctx context.Context, client *redis.Client) error {
	err := client.XGroupCreateMkStream(ctx, UsageEventsStream, UsageEventsConsumerGroup, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("create usage consumer group: %w", err)
}

func usageEventFromMessage(message redis.XMessage) (UsageEvent, error) {
	raw, ok := message.Values[UsageEventPayloadField]
	if !ok {
		return UsageEvent{}, errors.New("missing payload field")
	}
	var payload string
	switch value := raw.(type) {
	case string:
		payload = value
	case []byte:
		payload = string(value)
	default:
		payload = fmt.Sprint(value)
	}
	var event UsageEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return UsageEvent{}, fmt.Errorf("decode payload: %w", err)
	}
	if event.EventID == "" {
		event.EventID = message.ID
	}
	return event, nil
}

func defaultUsageConsumerName() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "worker"
	}
	random := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	return fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), random)
}

func sleepWithContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
