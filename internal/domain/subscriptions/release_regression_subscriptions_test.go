package subscriptions

import (
	"context"
	"errors"
	"testing"
	"time"

	aibrecorder "github.com/coder/aibridge/recorder"

	"promptgate/backend/internal/platform/usagecontext"
)

type regressionFailOnceRecorder struct {
	noopRecorder
	calls int
	saved int64
}

func (r *regressionFailOnceRecorder) RecordTokenUsage(_ context.Context, u *aibrecorder.TokenUsageRecord) error {
	r.calls++
	if r.calls == 1 {
		return errors.New("simulated durable write failure")
	}
	r.saved += u.Input + u.Output
	return nil
}
func TestRegressionCumulativeUsageMustRecoverFailedDelta(t *testing.T) {
	inner := &regressionFailOnceRecorder{}
	r := NewQuotaRecorder(inner, nil, nil)
	ctx := context.Background()
	if err := r.RecordInterception(ctx, &aibrecorder.InterceptionRecord{ID: "request-1", InitiatorID: "user-1"}); err != nil {
		t.Fatal(err)
	}
	usage := &aibrecorder.TokenUsageRecord{InterceptionID: "request-1", MsgID: "message-1", Input: 100}
	if err := r.RecordTokenUsage(ctx, usage); err == nil {
		t.Fatal("expected injected error")
	}
	usage.Input = 150
	if err := r.RecordTokenUsage(ctx, usage); err != nil {
		t.Fatal(err)
	}
	if inner.saved != 150 {
		t.Fatalf("after recovery durable total=%d, want 150", inner.saved)
	}
}
func TestRegressionCancelledUnlimitedUsageMustStillCount(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	if _, err := service.CreatePlan(context.Background(), PlanInput{Name: "Unlimited", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := NewQuotaRecorder(noopRecorder{}, store, nil)
	if err := r.RecordInterception(context.Background(), &aibrecorder.InterceptionRecord{ID: "request-1", InitiatorID: user.ID}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{InterceptionID: "request-1", Input: 100}); err != nil {
		t.Fatal(err)
	}
	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if status.Used5HTokens != 100 {
		t.Fatalf("cancelled usage count=%d, want 100", status.Used5HTokens)
	}
}

type ambiguousRecorder struct {
	noopRecorder
	attempts []string
	payloads []int64
	accepted map[string]int64
}

func (r *ambiguousRecorder) RecordTokenUsage(ctx context.Context, u *aibrecorder.TokenUsageRecord) error {
	id := usagecontext.EventID(ctx)
	r.attempts = append(r.attempts, id)
	r.payloads = append(r.payloads, u.Input)
	if id == "" {
		return errors.New("event ID missing")
	}
	r.accepted[id] = u.Input
	if len(r.attempts) == 1 {
		return errors.New("write accepted but acknowledgement lost")
	}
	return nil
}
func TestUsageRetriesKeepEventIDAndPayload(t *testing.T) {
	inner := &ambiguousRecorder{accepted: map[string]int64{}}
	r := NewQuotaRecorder(inner, nil, nil)
	ctx := context.Background()
	if err := r.RecordInterception(ctx, &aibrecorder.InterceptionRecord{ID: "retry", InitiatorID: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{InterceptionID: "retry", Input: 100}); err == nil {
		t.Fatal("expected ambiguous failure")
	}
	if err := r.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{InterceptionID: "retry", Input: 150}); err != nil {
		t.Fatal(err)
	}
	if len(inner.attempts) != 3 || inner.attempts[0] != inner.attempts[1] || inner.attempts[1] == inner.attempts[2] || inner.payloads[1] != 100 || inner.payloads[2] != 50 {
		t.Fatalf("unstable retry: %v %v", inner.attempts, inner.payloads)
	}
	var total int64
	for _, value := range inner.accepted {
		total += value
	}
	if total != 150 {
		t.Fatalf("duplicate durable usage: %d", total)
	}
}
func TestEndRetriesLastFailedUsageWithoutAnotherObservation(t *testing.T) {
	inner := &regressionFailOnceRecorder{}
	r := NewQuotaRecorder(inner, nil, nil)
	ctx := context.Background()
	if err := r.RecordInterception(ctx, &aibrecorder.InterceptionRecord{ID: "end", InitiatorID: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordTokenUsage(ctx, &aibrecorder.TokenUsageRecord{InterceptionID: "end", Input: 100}); err == nil {
		t.Fatal("expected failure")
	}
	if err := r.RecordInterceptionEnded(ctx, &aibrecorder.InterceptionRecordEnded{ID: "end"}); err != nil {
		t.Fatal(err)
	}
	if inner.saved != 100 {
		t.Fatalf("last usage lost: %d", inner.saved)
	}
}
