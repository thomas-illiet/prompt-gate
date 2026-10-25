package subscriptions

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/platform/proxylimits"

	"github.com/coder/aibridge/recorder"
)

func TestEstimateQuotaReservationUsesRequestAndOutputBounds(t *testing.T) {
	boundedBody := `{"model":"gpt","max_completion_tokens":10,"n":2}`
	bounded := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(boundedBody))
	got, err := estimateQuotaReservation(bounded, 1<<20)
	if err != nil {
		t.Fatalf("estimate bounded request: %v", err)
	}
	want := int64(3*len(boundedBody) + 20)
	if got != want {
		t.Fatalf("bounded reservation = %d, want %d", got, want)
	}

	unbounded := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", strings.NewReader(`{"model":"gpt"}`))
	if got, err := estimateQuotaReservation(unbounded, 1<<20); err != nil || got != -1 {
		t.Fatalf("unbounded reservation = %d, err=%v; want -1", got, err)
	}

	hostedToolBodies := []string{
		`{"model":"gpt","max_output_tokens":10,"tools":[{"type":"code_interpreter","container":"auto"}]}`,
		`{"model":"gpt","max_output_tokens":10,"tools":[{"type":"shell","environment":{"type":"local"}}]}`,
		`{"model":"gpt","max_output_tokens":10,"tools":[{"type":"web_search"}]}`,
		`{"model":"gpt","max_output_tokens":10,"tools":[{"type":"mcp","connector_id":"connector_dropbox"}]}`,
	}
	for _, body := range hostedToolBodies {
		request := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", strings.NewReader(body))
		if got, err := estimateQuotaReservation(request, 1<<20); err != nil || got != -1 {
			t.Fatalf("hosted tool reservation for %s = %d, err=%v; want -1", body, got, err)
		}
	}

	otherHostedToolRequests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt","max_tokens":1,"web_search_options":{}}`)),
		httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", strings.NewReader(`{"model":"claude","max_tokens":1,"tools":[{"type":"web_search_20250305","name":"web_search"}]}`)),
		httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", strings.NewReader(`{"model":"claude","max_tokens":1,"mcp_servers":[{"type":"url","url":"https://mcp.example"}]}`)),
	}
	for _, request := range otherHostedToolRequests {
		if got, err := estimateQuotaReservation(request, 1<<20); err != nil || got != -1 {
			t.Fatalf("hosted tool reservation for %s = %d, err=%v; want -1", request.URL.Path, got, err)
		}
	}

	clientToolBody := `{"model":"claude","max_tokens":10,"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`
	clientTool := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", strings.NewReader(clientToolBody))
	if got, err := estimateQuotaReservation(clientTool, 1<<20); err != nil || got != int64(3*len(clientToolBody)+10) {
		t.Fatalf("client tool reservation = %d, err=%v; want bounded estimate", got, err)
	}

	embeddingBody := `{"model":"embed","input":"hello"}`
	embedding := httptest.NewRequest(http.MethodPost, "/openai/v1/embeddings", strings.NewReader(embeddingBody))
	if got, err := estimateQuotaReservation(embedding, 1<<20); err != nil || got != int64(3*len(embeddingBody)) {
		t.Fatalf("embedding reservation = %d, err=%v", got, err)
	}

	oversized := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader("too large"))
	if _, err := estimateQuotaReservation(oversized, 3); !errors.Is(err, proxylimits.ErrExceeded) {
		t.Fatalf("oversized reservation error = %v, want ErrExceeded", err)
	}
}

func TestReserveQuotaAtomicallySerializesUnboundedAdmissions(t *testing.T) {
	_, store, user := newSubscriptionTestStore(t)
	quota := int64(100)
	plan := PlanSnapshot{Quota5HTokens: &quota, Quota7DTokens: &quota}

	const contenders = 20
	start := make(chan struct{})
	reservations := make(chan QuotaReservation, contenders)
	var admitted atomic.Int64
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			decision, err := store.ReserveQuota(context.Background(), user.ID, plan, -1)
			if err != nil {
				t.Errorf("reserve unbounded quota: %v", err)
				return
			}
			if decision.Exceeded != nil {
				return
			}
			admitted.Add(1)
			reservations <- decision.Reservation
		}()
	}
	close(start)
	wait.Wait()
	close(reservations)

	if got := admitted.Load(); got != 1 {
		t.Fatalf("admitted %d concurrent unbounded requests, want 1", got)
	}
	for reservation := range reservations {
		if err := store.ReleaseQuotaReservation(context.Background(), reservation); err != nil {
			t.Fatalf("release reservation: %v", err)
		}
	}
}

func TestReserveQuotaAtomicallyCapsConcurrentAdmissions(t *testing.T) {
	_, store, user := newSubscriptionTestStore(t)
	quota := int64(100)
	plan := PlanSnapshot{Quota5HTokens: &quota, Quota7DTokens: &quota}

	const contenders = 20
	start := make(chan struct{})
	reservations := make(chan QuotaReservation, contenders)
	var admitted atomic.Int64
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			decision, err := store.ReserveQuota(context.Background(), user.ID, plan, 30)
			if err != nil {
				t.Errorf("reserve quota: %v", err)
				return
			}
			if decision.Exceeded != nil {
				return
			}
			admitted.Add(1)
			reservations <- decision.Reservation
		}()
	}
	close(start)
	wait.Wait()
	close(reservations)

	if got := admitted.Load(); got != 3 {
		t.Fatalf("admitted %d concurrent requests, want 3", got)
	}
	for reservation := range reservations {
		if err := store.ReleaseQuotaReservation(context.Background(), reservation); err != nil {
			t.Fatalf("release reservation: %v", err)
		}
	}
}

func TestQuotaRecorderCommitsReservationAsActualUsage(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	quotaRecorder := NewQuotaRecorder(noopRecorder{}, store, nil)
	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := quotaRecorder.RecordInterception(r.Context(), &recorder.InterceptionRecord{
			ID:          "reserved-interception",
			InitiatorID: user.ID,
		}); err != nil {
			t.Fatalf("record interception: %v", err)
		}
		if err := quotaRecorder.RecordTokenUsage(r.Context(), &recorder.TokenUsageRecord{
			InterceptionID: "reserved-interception",
			Input:          4,
			Output:         3,
		}); err != nil {
			t.Fatalf("record token usage: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	request := quotaRequest(user.ID)
	request.Body = http.NoBody
	request.ContentLength = 0
	request.Body = io.NopCloser(strings.NewReader(`{"model":"gpt","max_tokens":10}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("request failed: %d %s", response.Code, response.Body.String())
	}

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	if status.Used5HTokens != 7 || status.Used7DTokens != 7 {
		t.Fatalf("reservation was not reconciled to actual usage: %#v", status)
	}
}

func TestQuotaRecorderKeepsHoldAndNormalizesCumulativeStreamUsage(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	quotaRecorder := NewQuotaRecorder(noopRecorder{}, store, nil)
	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := quotaRecorder.RecordInterception(r.Context(), &recorder.InterceptionRecord{
			ID:          "stream-interception",
			InitiatorID: user.ID,
		}); err != nil {
			t.Fatalf("record interception: %v", err)
		}
		if err := quotaRecorder.RecordTokenUsage(r.Context(), &recorder.TokenUsageRecord{
			InterceptionID: "stream-interception",
			MsgID:          "message-1",
			Input:          100,
			Output:         1,
		}); err != nil {
			t.Fatalf("record message start usage: %v", err)
		}

		// The original max-token hold must remain visible while generation is
		// still in flight, even after the first partial usage callback.
		plan := PlanSnapshot{Quota5HTokens: &quota, Quota7DTokens: &quota}
		concurrent, err := store.ReserveQuota(r.Context(), user.ID, plan, 200)
		if err != nil {
			t.Fatalf("reserve concurrent quota: %v", err)
		}
		if concurrent.Exceeded == nil {
			_ = store.ReleaseQuotaReservation(r.Context(), concurrent.Reservation)
			t.Fatal("concurrent request was admitted after a partial stream callback")
		}

		if err := quotaRecorder.RecordTokenUsage(r.Context(), &recorder.TokenUsageRecord{
			InterceptionID: "stream-interception",
			MsgID:          "message-1",
			Output:         240,
		}); err != nil {
			t.Fatalf("record message delta usage: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"message_stop\"}\n\n"))
	}))

	body := `{"model":"claude","max_tokens":800}`
	request := quotaRequest(user.ID)
	request.URL.Path = "/anthropic/v1/messages"
	request.Body = io.NopCloser(strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("request failed: %d %s", response.Code, response.Body.String())
	}

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	if status.Used5HTokens != 340 || status.Used7DTokens != 340 {
		t.Fatalf("cumulative stream usage = %#v, want 340", status)
	}
}

func TestPartialAnthropicStreamKeepsConservativeCharge(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	quotaRecorder := NewQuotaRecorder(noopRecorder{}, store, nil)
	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := quotaRecorder.RecordInterception(r.Context(), &recorder.InterceptionRecord{ID: "aborted-stream", InitiatorID: user.ID}); err != nil {
			t.Fatalf("record interception: %v", err)
		}
		if err := quotaRecorder.RecordTokenUsage(r.Context(), &recorder.TokenUsageRecord{
			InterceptionID: "aborted-stream", MsgID: "message-1", Input: 100,
		}); err != nil {
			t.Fatalf("record partial input usage: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"message_start\"}\n\n"))
		// No message_stop: this models a client/upstream abort before Anthropic's
		// final cumulative output callback.
	}))

	body := `{"model":"claude","max_tokens":800}`
	request := quotaRequest(user.ID)
	request.URL.Path = "/anthropic/v1/messages"
	request.Body = io.NopCloser(strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	want := int64(3*len(body) + 800)
	if status.Used5HTokens != want || status.Used7DTokens != want {
		t.Fatalf("partial stream charge = %#v, want conservative %d", status, want)
	}
}

func TestResponsesStreamUsageReconcilesUnboundedReservation(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	handler := MiddlewareWithOptions(store, nil, MiddlewareOptions{
		MaxBufferedResponseBytes: 1 << 20,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":13,"))
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("\"output_tokens\":4,\"total_tokens\":17}}}\n\n"))
	}))

	request := quotaRequest(user.ID)
	request.URL.Path = "/openai/v1/responses"
	request.Body = io.NopCloser(strings.NewReader(`{"model":"gpt","stream":true}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("request failed: %d %s", response.Code, response.Body.String())
	}

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	if status.Used5HTokens != 17 || status.Used7DTokens != 17 {
		t.Fatalf("Responses stream usage = %#v, want 17", status)
	}
}

func TestResponsesStreamWithoutCompletedUsageKeepsConservativeCharge(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	handler := MiddlewareWithOptions(store, nil, MiddlewareOptions{
		MaxBufferedResponseBytes: 16,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: a response event larger than the observer bound\n\n"))
	}))
	request := quotaRequest(user.ID)
	request.URL.Path = "/openai/v1/responses"
	request.Body = io.NopCloser(strings.NewReader(`{"model":"gpt","stream":true}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	if status.Used5HTokens != quota || status.Used7DTokens != quota {
		t.Fatalf("missing stream usage charged %#v, want conservative %d", status, quota)
	}
}

func TestSuccessfulResponseWithoutUsageCommitsConservativeReservation(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	body := `{"model":"gpt","max_tokens":10}`
	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("quota response writer dropped streaming support")
		}
		w.WriteHeader(http.StatusOK)
	}))
	request := quotaRequest(user.ID)
	request.Body = io.NopCloser(strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	want := int64(3*len(body) + 10)
	if status.Used5HTokens != want || status.Used7DTokens != want {
		t.Fatalf("fallback usage = %#v, want %d", status, want)
	}
}

func TestFailedResponseReleasesReservation(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	handler := Middleware(store, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, quotaRequest(user.ID))

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	if status.Used5HTokens != 0 || status.Used7DTokens != 0 {
		t.Fatalf("failed response consumed quota: %#v", status)
	}
}

func TestCancelledRequestCommitsConservativeReservation(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	quota := int64(1_000)
	if _, err := service.CreatePlan(context.Background(), PlanInput{
		Name:          "Default",
		Quota5HTokens: &quota,
		Quota7DTokens: &quota,
		IsDefault:     true,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := store.WarmSnapshot(context.Background()); err != nil {
		t.Fatalf("warm snapshot: %v", err)
	}

	request := quotaRequest(user.ID)
	body := `{"model":"gpt","max_tokens":10}`
	request.Body = io.NopCloser(strings.NewReader(body))
	requestContext, cancel := context.WithCancel(request.Context())
	request = request.WithContext(requestContext)
	handler := Middleware(store, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		// Simulate the client disappearing after provider dispatch but before any
		// upstream response headers reach Prompt Gate.
		cancel()
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	status, err := store.CurrentQuota(context.Background(), user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load quota: %v", err)
	}
	want := int64(3*len(body) + 10)
	if status.Used5HTokens != want || status.Used7DTokens != want {
		t.Fatalf("cancelled request usage = %#v, want conservative %d", status, want)
	}
}

func TestUnboundedRequestHoldsAllRemainingQuotaUntilReturn(t *testing.T) {
	_, store, user := newSubscriptionTestStore(t)
	quota := int64(100)
	plan := PlanSnapshot{Quota5HTokens: &quota, Quota7DTokens: &quota}

	first, err := store.ReserveQuota(context.Background(), user.ID, plan, -1)
	if err != nil || first.Exceeded != nil {
		t.Fatalf("reserve all remaining quota: decision=%#v err=%v", first, err)
	}
	if first.Reservation.Tokens != quota {
		t.Fatalf("reserved %d tokens, want %d", first.Reservation.Tokens, quota)
	}
	second, err := store.ReserveQuota(context.Background(), user.ID, plan, 1)
	if err != nil {
		t.Fatalf("second reservation: %v", err)
	}
	if second.Exceeded == nil {
		t.Fatal("second request was admitted while unbounded request held remaining quota")
	}
	if err := store.ReleaseQuotaReservation(context.Background(), first.Reservation); err != nil {
		t.Fatalf("release first reservation: %v", err)
	}
}
