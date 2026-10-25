package runtime

import (
	"context"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cdr.dev/slog/v3/sloggers/slogtest"
	coderbridge "github.com/coder/aibridge"

	platformhttp "promptgate/backend/internal/platform/httpclient"
)

// TestProviderRoutesUseConfiguredPrivateCA verifies every provider transport
// path uses the common.tls.ca_file-backed client, including SDK-intercepted and
// raw passthrough routes.
func TestProviderRoutesUseConfiguredPrivateCA(t *testing.T) {
	type upstreamRequest struct {
		method         string
		path           string
		authorization  string
		apiKey         string
		acceptEncoding string
	}
	requests := make(chan upstreamRequest, 8)

	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- upstreamRequest{
			method:         r.Method,
			path:           r.URL.Path,
			authorization:  r.Header.Get("Authorization"),
			apiKey:         r.Header.Get("X-Api-Key"),
			acceptEncoding: r.Header.Get("Accept-Encoding"),
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat/completions":
			_, _ = io.WriteString(w, `{"id":"chatcmpl-123","object":"chat.completion","created":1677652288,"model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":12,"total_tokens":21}}`)
		case "/v1/responses":
			_, _ = io.WriteString(w, `{"id":"resp-123","object":"response","created_at":1677652288,"model":"gpt-5","output":[],"usage":{"input_tokens":5,"output_tokens":10,"total_tokens":15}}`)
		case "/v1/messages":
			_, _ = io.WriteString(w, `{"id":"msg-123","type":"message","role":"assistant","content":[{"type":"text","text":"Hello!"}],"model":"claude-opus-4-5","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
		case "/v1/embeddings":
			_, _ = io.WriteString(w, `{"object":"list","data":[],"model":"text-embedding-3-small","usage":{"prompt_tokens":2,"total_tokens":2}}`)
		case "/v1/messages/count_tokens":
			_, _ = io.WriteString(w, `{"input_tokens":3}`)
		case "/v1/models":
			_, _ = io.WriteString(w, `{"object":"list","data":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	caFile := privateCATestFile(t, upstream)
	client, err := platformhttp.NewWithCAFile(caFile, 5*time.Second)
	if err != nil {
		t.Fatalf("create private-CA client: %v", err)
	}

	rec := &embeddingTestRecorder{}
	manager := &Manager{opts: Options{
		HTTPClient:   client,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		BridgeLogger: slogtest.Make(t, nil),
		Recorder:     rec,
		Tracer:       embeddingsTestTracer,
	}}
	providers := []coderbridge.Provider{
		newOpenAIProvider("openai-private", upstream.URL+"/v1", "openai-key", providerRuntimeOptions{httpClient: client}),
		newOllamaProvider("ollama-private", upstream.URL+"/v1", "ollama-key", providerRuntimeOptions{httpClient: client}),
		newAnthropicProvider("anthropic-private", upstream.URL, "anthropic-key"),
	}
	bridge, err := manager.newBridge(t.Context(), providers)
	if err != nil {
		t.Fatalf("create bridge: %v", err)
	}
	manager.installBridge(bridge)
	t.Cleanup(func() {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown bridge: %v", err)
		}
	})

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		upstreamPath   string
		authorization  string
		apiKey         string
		acceptEncoding string
	}{
		{
			name:          "OpenAI chat",
			method:        http.MethodPost,
			path:          "/openai-private/v1/chat/completions",
			body:          `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`,
			upstreamPath:  "/v1/chat/completions",
			authorization: "Bearer openai-key",
		},
		{
			name:          "OpenAI responses",
			method:        http.MethodPost,
			path:          "/openai-private/v1/responses",
			body:          `{"model":"gpt-5","input":"hello"}`,
			upstreamPath:  "/v1/responses",
			authorization: "Bearer openai-key",
		},
		{
			name:           "OpenAI models passthrough",
			method:         http.MethodGet,
			path:           "/openai-private/v1/models",
			upstreamPath:   "/v1/models",
			authorization:  "Bearer openai-key",
			acceptEncoding: "identity",
		},
		{
			name:          "OpenAI embeddings",
			method:        http.MethodPost,
			path:          "/openai-private/v1/embeddings",
			body:          `{"model":"text-embedding-3-small","input":"hello"}`,
			upstreamPath:  "/v1/embeddings",
			authorization: "Bearer openai-key",
		},
		{
			name:          "Ollama chat",
			method:        http.MethodPost,
			path:          "/ollama-private/v1/chat/completions",
			body:          `{"model":"llama3.2","messages":[{"role":"user","content":"hello"}]}`,
			upstreamPath:  "/v1/chat/completions",
			authorization: "Bearer ollama-key",
		},
		{
			name:           "Ollama models passthrough",
			method:         http.MethodGet,
			path:           "/ollama-private/v1/models",
			upstreamPath:   "/v1/models",
			authorization:  "Bearer ollama-key",
			acceptEncoding: "identity",
		},
		{
			name:          "Ollama embeddings",
			method:        http.MethodPost,
			path:          "/ollama-private/v1/embeddings",
			body:          `{"model":"nomic-embed-text","input":"hello"}`,
			upstreamPath:  "/v1/embeddings",
			authorization: "Bearer ollama-key",
		},
		{
			name:         "Anthropic messages",
			method:       http.MethodPost,
			path:         "/anthropic-private/v1/messages",
			body:         `{"model":"claude-opus-4-5","max_tokens":8,"messages":[{"role":"user","content":"hello"}]}`,
			upstreamPath: "/v1/messages",
			apiKey:       "anthropic-key",
		},
		{
			name:           "Anthropic models passthrough",
			method:         http.MethodGet,
			path:           "/anthropic-private/v1/models",
			upstreamPath:   "/v1/models",
			apiKey:         "anthropic-key",
			acceptEncoding: "identity",
		},
		{
			name:           "Anthropic count tokens passthrough",
			method:         http.MethodPost,
			path:           "/anthropic-private/v1/messages/count_tokens",
			body:           `{"model":"claude-opus-4-5","messages":[{"role":"user","content":"hello"}]}`,
			upstreamPath:   "/v1/messages/count_tokens",
			apiKey:         "anthropic-key",
			acceptEncoding: "identity",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(coderbridge.AsActor(
				request.Context(),
				"11111111-1111-1111-1111-111111111111",
				coderbridge.Metadata{},
			))
			response := httptest.NewRecorder()

			manager.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200: %s", response.Code, response.Body.String())
			}
			select {
			case got := <-requests:
				if got.method != tc.method || got.path != tc.upstreamPath {
					t.Fatalf("upstream target: got %s %s, want %s %s", got.method, got.path, tc.method, tc.upstreamPath)
				}
				if got.authorization != tc.authorization || got.apiKey != tc.apiKey {
					t.Fatalf("upstream auth: got Authorization=%q X-Api-Key=%q, want Authorization=%q X-Api-Key=%q", got.authorization, got.apiKey, tc.authorization, tc.apiKey)
				}
				if tc.acceptEncoding != "" && got.acceptEncoding != tc.acceptEncoding {
					t.Fatalf("upstream Accept-Encoding: got %q, want %q", got.acceptEncoding, tc.acceptEncoding)
				}
			case <-time.After(time.Second):
				t.Fatal("private-CA upstream did not receive request")
			}
		})
	}
}

func privateCATestFile(t *testing.T, server *httptest.Server) string {
	t.Helper()
	block := &pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}
	path := t.TempDir() + "/ca.pem"
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write private CA: %v", err)
	}
	return path
}
