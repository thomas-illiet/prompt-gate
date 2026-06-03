package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/redisstore"

	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	UsageEventsStream        = "promptgate:usage:events"
	UsageEventsConsumerGroup = "promptgate-workers"
	UsageEventPayloadField   = "payload"
)

type UsageEventType string

const (
	UsageEventInterceptionStarted UsageEventType = "interception_started"
	UsageEventInterceptionEnded   UsageEventType = "interception_ended"
	UsageEventTokenUsage          UsageEventType = "token_usage"
	UsageEventPromptUsage         UsageEventType = "prompt_usage"
	UsageEventToolUsage           UsageEventType = "tool_usage"
)

type UsageEvent struct {
	EventID             string                    `json:"eventId"`
	Type                UsageEventType            `json:"type"`
	CreatedAt           time.Time                 `json:"createdAt"`
	InterceptionStarted *InterceptionStartedEvent `json:"interceptionStarted,omitempty"`
	InterceptionEnded   *InterceptionEndedEvent   `json:"interceptionEnded,omitempty"`
	TokenUsage          *TokenUsageEvent          `json:"tokenUsage,omitempty"`
	PromptUsage         *PromptUsageEvent         `json:"promptUsage,omitempty"`
	ToolUsage           *ToolUsageEvent           `json:"toolUsage,omitempty"`
}

type InterceptionStartedEvent struct {
	ID                    string    `json:"id"`
	InitiatorID           string    `json:"initiatorId"`
	Provider              string    `json:"provider"`
	ProviderType          string    `json:"providerType"`
	Model                 string    `json:"model"`
	ClientIP              string    `json:"clientIp"`
	StartedAt             time.Time `json:"startedAt"`
	Metadata              string    `json:"metadata"`
	ClientSessionID       *string   `json:"clientSessionId,omitempty"`
	Client                string    `json:"client,omitempty"`
	UserAgent             string    `json:"userAgent,omitempty"`
	CorrelatingToolCallID *string   `json:"correlatingToolCallId,omitempty"`
	CredentialKind        string    `json:"credentialKind,omitempty"`
	CredentialHint        string    `json:"credentialHint,omitempty"`
}

type InterceptionEndedEvent struct {
	ID      string    `json:"id"`
	EndedAt time.Time `json:"endedAt"`
}

type TokenUsageEvent struct {
	InterceptionID        string    `json:"interceptionId"`
	ProviderResponseID    string    `json:"providerResponseId"`
	InputTokens           int64     `json:"inputTokens"`
	OutputTokens          int64     `json:"outputTokens"`
	CacheReadInputTokens  int64     `json:"cacheReadInputTokens"`
	CacheWriteInputTokens int64     `json:"cacheWriteInputTokens"`
	Type                  string    `json:"type"`
	Metadata              string    `json:"metadata"`
	CreatedAt             time.Time `json:"createdAt"`
}

type PromptUsageEvent struct {
	InterceptionID     string    `json:"interceptionId"`
	ProviderResponseID string    `json:"providerResponseId"`
	Prompt             string    `json:"prompt"`
	Metadata           string    `json:"metadata"`
	CreatedAt          time.Time `json:"createdAt"`
}

type ToolUsageEvent struct {
	InterceptionID     string    `json:"interceptionId"`
	ProviderResponseID string    `json:"providerResponseId"`
	ServerURL          *string   `json:"serverUrl,omitempty"`
	Tool               string    `json:"tool"`
	Input              string    `json:"input"`
	Injected           bool      `json:"injected"`
	InvocationError    *string   `json:"invocationError,omitempty"`
	Metadata           string    `json:"metadata"`
	CreatedAt          time.Time `json:"createdAt"`
}

type RedisRecorder struct {
	store  *redisstore.Store
	logger *slog.Logger
}

// NewRedisRecorder creates a Redis Stream-backed recorder for proxy usage events.
func NewRedisRecorder(store *redisstore.Store, logger *slog.Logger) *RedisRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisRecorder{store: store, logger: logger}
}

// RecordInterception enqueues the start of a proxied interaction.
func (r *RedisRecorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		r.logEventError(UsageEventInterceptionStarted, req.ID, "", err)
		return nil
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

// RecordPromptUsage enqueues a user prompt observed by the proxy.
func (r *RedisRecorder) RecordPromptUsage(ctx context.Context, req *aibrecorder.PromptUsageRecord) error {
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		r.logEventError(UsageEventPromptUsage, req.InterceptionID, "", err)
		return nil
	}
	event := newUsageEvent(UsageEventPromptUsage)
	event.PromptUsage = &PromptUsageEvent{
		InterceptionID:     req.InterceptionID,
		ProviderResponseID: req.MsgID,
		Prompt:             req.Prompt,
		Metadata:           metadata,
		CreatedAt:          timestamp(req.CreatedAt),
	}
	return r.enqueue(ctx, event)
}

// RecordToolUsage enqueues tool invocation data observed by the proxy.
func (r *RedisRecorder) RecordToolUsage(ctx context.Context, req *aibrecorder.ToolUsageRecord) error {
	metadata, err := marshalMetadata(req.Metadata)
	if err != nil {
		r.logEventError(UsageEventToolUsage, req.InterceptionID, "", err)
		return nil
	}
	input, err := json.Marshal(req.Args)
	if err != nil {
		r.logEventError(UsageEventToolUsage, req.InterceptionID, "", fmt.Errorf("marshal tool input: %w", err))
		return nil
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
	client := r.store.Client()
	if client == nil {
		r.logEventError(event.Type, eventInterceptionID(event), event.EventID, fmt.Errorf("redis store unavailable"))
		return nil
	}
	payload, err := json.Marshal(event)
	if err != nil {
		r.logEventError(event.Type, eventInterceptionID(event), event.EventID, fmt.Errorf("marshal usage event: %w", err))
		return nil
	}
	if err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: UsageEventsStream,
		Values: map[string]any{UsageEventPayloadField: string(payload)},
	}).Err(); err != nil {
		r.logEventError(event.Type, eventInterceptionID(event), event.EventID, err)
	}
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
	case event.PromptUsage != nil:
		return event.PromptUsage.InterceptionID
	case event.ToolUsage != nil:
		return event.ToolUsage.InterceptionID
	default:
		return ""
	}
}
