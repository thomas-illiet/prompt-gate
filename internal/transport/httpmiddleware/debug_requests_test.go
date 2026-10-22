package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebugRequestsLogsJSONBodyAndOriginalRequestMetadata(t *testing.T) {
	var output bytes.Buffer
	logger := NewJSONLineWriter(&output)
	handler := logger.DebugRequests(1024)(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") == "" {
			t.Fatal("middleware changed headers before the handler")
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read mirrored body: %v", err)
		}
		if string(body) != `{"prompt":"secret"}` {
			t.Fatalf("unexpected body delivered to handler: %q", body)
		}
		request.Header.Del("Authorization")
		request.Header.Del("X-Multi")
		response.Header().Set("X-Response-Secret", "must-not-be-logged")
	}))

	request := httptest.NewRequest(http.MethodPost, "https://provider.example/v1/chat?raw=a%2Bb", strings.NewReader(`{"prompt":"secret"}`))
	request.RemoteAddr = "192.0.2.10:4321"
	request.Header.Add("X-Multi", "first")
	request.Header.Add("X-Multi", "second")
	request.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	event := decodeDebugEvent(t, output.Bytes())
	if event["level"] != "DEBUG" || event["event"] != debugRequestEvent {
		t.Fatalf("unexpected event identity: %#v", event)
	}
	if event["scheme"] != "https" || event["host"] != "provider.example" || event["raw_query"] != "raw=a%2Bb" {
		t.Fatalf("unexpected request target: %#v", event)
	}
	headers := event["headers"].(map[string]any)
	if headers["Authorization"].([]any)[0] != "Bearer secret" {
		t.Fatalf("original authorization header was not retained: %#v", headers)
	}
	values := headers["X-Multi"].([]any)
	if len(values) != 2 || values[0] != "first" || values[1] != "second" {
		t.Fatalf("multiple header values were not retained: %#v", values)
	}
	body := event["body"].(map[string]any)
	if body["prompt"] != "secret" {
		t.Fatalf("JSON body was not structured: %#v", body)
	}
	if _, present := event["body_truncated"]; present {
		t.Fatal("body_truncated must be omitted for requests within the limit")
	}
	if bytes.Contains(output.Bytes(), []byte("must-not-be-logged")) {
		t.Fatal("response data was logged")
	}
}

func TestDebugRequestsDoesNotDrainUnreadBody(t *testing.T) {
	var output bytes.Buffer
	body := &countingReadCloser{Reader: strings.NewReader("not-json-and-long")}
	handler := NewJSONLineWriter(&output).DebugRequests(5)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusUnauthorized)
	}))

	request := httptest.NewRequest(http.MethodPost, "http://proxy.example/rejected", nil)
	request.Body = body
	handler.ServeHTTP(httptest.NewRecorder(), request)

	event := decodeDebugEvent(t, output.Bytes())
	if body.read != 0 {
		t.Fatalf("middleware drained %d unread body bytes", body.read)
	}
	if event["body"] != "" {
		t.Fatalf("expected only handler-consumed bytes to be logged: %#v", event)
	}
	if _, present := event["body_truncated"]; present {
		t.Fatal("unread bytes must not be reported as captured truncation")
	}
}

type countingReadCloser struct {
	io.Reader
	read int
}

func (body *countingReadCloser) Read(buffer []byte) (int, error) {
	count, err := body.Reader.Read(buffer)
	body.read += count
	return count, err
}

func (*countingReadCloser) Close() error { return nil }

func TestDebugRequestsKeepsInvalidJSONAsRawString(t *testing.T) {
	var output bytes.Buffer
	handler := NewJSONLineWriter(&output).DebugRequests(1024)(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		buffer := make([]byte, 2)
		for {
			_, err := request.Body.Read(buffer)
			if err == io.EOF {
				return
			}
			if err != nil {
				t.Fatalf("streaming read failed: %v", err)
			}
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader("{invalid")))

	event := decodeDebugEvent(t, output.Bytes())
	if event["body"] != "{invalid" {
		t.Fatalf("invalid JSON was not retained verbatim: %#v", event["body"])
	}
}

func TestDebugRequestsLogsJSONArrayAsStructuredData(t *testing.T) {
	var output bytes.Buffer
	handler := NewJSONLineWriter(&output).DebugRequests(1024)(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/array", strings.NewReader(`[1,"two"]`)))

	event := decodeDebugEvent(t, output.Bytes())
	body := event["body"].([]any)
	if len(body) != 2 || body[0] != float64(1) || body[1] != "two" {
		t.Fatalf("JSON array was not structured: %#v", body)
	}
}

func decodeDebugEvent(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("decode JSON line: %v\n%s", err, data)
	}
	return event
}
