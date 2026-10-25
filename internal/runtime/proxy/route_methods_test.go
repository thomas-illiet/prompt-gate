package runtime

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMethodGuardRejectsUnsupportedMethodsBeforeReadingBody(t *testing.T) {
	tests := []struct {
		method      string
		path        string
		allowMethod string
	}{
		{method: http.MethodGet, path: "/openai/v1/chat/completions", allowMethod: http.MethodPost},
		{method: http.MethodDelete, path: "/openai/v1/embeddings", allowMethod: http.MethodPost},
		{method: http.MethodPatch, path: "/anthropic/v1/messages", allowMethod: http.MethodPost},
		{method: http.MethodDelete, path: "/openai/v1/responses", allowMethod: http.MethodPost},
		{method: http.MethodDelete, path: "/openai/v1/models", allowMethod: http.MethodGet},
		{method: http.MethodPost, path: "/openai/v1/models", allowMethod: http.MethodGet},
		{method: http.MethodDelete, path: "/ollama/v1/models", allowMethod: http.MethodGet},
		{method: http.MethodGet, path: "/anthropic/v1/messages/count_tokens", allowMethod: http.MethodPost},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			called := false
			handler := MethodGuardMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{"model":"must-not-be-read"}`))
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if called {
				t.Fatal("downstream handler was called")
			}
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status: got %d, want %d", response.Code, http.StatusMethodNotAllowed)
			}
			if response.Header().Get("Allow") != test.allowMethod {
				t.Fatalf("Allow header: got %q, want %s", response.Header().Get("Allow"), test.allowMethod)
			}
			if !strings.Contains(response.Body.String(), "method_not_allowed") {
				t.Fatalf("unexpected body: %s", response.Body.String())
			}
		})
	}
}

func TestMethodGuardRejectsUnknownProviderSubpaths(t *testing.T) {
	for _, path := range []string{
		"/openai/",
		"/openai//v1/models",
		"/openai/v1/future-sensitive-route",
		"/ollama/v1/models/llama3.2",
		"/anthropic/v1/models/claude-sonnet-4",
		"/anthropic/api/event_logging/events",
	} {
		t.Run(path, func(t *testing.T) {
			called := false
			handler := MethodGuardMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if called {
				t.Fatal("downstream handler was called")
			}
			if response.Code != http.StatusNotFound {
				t.Fatalf("status: got %d, want %d", response.Code, http.StatusNotFound)
			}
			if !strings.Contains(response.Body.String(), "route_not_allowed") {
				t.Fatalf("unexpected body: %s", response.Body.String())
			}
		})
	}
}

func TestMethodGuardAllowsPostInterceptedAndNonInterceptedRoutes(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/openai/v1/chat/completions", body: `{"model":"gpt","messages":[{"role":"user","content":"hello"}]}`},
		{method: http.MethodPost, path: "/openai/v1/embeddings", body: `{"model":"embed","input":"hello"}`},
		{method: http.MethodPost, path: "/anthropic/v1/messages", body: `{"model":"claude","messages":[{"role":"user","content":"hello"}]}`},
		{method: http.MethodPost, path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello"}`},
		{method: http.MethodGet, path: "/openai/v1/models"},
		{method: http.MethodGet, path: "/ollama/v1/models"},
		{method: http.MethodPost, path: "/anthropic/v1/messages/count_tokens", body: `{"model":"claude","messages":[{"role":"user","content":"hello"}]}`},
		{method: http.MethodGet, path: "/unsupported"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			called := false
			handler := MethodGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if test.body != "" {
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("read restored body: %v", err)
					}
					if string(raw) != test.body {
						t.Fatalf("restored body: got %q, want %q", raw, test.body)
					}
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
			if !called || response.Code != http.StatusNoContent {
				t.Fatalf("downstream result: called=%v status=%d", called, response.Code)
			}
		})
	}
}

func TestMethodGuardRejectsProviderOwnedStateReferences(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "previous response", path: "/openai/v1/responses", body: `{"model":"gpt","input":"continue","previous_response_id":"resp_other"}`},
		{name: "conversation object", path: "/openai/v1/responses", body: `{"model":"gpt","input":"continue","conversation":{"id":"conv_other"}}`},
		{name: "stored prompt", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","prompt":{"id":"pmpt_other"}}`},
		{name: "item reference", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"item_reference","id":"item_other"}]}`},
		{name: "input file id", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"input_file","file_id":"file_other"}]}`},
		{name: "vector store", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","tools":[{"type":"file_search","vector_store_ids":["vs_other"]}]}`},
		{name: "container id", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","tools":[{"type":"code_interpreter","container":"cntr_other"}]}`},
		{name: "automatic container nested file id", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","tools":[{"type":"code_interpreter","container":{"type":"auto","file_ids":["file_other"]}}]}`},
		{name: "shell container reference", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","tools":[{"type":"shell","environment":{"type":"container_reference","container_id":"cntr_other"}}]}`},
		{name: "shell automatic container file", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","tools":[{"type":"shell","environment":{"type":"container_auto","file_ids":["file_other"]}}]}`},
		{name: "shell provider skill reference", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","tools":[{"type":"shell","environment":{"type":"container_auto","skills":[{"type":"skill_reference","skill_id":"skill_other"}]}}]}`},
		{name: "image generation tool", path: "/openai/v1/responses", body: `{"model":"gpt","input":"draw","tools":[{"type":"image_generation","model":"gpt-image-1","input_image_mask":{"file_id":"file_other"}}]}`},
		{name: "provider code interpreter output", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"code_interpreter_call","id":"ci_other","container_id":"cntr_other","code":"print(1)","outputs":[],"status":"completed"}]}`},
		{name: "provider MCP approval", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"mcp_approval_response","approval_request_id":"approval_other","approve":true}]}`},
		{name: "provider assistant output", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"message","id":"msg_other","role":"assistant","status":"completed","content":[{"type":"output_text","text":"secret"}]}]}`},
		{name: "MCP provider connector", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","tools":[{"type":"mcp","server_label":"drive","connector_id":"connector_dropbox"}]}`},
		{name: "remote response image", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"role":"user","content":[{"type":"input_image","image_url":"https://private.example/image"}]}]}`},
		{name: "encrypted response reasoning", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"reasoning","encrypted_content":"opaque-other-user-context","summary":[]}]}`},
		{name: "encrypted response compaction", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"compaction","encrypted_content":"opaque-other-user-context"}]}`},
		{name: "remote chat image", path: "/openai/v1/chat/completions", body: `{"model":"gpt","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://private.example/image"}}]}]}`},
		{name: "chat file id", path: "/openai/v1/chat/completions", body: `{"model":"gpt","messages":[{"role":"user","content":[{"type":"file","file":{"file_id":"file_other"}}]}]}`},
		{name: "anthropic file", path: "/anthropic/v1/messages", body: `{"model":"claude","messages":[{"role":"user","content":[{"type":"document","source":{"type":"file","file_id":"file_other"}}]}]}`},
		{name: "anthropic remote document", path: "/anthropic/v1/messages/count_tokens", body: `{"model":"claude","messages":[{"role":"user","content":[{"type":"document","source":{"type":"url","url":"https://private.example/document"}}]}]}`},
		{name: "anthropic nested document file", path: "/anthropic/v1/messages", body: `{"model":"claude","max_tokens":1,"messages":[{"role":"user","content":[{"type":"document","source":{"type":"content","content":[{"type":"image","source":{"type":"file","file_id":"file_other"}}]}}]}]}`},
		{name: "anthropic container upload", path: "/anthropic/v1/messages", body: `{"model":"claude","max_tokens":1,"messages":[{"role":"user","content":[{"type":"container_upload","file_id":"file_other"}]}]}`},
		{name: "anthropic reusable container", path: "/anthropic/v1/messages", body: `{"model":"claude","max_tokens":1,"container":"container_other","messages":[{"role":"user","content":"list files"}]}`},
		{name: "anthropic unknown beta block", path: "/anthropic/v1/messages", body: `{"model":"claude","max_tokens":1,"messages":[{"role":"user","content":[{"type":"future_provider_block","resource_id":"other"}]}]}`},
		{name: "anthropic thinking signature", path: "/anthropic/v1/messages", body: `{"model":"claude","messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"hidden","signature":"opaque-other-user-signature"}]}]}`},
		{name: "anthropic redacted thinking", path: "/anthropic/v1/messages", body: `{"model":"claude","messages":[{"role":"assistant","content":[{"type":"redacted_thinking","data":"opaque-other-user-context"}]}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := MethodGuardMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body)))

			if called {
				t.Fatal("downstream handler was called")
			}
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "provider_state_reference_not_allowed") {
				t.Fatalf("unexpected body: %s", response.Body.String())
			}
		})
	}
}

func TestMethodGuardAllowsInlineDataWithoutProviderReferences(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "chat data image", path: "/openai/v1/chat/completions", body: `{"model":"gpt","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}]}`},
		{name: "responses data image", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}]}`},
		{name: "anthropic base64 document", path: "/anthropic/v1/messages", body: `{"model":"claude","messages":[{"role":"user","content":[{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"AAAA"}}]}]}`},
		{name: "anthropic tool loop with arbitrary business JSON", path: "/anthropic/v1/messages", body: `{"model":"claude","metadata":{"user_id":"local-user"},"tools":[{"name":"lookup","description":"lookup","input_schema":{"type":"object","properties":{"customer_id":{"type":"string"},"url":{"type":"string"}}}}],"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"customer_id":"123","order_id":"456","project_id":"789","url":"https://api.example.test/customer","encrypted_content":"business-value","signature":"business-signature"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"inline result"}]}]}`},
		{name: "local tool correlation", path: "/openai/v1/chat/completions", body: `{"model":"gpt","messages":[{"role":"assistant","tool_calls":[{"id":"call_local","type":"function","function":{"name":"lookup","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_local","content":"inline result"}]}`},
		{name: "automatic container", path: "/openai/v1/responses", body: `{"model":"gpt","input":"calculate","tools":[{"type":"code_interpreter","container":"auto"}]}`},
		{name: "local shell", path: "/openai/v1/responses", body: `{"model":"gpt","input":"calculate","tools":[{"type":"shell","environment":{"type":"local","skills":[{"name":"lookup","description":"local","path":"/skills/lookup"}]}}]}`},
		{name: "responses local function loop", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"function_call","call_id":"call_local","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_local","output":"inline result"}]}`},
		{name: "arbitrary mixed-case business fields", path: "/anthropic/v1/messages", body: `{"model":"claude","tools":[{"name":"lookup","input_schema":{"type":"object","properties":{"Model":{"type":"string"},"Signature":{"type":"string"},"File_ID":{"type":"string"}}}}],"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"Model":"gold","Signature":"signed","File_ID":"customer-file"}}]}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := MethodGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read restored body: %v", err)
				}
				if string(raw) != test.body {
					t.Fatalf("restored body: got %q, want %q", raw, test.body)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body)))

			if !called || response.Code != http.StatusNoContent {
				t.Fatalf("downstream result: called=%v status=%d body=%s", called, response.Code, response.Body.String())
			}
		})
	}
}

func TestMethodGuardRejectsInvalidAndOversizedPOSTBodies(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		maxBytes   int64
		statusCode int
		errorCode  string
	}{
		{name: "empty", path: "/openai/v1/responses", body: "", maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "malformed", path: "/openai/v1/responses", body: `{"model":`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "array", path: "/openai/v1/responses", body: `[{"model":"gpt"}]`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "duplicate key", path: "/openai/v1/responses", body: `{"model":"gpt","model":"other"}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "case folded duplicate model responses", path: "/openai/v1/responses", body: `{"model":"forbidden","MODEL":"allowed"}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "case folded duplicate model embeddings", path: "/openai/v1/embeddings", body: `{"model":"forbidden","MODEL":"allowed","input":"hello"}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "case folded duplicate model count tokens", path: "/anthropic/v1/messages/count_tokens", body: `{"model":"forbidden","MODEL":"allowed","messages":[]}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "case variant max output", path: "/openai/v1/responses", body: `{"model":"gpt","max_output_tokens":100000,"MAX_OUTPUT_TOKENS":1}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "unicode simple-fold max tokens", path: "/openai/v1/chat/completions", body: `{"model":"gpt","max_tokens":100000,"max_tokenſ":1}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "unicode simple-fold provider IDs", path: "/openai/v1/responses", body: `{"model":"gpt","input":"hello","vector_store_ids":["vs_other"],"vector_store_idſ":[]}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "nested case variant URL", path: "/openai/v1/responses", body: `{"model":"gpt","input":[{"type":"input_image","IMAGE_URL":"https://private.example/image"}]}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "provider_state_reference_not_allowed"},
		{name: "trailing value", path: "/openai/v1/responses", body: `{"model":"gpt"}{"model":"other"}`, maxBytes: 1024, statusCode: http.StatusBadRequest, errorCode: "invalid_request_body"},
		{name: "oversized", path: "/openai/v1/responses", body: `{"model":"gpt"}`, maxBytes: 5, statusCode: http.StatusRequestEntityTooLarge, errorCode: "request_body_too_large"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := MethodGuardMiddlewareWithOptions(MethodGuardOptions{MaxBufferedRequestBytes: test.maxBytes})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body)))

			if called {
				t.Fatal("downstream handler was called")
			}
			if response.Code != test.statusCode || !strings.Contains(response.Body.String(), test.errorCode) {
				t.Fatalf("response: got %d %s, want %d containing %q", response.Code, response.Body.String(), test.statusCode, test.errorCode)
			}
		})
	}
}

func TestDecodeStrictJSONHandlesManyUniqueKeys(t *testing.T) {
	const keyCount = 10_000
	var body strings.Builder
	body.WriteByte('{')
	for index := range keyCount {
		if index > 0 {
			body.WriteByte(',')
		}
		fmt.Fprintf(&body, "%q:%d", fmt.Sprintf("field_%d", index), index)
	}
	body.WriteByte('}')

	decoded, err := decodeStrictJSON([]byte(body.String()))
	if err != nil {
		t.Fatalf("decode large object: %v", err)
	}
	object, ok := decoded.(map[string]any)
	if !ok || len(object) != keyCount {
		t.Fatalf("decoded %T with %d keys, want object with %d keys", decoded, len(object), keyCount)
	}
}
