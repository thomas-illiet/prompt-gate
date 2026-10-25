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
	State     *quotaRequestState

	mu             sync.Mutex
	usageByMessage map[string]quotaUsageCounters
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
			UserID:         req.InitiatorID,
			CreatedAt:      time.Now().UTC(),
			State:          state,
			usageByMessage: make(map[string]quotaUsageCounters),
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
			actor, _ := raw.(*interceptorQuotaActor)
			if actor == nil {
				return r.inner.RecordTokenUsage(ctx, req)
			}
			tokens := actor.observeTokenUsage(req)
			if tokens > 0 && actor.UserID != "" && r.store != nil {
				when := req.CreatedAt
				if when.IsZero() {
					when = time.Now().UTC()
				}
				var err error
				if actor.State != nil && actor.State.reservation.ID != "" && actor.State.reservation.UserID == actor.UserID {
					// Keep the Redis hold intact for the whole upstream request. Streaming
					// providers emit partial/cumulative callbacks before generation ends;
					// the middleware commits the aggregate only after the bridge has waited
					// for all asynchronous recorder calls.
					actor.State.addRecorderTokens(tokens)
				} else {
					err = r.store.IncrementUsage(ctx, actor.UserID, tokens, when)
				}
				if err != nil {
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

// observeTokenUsage converts repeated cumulative provider callbacks into the
// positive delta for one message. Anthropic streaming, for example, reports
// output=1 at message_start and output=240 at message_delta; charging both raw
// values would over-count by one token.
func (a *interceptorQuotaActor) observeTokenUsage(req *aibrecorder.TokenUsageRecord) int64 {
	if a == nil || req == nil {
		return 0
	}
	key := req.MsgID
	if key == "" {
		key = req.InterceptionID
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	previous := a.usageByMessage[key]
	next := previous
	next.input = maxInt64(next.input, req.Input)
	next.output = maxInt64(next.output, req.Output)
	next.cacheRead = maxInt64(next.cacheRead, req.CacheReadInputTokens)
	next.cacheWrite = maxInt64(next.cacheWrite, req.CacheWriteInputTokens)
	a.usageByMessage[key] = next
	return quotaTokenTotal(
		next.input-previous.input,
		next.output-previous.output,
		next.cacheRead-previous.cacheRead,
		next.cacheWrite-previous.cacheWrite,
	)
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
