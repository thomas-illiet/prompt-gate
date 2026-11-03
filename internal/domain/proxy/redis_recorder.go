package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/requestprivacy"
	"promptgate/backend/internal/platform/usagecontext"
)

const maxPolicyDenialReasonCharacters = 2048

type RedisRecorder struct {
	store    *redisstore.Store
	logger   *slog.Logger
	fallback *gorm.DB
}

// NewRedisRecorder creates a Redis Stream-backed recorder for proxy usage events.
func NewRedisRecorder(store *redisstore.Store, logger *slog.Logger) *RedisRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisRecorder{store: store, logger: logger}
}

// WithDurableFallback retains events in PostgreSQL when Redis cannot accept
// them. Configure this before using the recorder concurrently.
func (r *RedisRecorder) WithDurableFallback(db *gorm.DB) *RedisRecorder {
	r.fallback = db
	return r
}

// RecordInterception enqueues the start of a proxied interaction.
func (r *RedisRecorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		r.logEventError(UsageEventInterceptionStarted, req.ID, "", err)
		return err
	}
	event := newUsageEvent(UsageEventInterceptionStarted)
	event.InterceptionStarted = &InterceptionStartedEvent{
		ID:                    req.ID,
		InitiatorID:           req.InitiatorID,
		Provider:              req.ProviderName,
		ProviderType:          req.Provider,
		Model:                 req.Model,
		ClientIP:              clientip.FromContext(ctx),
		StartedAt:             timestamp(req.StartedAt),
		Metadata:              metadata,
		ClientSessionID:       req.ClientSessionID,
		Client:                req.Client,
		UserAgent:             req.UserAgent,
		CorrelatingToolCallID: req.CorrelatingToolCallID,
		CredentialKind:        req.CredentialKind,
		CredentialHint:        req.CredentialHint,
	}
	return r.enqueue(ctx, event)
}

// RecordPolicyDenial enqueues the latest authenticated OPA denial for an account IP.
func (r *RedisRecorder) RecordPolicyDenial(ctx context.Context, accountID, clientIP, reason string, deniedAt time.Time) error {
	if requestprivacy.IsMetadataOnly(ctx) {
		return nil
	}
	event := newUsageEvent(UsageEventPolicyDenied)
	event.PolicyDenied = &PolicyDeniedEvent{
		AccountID: accountID,
		ClientIP:  clientIP,
		Reason:    truncatePolicyDenialReason(reason),
		DeniedAt:  timestamp(deniedAt),
	}
	return r.enqueue(ctx, event)
}

func truncatePolicyDenialReason(reason string) string {
	if utf8.RuneCountInString(reason) <= maxPolicyDenialReasonCharacters {
		return reason
	}
	runes := []rune(reason)
	return string(runes[:maxPolicyDenialReasonCharacters])
}

// RecordInterceptionEnded enqueues completion data for a proxied interaction.
func (r *RedisRecorder) RecordInterceptionEnded(ctx context.Context, req *aibrecorder.InterceptionRecordEnded) error {
	event := newUsageEvent(UsageEventInterceptionEnded)
	event.InterceptionEnded = &InterceptionEndedEvent{
		ID:      req.ID,
		EndedAt: timestamp(req.EndedAt),
	}
	return r.enqueue(ctx, event)
}

// RecordTokenUsage enqueues token usage for a proxied request.
func (r *RedisRecorder) RecordTokenUsage(ctx context.Context, req *aibrecorder.TokenUsageRecord) error {
	event := newUsageEvent(UsageEventTokenUsage)
	if id := usagecontext.EventID(ctx); id != "" {
		event.EventID = id
	}
	event.TokenUsage = &TokenUsageEvent{
		InterceptionID:        req.InterceptionID,
		ProviderResponseID:    req.MsgID,
		InputTokens:           req.Input,
		OutputTokens:          req.Output,
		CacheReadInputTokens:  req.CacheReadInputTokens,
		CacheWriteInputTokens: req.CacheWriteInputTokens,
		Type:                  metadataTokenUsageType(req.Metadata),
		Metadata:              mergeMetadata(req.Metadata, req.ExtraTokenTypes),
		CreatedAt:             timestamp(req.CreatedAt),
	}
	return r.enqueue(ctx, event)
}

// RecordPromptUsage satisfies the recorder interface without collecting prompts.
func (r *RedisRecorder) RecordPromptUsage(_ context.Context, _ *aibrecorder.PromptUsageRecord) error {
	return nil
}

// RecordToolUsage enqueues tool invocation data observed by the proxy.
func (r *RedisRecorder) RecordToolUsage(ctx context.Context, req *aibrecorder.ToolUsageRecord) error {
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		r.logEventError(UsageEventToolUsage, req.InterceptionID, "", err)
		return err
	}
	input, err := json.Marshal(req.Args)
	if err != nil {
		r.logEventError(UsageEventToolUsage, req.InterceptionID, "", fmt.Errorf("marshal tool input: %w", err))
		return fmt.Errorf("marshal tool input: %w", err)
	}
	var invocationError *string
	if req.InvocationError != nil {
		value := req.InvocationError.Error()
		invocationError = &value
	}
	event := newUsageEvent(UsageEventToolUsage)
	event.ToolUsage = &ToolUsageEvent{
		InterceptionID:     req.InterceptionID,
		ProviderResponseID: req.MsgID,
		ServerURL:          req.ServerURL,
		Tool:               req.Tool,
		Input:              string(input),
		Injected:           req.Injected,
		InvocationError:    invocationError,
		Metadata:           metadata,
		CreatedAt:          timestamp(req.CreatedAt),
	}
	return r.enqueue(ctx, event)
}

// RecordModelThought intentionally keeps model thoughts out of persistent usage storage.
func (r *RedisRecorder) RecordModelThought(_ context.Context, _ *aibrecorder.ModelThoughtRecord) error {
	return nil
}

func newUsageEvent(eventType UsageEventType) UsageEvent {
	return UsageEvent{
		EventID:   uuid.NewString(),
		Type:      eventType,
		CreatedAt: time.Now().UTC(),
	}
}

func (r *RedisRecorder) enqueue(ctx context.Context, event UsageEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		r.logEventError(event.Type, eventInterceptionID(event), event.EventID, fmt.Errorf("marshal usage event: %w", err))
		return fmt.Errorf("marshal usage event: %w", err)
	}
	client := r.store.Client()
	err = errors.New("redis store unavailable")
	if client != nil {
		err = client.XAdd(ctx, &redis.XAddArgs{
			Stream: UsageEventsStream,
			Values: map[string]any{UsageEventPayloadField: string(payload)},
		}).Err()
	}
	if err == nil {
		return nil
	}
	r.logEventError(event.Type, eventInterceptionID(event), event.EventID, err)
	if r.fallback == nil {
		return fmt.Errorf("enqueue usage event: %w", err)
	}
	// A client cancellation or Redis timeout must not cancel durable recording.
	// Keep the event ID unchanged: an ambiguous XADD may already have succeeded.
	fallbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	pending := PendingUsageEvent{EventID: event.EventID, Payload: string(payload), CreatedAt: event.CreatedAt}
	if fallbackErr := r.fallback.WithContext(fallbackCtx).Clauses(clause.OnConflict{DoNothing: true}).Create(&pending).Error; fallbackErr != nil {
		return errors.Join(fmt.Errorf("enqueue usage event: %w", err), fmt.Errorf("persist usage event fallback: %w", fallbackErr))
	}
	r.logger.Warn("usage event retained in PostgreSQL for retry", "eventId", event.EventID, "eventType", event.Type)
	return nil
}

func (r *RedisRecorder) logEventError(eventType UsageEventType, interceptionID string, eventID string, err error) {
	if r.logger == nil {
		return
	}
	r.logger.Error(
		"failed to enqueue proxy usage event",
		"eventType", eventType,
		"interceptionId", interceptionID,
		"eventId", eventID,
		"error", err,
	)
}

func eventInterceptionID(event UsageEvent) string {
	switch {
	case event.InterceptionStarted != nil:
		return event.InterceptionStarted.ID
	case event.InterceptionEnded != nil:
		return event.InterceptionEnded.ID
	case event.TokenUsage != nil:
		return event.TokenUsage.InterceptionID
	case event.ToolUsage != nil:
		return event.ToolUsage.InterceptionID
	case event.PolicyDenied != nil:
		return event.PolicyDenied.AccountID
	default:
		return ""
	}
}
