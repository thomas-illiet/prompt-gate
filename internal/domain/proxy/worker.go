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
)

var errUsageEventAlreadyProcessed = errors.New("usage event already processed")

var ackDeleteUsageScript = redis.NewScript(`
for _, id in ipairs(ARGV) do
 redis.call("XACK", KEYS[1], KEYS[2], id)
 redis.call("XDEL", KEYS[1], id)
end
return #ARGV
`)

type WorkerOptions struct {
	ConsumerName       string
	BatchSize          int64
	BlockTimeout       time.Duration
	PendingIdleTimeout time.Duration
}

type Worker struct {
	db            *gorm.DB
	store         *redisstore.Store
	logger        *slog.Logger
	opts          WorkerOptions
	pendingCursor string
}

// UsageEventMessage couples a decoded usage event to its Redis stream ID.
type UsageEventMessage struct {
	Event          UsageEvent
	RedisMessageID string
}

// UsageEventResult reports whether a message can be acknowledged after batching.
type UsageEventResult struct {
	RedisMessageID string
	Err            error
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
	start := w.pendingCursor
	if start == "" {
		start = "0-0"
	}
	messages, next, err := client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   UsageEventsStream,
		Group:    UsageEventsConsumerGroup,
		Consumer: w.opts.ConsumerName,
		MinIdle:  w.opts.PendingIdleTimeout,
		Start:    start,
		Count:    w.opts.BatchSize,
	}).Result()
	if errors.Is(err, redis.Nil) {
		w.pendingCursor = "0-0"
		return nil, nil
	}
	if err == nil {
		w.pendingCursor = next
		if next == "0-0" {
			// A complete pass is finished; the next scan starts at the beginning.
			w.pendingCursor = "0-0"
		}
	}
	return messages, err
}

func (w *Worker) handleMessages(ctx context.Context, client *redis.Client, messages []redis.XMessage) {
	ackIDs := make([]string, 0, len(messages))
	decoded := make([]UsageEventMessage, 0, len(messages))
	for _, message := range messages {
		event, err := usageEventFromMessage(message)
		if err != nil {
			w.logger.Error("dropping invalid usage event", "redisMessageId", message.ID, "error", err)
			ackIDs = append(ackIDs, message.ID)
			continue
		}
		decoded = append(decoded, UsageEventMessage{Event: event, RedisMessageID: message.ID})
	}
	for _, result := range w.ProcessUsageBatch(ctx, decoded) {
		if result.Err == nil || errors.Is(result.Err, errUsageEventAlreadyProcessed) {
			ackIDs = append(ackIDs, result.RedisMessageID)
			continue
		}
		if errors.Is(result.Err, ErrUsageEventDependencyMissing) {
			w.logger.Warn("usage event dependency missing; leaving message pending", "redisMessageId", result.RedisMessageID)
			continue
		}
		w.logger.Error("failed to process usage event", "redisMessageId", result.RedisMessageID, "error", result.Err)
	}
	if len(ackIDs) > 0 {
		w.ackAndDeleteBatch(ctx, client, ackIDs)
	}
}

// ProcessUsageEvent persists one decoded usage event and updates aggregate KPI tables.
func (w *Worker) ProcessUsageEvent(ctx context.Context, event UsageEvent, redisMessageID string) error {
	results := w.ProcessUsageBatch(ctx, []UsageEventMessage{{Event: event, RedisMessageID: redisMessageID}})
	if len(results) == 0 {
		return nil
	}
	return results[0].Err
}

// ProcessUsageBatch persists a batch in one transaction. Savepoints isolate
// dependency failures so unrelated events can still commit and be acknowledged.
func (w *Worker) ProcessUsageBatch(ctx context.Context, messages []UsageEventMessage) []UsageEventResult {
	results := make([]UsageEventResult, len(messages))
	for i, message := range messages {
		results[i].RedisMessageID = message.RedisMessageID
	}
	if len(messages) == 0 {
		return results
	}
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, message := range messages {
			name := fmt.Sprintf("usage_event_%d", i)
			if err := tx.SavePoint(name).Error; err != nil {
				return err
			}
			err := processUsageEventTx(tx, message.Event, message.RedisMessageID)
			results[i].Err = err
			if err != nil {
				if rollbackErr := tx.RollbackTo(name).Error; rollbackErr != nil {
					return rollbackErr
				}
			}
		}
		return nil
	})
	if err != nil {
		for i := range results {
			if results[i].Err == nil {
				results[i].Err = err
			}
		}
	}
	return results
}

func (w *Worker) ackAndDeleteBatch(ctx context.Context, client *redis.Client, messageIDs []string) {
	if len(messageIDs) == 0 {
		return
	}
	args := make([]any, len(messageIDs))
	for i, id := range messageIDs {
		args[i] = id
	}
	if err := ackDeleteUsageScript.Run(ctx, client,
		[]string{UsageEventsStream, UsageEventsConsumerGroup}, args...).Err(); err != nil {
		w.logger.Error("failed to ack and delete usage events", "count", len(messageIDs), "error", err)
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
