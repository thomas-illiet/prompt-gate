package subscriptions

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	aibrecorder "github.com/coder/aibridge/recorder"
	"github.com/google/uuid"

	"promptgate/backend/internal/platform/usagecontext"
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
	State     *quotaRequestState

	mu                 sync.Mutex
	usageByMessage     map[string]quotaUsageCounters
	persistedByMessage map[string]quotaUsageCounters
	pendingByMessage   map[string]*pendingTokenUsage
}

type pendingTokenUsage struct {
	id       string
	record   aibrecorder.TokenUsageRecord
	counters quotaUsageCounters
}

type quotaUsageCounters struct {
	input      int64
	output     int64
	cacheRead  int64
	cacheWrite int64
}

func NewQuotaRecorder(inner aibrecorder.Recorder, store *RedisStore, logger *slog.Logger) *QuotaRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &QuotaRecorder{inner: inner, store: store, logger: logger}
}

func (r *QuotaRecorder) RecordInterception(ctx context.Context, req *aibrecorder.InterceptionRecord) error {
	storedID := ""
	if req != nil && req.ID != "" && req.InitiatorID != "" {
		state, _ := quotaRequestStateFromContext(ctx)
		r.interceptors.Store(req.ID, &interceptorQuotaActor{
			UserID:             req.InitiatorID,
			CreatedAt:          time.Now().UTC(),
			State:              state,
			usageByMessage:     make(map[string]quotaUsageCounters),
			persistedByMessage: make(map[string]quotaUsageCounters),
			pendingByMessage:   make(map[string]*pendingTokenUsage),
		})
		storedID = req.ID
	}
	if err := r.inner.RecordInterception(ctx, req); err != nil {
		if storedID != "" {
			r.interceptors.Delete(storedID)
		}
		return err
	}
	return nil
}

func (r *QuotaRecorder) RecordInterceptionEnded(ctx context.Context, req *aibrecorder.InterceptionRecordEnded) error {
	var retryErr error
	if req != nil && req.ID != "" {
		id := req.ID
		if raw, ok := r.interceptors.Load(id); ok {
			actor := raw.(*interceptorQuotaActor)
			actor.mu.Lock()
			retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			for key, pending := range actor.pendingByMessage {
				retryErr = errors.Join(retryErr, r.persistTokenUsage(retryCtx, actor, key, pending))
			}
			cancel()
			actor.mu.Unlock()
		}
		time.AfterFunc(5*time.Minute, func() {
			r.interceptors.Delete(id)
		})
	}
	return errors.Join(retryErr, r.inner.RecordInterceptionEnded(ctx, req))
}

func (r *QuotaRecorder) RecordTokenUsage(ctx context.Context, req *aibrecorder.TokenUsageRecord) error {
	if req == nil {
		return r.inner.RecordTokenUsage(ctx, req)
	}
	raw, ok := r.interceptors.Load(req.InterceptionID)
	if !ok {
		return r.inner.RecordTokenUsage(ctx, req)
	}
	actor, ok := raw.(*interceptorQuotaActor)
	if !ok || actor == nil {
		return r.inner.RecordTokenUsage(ctx, req)
	}
	// Serialize observations and confirmations for this request. Quota charging
	// and durable persistence have independent watermarks.
	actor.mu.Lock()
	defer actor.mu.Unlock()
	key := req.MsgID
	if key == "" {
		key = req.InterceptionID
	}
	observed := maxUsageCounters(actor.usageByMessage[key], req)
	quotaDelta := subtractUsage(observed, actor.usageByMessage[key])
	actor.usageByMessage[key] = observed
	tokens := quotaTokenTotal(quotaDelta.input, quotaDelta.output, quotaDelta.cacheRead, quotaDelta.cacheWrite)
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if tokens > 0 && actor.UserID != "" && r.store != nil {
		if actor.State != nil && actor.State.reservation.ID != "" && actor.State.reservation.UserID == actor.UserID {
			actor.State.addRecorderTokens(tokens)
		} else if err := r.store.IncrementUsage(recordCtx, actor.UserID, tokens, req.CreatedAt); err != nil {
			r.logger.Error("failed to increment subscription quota usage", "user_id", actor.UserID, "interception_id", req.InterceptionID, "tokens", tokens, "error", err)
		}
	}
	// Retry the original payload with its original event ID before recording any
	// later delta. An XADD timeout may have accepted that event before returning.
	if pending := actor.pendingByMessage[key]; pending != nil {
		if err := r.persistTokenUsage(recordCtx, actor, key, pending); err != nil {
			return err
		}
	}
	next := maxUsageCounters(actor.persistedByMessage[key], req)
	delta := subtractUsage(next, actor.persistedByMessage[key])
	if quotaTokenTotal(delta.input, delta.output, delta.cacheRead, delta.cacheWrite) == 0 {
		return nil
	}
	normalized := *req
	normalized.Input, normalized.Output = delta.input, delta.output
	normalized.CacheReadInputTokens, normalized.CacheWriteInputTokens = delta.cacheRead, delta.cacheWrite
	pending := &pendingTokenUsage{id: uuid.NewString(), record: normalized, counters: next}
	actor.pendingByMessage[key] = pending
	return r.persistTokenUsage(recordCtx, actor, key, pending)
}

func (r *QuotaRecorder) persistTokenUsage(ctx context.Context, actor *interceptorQuotaActor, key string, pending *pendingTokenUsage) error {
	if err := r.inner.RecordTokenUsage(usagecontext.WithEventID(ctx, pending.id), &pending.record); err != nil {
		return err
	}
	actor.persistedByMessage[key] = pending.counters
	delete(actor.pendingByMessage, key)
	return nil
}

func maxUsageCounters(previous quotaUsageCounters, req *aibrecorder.TokenUsageRecord) quotaUsageCounters {
	return quotaUsageCounters{
		input: maxInt64(previous.input, req.Input), output: maxInt64(previous.output, req.Output),
		cacheRead: maxInt64(previous.cacheRead, req.CacheReadInputTokens), cacheWrite: maxInt64(previous.cacheWrite, req.CacheWriteInputTokens),
	}
}

func subtractUsage(next, previous quotaUsageCounters) quotaUsageCounters {
	return quotaUsageCounters{input: next.input - previous.input, output: next.output - previous.output,
		cacheRead: next.cacheRead - previous.cacheRead, cacheWrite: next.cacheWrite - previous.cacheWrite}
}

func maxInt64(left, right int64) int64 {
	if right > left {
		return right
	}
	return left
}

func quotaTokenTotal(values ...int64) int64 {
	var total int64
	for _, value := range values {
		if value > 0 {
			total = saturatingAdd(total, value)
		}
	}
	return total
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
