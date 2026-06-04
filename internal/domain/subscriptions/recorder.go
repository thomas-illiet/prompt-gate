package subscriptions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	aibrecorder "github.com/coder/aibridge/recorder"
)

type QuotaRecorder struct {
	inner        aibrecorder.Recorder
	store        *RedisStore
	logger       *slog.Logger
	interceptors sync.Map
}

type interceptorQuotaActor struct {
	UserID    string
	CreatedAt time.Time
}

func NewQuotaRecorder(inner aibrecorder.Recorder, store *RedisStore, logger *slog.Logger) *QuotaRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &QuotaRecorder{inner: inner, store: store, logger: logger}
}

func (r *QuotaRecorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	if req != nil && req.ID != "" && req.InitiatorID != "" {
		r.interceptors.Store(req.ID, interceptorQuotaActor{
			UserID:    req.InitiatorID,
			CreatedAt: time.Now().UTC(),
		})
	}
	return r.inner.RecordInterception(ctx, req)
}

func (r *QuotaRecorder) RecordInterceptionEnded(ctx context.Context, req *aibrecorder.InterceptionRecordEnded) error {
	if req != nil && req.ID != "" {
		id := req.ID
		time.AfterFunc(5*time.Minute, func() {
			r.interceptors.Delete(id)
		})
	}
	return r.inner.RecordInterceptionEnded(ctx, req)
}

func (r *QuotaRecorder) RecordTokenUsage(ctx context.Context, req *aibrecorder.TokenUsageRecord) error {
	if req != nil {
		if raw, ok := r.interceptors.Load(req.InterceptionID); ok {
			actor, _ := raw.(interceptorQuotaActor)
			tokens := req.Input + req.Output + req.CacheReadInputTokens + req.CacheWriteInputTokens
			if tokens > 0 && actor.UserID != "" && r.store != nil {
				when := req.CreatedAt
				if when.IsZero() {
					when = time.Now().UTC()
				}
				if err := r.store.IncrementUsage(ctx, actor.UserID, tokens, when); err != nil {
					r.logger.Error(
						"failed to increment subscription quota usage",
						"user_id", actor.UserID,
						"interception_id", req.InterceptionID,
						"tokens", tokens,
						"error", err,
					)
				}
			}
		}
	}
	return r.inner.RecordTokenUsage(ctx, req)
}

func (r *QuotaRecorder) RecordPromptUsage(ctx context.Context, req *aibrecorder.PromptUsageRecord) error {
	return r.inner.RecordPromptUsage(ctx, req)
}

func (r *QuotaRecorder) RecordToolUsage(ctx context.Context, req *aibrecorder.ToolUsageRecord) error {
	return r.inner.RecordToolUsage(ctx, req)
}

func (r *QuotaRecorder) RecordModelThought(ctx context.Context, req *aibrecorder.ModelThoughtRecord) error {
	return r.inner.RecordModelThought(ctx, req)
}
