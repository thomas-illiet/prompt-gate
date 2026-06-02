package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"cdr.dev/slog/v3/sloggers/slogtest"
	coderbridge "github.com/coder/aibridge"
	"github.com/coder/aibridge/recorder"
	"go.opentelemetry.io/otel"
)

var embeddingsTestTracer = otel.Tracer("promptgate-runtime-proxy-test")

type embeddingTestRecorder struct {
	mu            sync.Mutex
	intercepts    []recorder.InterceptionRecord
	ended         []recorder.InterceptionRecordEnded
	tokenUsages   []recorder.TokenUsageRecord
	promptUsages  []recorder.PromptUsageRecord
	toolUsages    []recorder.ToolUsageRecord
	modelThoughts []recorder.ModelThoughtRecord
}

// RecordInterception records interception for assertions.
func (r *embeddingTestRecorder) RecordInterception(_ context.Context, req *recorder.InterceptionRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intercepts = append(r.intercepts, *req)
	return nil
}

// RecordInterceptionEnded records interception ended for assertions.
func (r *embeddingTestRecorder) RecordInterceptionEnded(_ context.Context, req *recorder.InterceptionRecordEnded) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ended = append(r.ended, *req)
	return nil
}

// RecordTokenUsage records token usage for assertions.
func (r *embeddingTestRecorder) RecordTokenUsage(_ context.Context, req *recorder.TokenUsageRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokenUsages = append(r.tokenUsages, *req)
	return nil
}

// RecordPromptUsage records prompt usage for assertions.
func (r *embeddingTestRecorder) RecordPromptUsage(_ context.Context, req *recorder.PromptUsageRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.promptUsages = append(r.promptUsages, *req)
	return nil
}

// RecordToolUsage records tool usage for assertions.
func (r *embeddingTestRecorder) RecordToolUsage(_ context.Context, req *recorder.ToolUsageRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.toolUsages = append(r.toolUsages, *req)
	return nil
}

// RecordModelThought records model thought for assertions.
func (r *embeddingTestRecorder) RecordModelThought(_ context.Context, req *recorder.ModelThoughtRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modelThoughts = append(r.modelThoughts, *req)
	return nil
}

// TestOpenAIEmbeddingsForwardAndRecordUsage verifies OpenAI embeddings forward and record usage.
func TestOpenAIEmbeddingsForwardAndRecordUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("expected /v1/embeddings, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer provider-key" {
			t.Fatalf("expected provider auth, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"text-embedding-3-small","usage":{"prompt_tokens":7,"total_tokens":7}}`))
	}))
	t.Cleanup(upstream.Close)

	rec := &embeddingTestRecorder{}
	bridge := newEmbeddingTestBridge(t, newOpenAIProvider("openai", upstream.URL+"/v1", "provider-key"), rec)

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/embeddings", strings.NewReader(`{"model":"text-embedding-3-small","input":"hello"}`))
	req.Header.Set("Authorization", "Bearer promptgate-token")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	bridge.ServeHTTP(recorder, withEmbeddingActor(req))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["model"] != "text-embedding-3-small" {
		t.Fatalf("expected upstream response body unchanged, got %#v", body)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.intercepts) != 1 {
		t.Fatalf("expected one interception, got %d", len(rec.intercepts))
	}
	if rec.intercepts[0].Provider != "openai" || rec.intercepts[0].ProviderName != "openai" || rec.intercepts[0].Model != "text-embedding-3-small" {
		t.Fatalf("unexpected interception: %#v", rec.intercepts[0])
	}
	if len(rec.ended) != 1 || rec.ended[0].ID != rec.intercepts[0].ID {
		t.Fatalf("expected matching interception end, got %#v", rec.ended)
	}
	if len(rec.tokenUsages) != 1 {
		t.Fatalf("expected one token usage record, got %d", len(rec.tokenUsages))
	}
	if rec.tokenUsages[0].InterceptionID != rec.intercepts[0].ID || rec.tokenUsages[0].Input != 7 || rec.tokenUsages[0].Output != 0 {
		t.Fatalf("unexpected token usage: %#v", rec.tokenUsages[0])
	}
	if rec.tokenUsages[0].Metadata["type"] != "embedding" {
		t.Fatalf("expected embedding token usage metadata, got %#v", rec.tokenUsages[0].Metadata)
	}
	if rec.tokenUsages[0].Metadata["token_source"] != embeddingTokenSourceProviderUsage {
		t.Fatalf("expected provider token source, got %#v", rec.tokenUsages[0].Metadata)
	}
	if len(rec.promptUsages) != 0 {
		t.Fatalf("expected no prompt usage records, got %#v", rec.promptUsages)
	}
}

// TestOpenAIEmbeddingsRecordTotalTokensOnlyUsage verifies OpenAI embeddings record total tokens only usage.
func TestOpenAIEmbeddingsRecordTotalTokensOnlyUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"custom-embedding","usage":{"total_tokens":6}}`))
	}))
	t.Cleanup(upstream.Close)

	rec := &embeddingTestRecorder{}
	bridge := newEmbeddingTestBridge(t, newOpenAIProvider("openai", upstream.URL+"/v1", "provider-key"), rec)

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/embeddings", strings.NewReader(`{"model":"custom-embedding","input":"hello world"}`))
	recorder := httptest.NewRecorder()

	bridge.ServeHTTP(recorder, withEmbeddingActor(req))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.tokenUsages) != 1 {
		t.Fatalf("expected one token usage record, got %d", len(rec.tokenUsages))
	}
	if rec.tokenUsages[0].Input != 6 || rec.tokenUsages[0].Output != 0 {
		t.Fatalf("unexpected token usage: %#v", rec.tokenUsages[0])
	}
	if rec.tokenUsages[0].Metadata["token_source"] != embeddingTokenSourceProviderUsage {
		t.Fatalf("expected provider token source, got %#v", rec.tokenUsages[0].Metadata)
	}
}

// TestOpenAIEmbeddingsRecordsProviderUsageMismatchWarning verifies OpenAI embeddings records provider usage mismatch warning.
func TestOpenAIEmbeddingsRecordsProviderUsageMismatchWarning(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"text-embedding-3-small","usage":{"prompt_tokens":7,"total_tokens":8}}`))
	}))
	t.Cleanup(upstream.Close)

	rec := &embeddingTestRecorder{}
	bridge := newEmbeddingTestBridge(t, newOpenAIProvider("openai", upstream.URL+"/v1", "provider-key"), rec)

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/embeddings", strings.NewReader(`{"model":"text-embedding-3-small","input":"hello"}`))
	recorder := httptest.NewRecorder()

	bridge.ServeHTTP(recorder, withEmbeddingActor(req))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.tokenUsages) != 1 {
		t.Fatalf("expected one token usage record, got %d", len(rec.tokenUsages))
	}
	if rec.tokenUsages[0].Input != 7 {
		t.Fatalf("expected prompt_tokens input, got %#v", rec.tokenUsages[0])
	}
	if rec.tokenUsages[0].Metadata["token_warning"] != embeddingTokenWarningTotalMismatch {
		t.Fatalf("expected mismatch warning metadata, got %#v", rec.tokenUsages[0].Metadata)
	}
}

// TestOpenAIEmbeddingsRecordsZeroWithoutUsage verifies OpenAI embeddings records zero without usage.
func TestOpenAIEmbeddingsRecordsZeroWithoutUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"text-embedding-3-small"}`))
	}))
	t.Cleanup(upstream.Close)

	rec := &embeddingTestRecorder{}
	bridge := newEmbeddingTestBridge(t, newOpenAIProvider("openai", upstream.URL+"/v1", "provider-key"), rec)

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/embeddings", strings.NewReader(`{"model":"text-embedding-3-small","input":"hello"}`))
	recorder := httptest.NewRecorder()

	bridge.ServeHTTP(recorder, withEmbeddingActor(req))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.tokenUsages) != 1 {
		t.Fatalf("expected one token usage record, got %d", len(rec.tokenUsages))
	}
	if rec.tokenUsages[0].Input != 0 || rec.tokenUsages[0].Output != 0 {
		t.Fatalf("unexpected token usage: %#v", rec.tokenUsages[0])
	}
	if rec.tokenUsages[0].Metadata["token_source"] != embeddingTokenSourceProviderMissing {
		t.Fatalf("expected missing provider usage token source, got %#v", rec.tokenUsages[0].Metadata)
	}
}

// TestOpenAIEmbeddingsRecordsZeroForUnknownModelWithoutUsage verifies OpenAI embeddings records zero for unknown model without usage.
func TestOpenAIEmbeddingsRecordsZeroForUnknownModelWithoutUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"custom-embedding"}`))
	}))
	t.Cleanup(upstream.Close)

	rec := &embeddingTestRecorder{}
	bridge := newEmbeddingTestBridge(t, newOpenAIProvider("openai", upstream.URL+"/v1", "provider-key"), rec)

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/embeddings", strings.NewReader(`{"model":"custom-embedding","input":"hello"}`))
	recorder := httptest.NewRecorder()

	bridge.ServeHTTP(recorder, withEmbeddingActor(req))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.tokenUsages) != 1 {
		t.Fatalf("expected one token usage record, got %d", len(rec.tokenUsages))
	}
	if rec.tokenUsages[0].Input != 0 || rec.tokenUsages[0].Output != 0 {
		t.Fatalf("unexpected token usage: %#v", rec.tokenUsages[0])
	}
	if rec.tokenUsages[0].Metadata["token_source"] != embeddingTokenSourceProviderMissing {
		t.Fatalf("expected missing provider usage token source, got %#v", rec.tokenUsages[0].Metadata)
	}
}

// TestEmbeddingsUpstreamErrorDoesNotRecordTokenUsage verifies embeddings upstream error does not record token usage.
func TestEmbeddingsUpstreamErrorDoesNotRecordTokenUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad embeddings request"}}`))
	}))
	t.Cleanup(upstream.Close)

	rec := &embeddingTestRecorder{}
	bridge := newEmbeddingTestBridge(t, newOpenAIProvider("openai", upstream.URL+"/v1", "provider-key"), rec)

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/embeddings", strings.NewReader(`{"model":"text-embedding-3-small","input":""}`))
	recorder := httptest.NewRecorder()

	bridge.ServeHTTP(recorder, withEmbeddingActor(req))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.ended) != 1 {
		t.Fatalf("expected interception end on upstream error, got %d", len(rec.ended))
	}
	if len(rec.tokenUsages) != 0 {
		t.Fatalf("expected no token usage on upstream error, got %#v", rec.tokenUsages)
	}
}

// TestOllamaEmbeddingsForwardAndRecordUsage verifies Ollama embeddings forward and record usage.
func TestOllamaEmbeddingsForwardAndRecordUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("expected /v1/embeddings, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer ollama" {
			t.Fatalf("expected default Ollama auth, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"nomic-embed-text","usage":{"prompt_tokens":3,"total_tokens":3},"data":[]}`))
	}))
	t.Cleanup(upstream.Close)

	rec := &embeddingTestRecorder{}
	bridge := newEmbeddingTestBridge(t, newOllamaProvider("ollama-local", upstream.URL+"/v1", ""), rec)

	req := httptest.NewRequest(http.MethodPost, "/ollama-local/v1/embeddings", strings.NewReader(`{"model":"nomic-embed-text","input":"hello"}`))
	recorder := httptest.NewRecorder()

	bridge.ServeHTTP(recorder, withEmbeddingActor(req))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.intercepts) != 1 {
		t.Fatalf("expected one interception, got %d", len(rec.intercepts))
	}
	if rec.intercepts[0].Provider != "ollama" || rec.intercepts[0].ProviderName != "ollama-local" || rec.intercepts[0].Model != "nomic-embed-text" {
		t.Fatalf("unexpected interception: %#v", rec.intercepts[0])
	}
	if len(rec.tokenUsages) != 1 || rec.tokenUsages[0].Input != 3 {
		t.Fatalf("unexpected token usage: %#v", rec.tokenUsages)
	}
	if rec.tokenUsages[0].Metadata["type"] != "embedding" {
		t.Fatalf("expected embedding token usage metadata, got %#v", rec.tokenUsages[0].Metadata)
	}
	if rec.tokenUsages[0].Metadata["token_source"] != embeddingTokenSourceProviderUsage {
		t.Fatalf("expected provider token source, got %#v", rec.tokenUsages[0].Metadata)
	}
}

// newEmbeddingTestBridge creates embedding test bridge.
func newEmbeddingTestBridge(t *testing.T, provider coderbridge.Provider, rec recorder.Recorder) *coderbridge.RequestBridge {
	t.Helper()
	bridge, err := coderbridge.NewRequestBridge(t.Context(), []coderbridge.Provider{provider}, rec, nil, slogtest.Make(t, nil), nil, embeddingsTestTracer)
	if err != nil {
		t.Fatalf("create bridge: %v", err)
	}
	return bridge
}

// withEmbeddingActor adds an authenticated actor to the embedding request.
func withEmbeddingActor(r *http.Request) *http.Request {
	ctx := coderbridge.AsActor(r.Context(), "11111111-1111-1111-1111-111111111111", coderbridge.Metadata{})
	return r.WithContext(ctx)
}
