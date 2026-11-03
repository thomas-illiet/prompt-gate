package runtime

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"cdr.dev/slog/v3/sloggers/slogtest"
	coderbridge "github.com/coder/aibridge"
)

func TestRegressionAnthropicRedirectMustNotLeakProviderKey(t *testing.T) {
	var leaked atomic.Bool
	var sourceCalls, targetCalls atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls.Add(1)
		if r.Header.Get("X-Api-Key") == "fake-audit-key" {
			leaked.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"msg-123","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"model":"claude-test","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)
	}))
	defer target.Close()
	// A different hostname represents a redirect away from the configured origin.
	destination := strings.Replace(target.URL, "127.0.0.1", "localhost", 1) + "/v1/messages"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls.Add(1)
		http.Redirect(w, r, destination, http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	m := &Manager{opts: Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), BridgeLogger: slogtest.Make(t, nil), Recorder: &embeddingTestRecorder{}, Tracer: embeddingsTestTracer}}
	bridge, err := m.newBridge(t.Context(), []coderbridge.Provider{newAnthropicProvider("audit", upstream.URL, "fake-audit-key")})
	if err != nil {
		t.Fatal(err)
	}
	m.installBridge(bridge)
	defer m.Shutdown(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/audit/v1/messages", strings.NewReader(`{"model":"claude-test","max_tokens":8,"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(coderbridge.AsActor(req.Context(), "11111111-1111-1111-1111-111111111111", coderbridge.Metadata{}))
	response := httptest.NewRecorder()
	m.ServeHTTP(response, req)
	if sourceCalls.Load() == 0 || targetCalls.Load() != 0 || response.Code < 400 {
		t.Fatalf("redirect boundary not exercised: source=%d target=%d status=%d", sourceCalls.Load(), targetCalls.Load(), response.Code)
	}
	if leaked.Load() {
		t.Fatal("redirect to a different hostname received the configured Anthropic API key")
	}
}
