package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	cdrslog "cdr.dev/slog/v3"
)

// TestProxyHTTPServerTimesOutSlowRequestBody verifies ReadTimeout is a real
// socket read deadline, not merely a handler context deadline.
func TestProxyHTTPServerTimesOutSlowRequestBody(t *testing.T) {
	readErr := make(chan error, 1)
	server, address := startProxyTestServer(t, 100*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		readErr <- err
		if err != nil {
			http.Error(w, "request body timeout", http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("connect to proxy test server: %v", err)
	}
	defer connection.Close()
	if _, err := io.WriteString(connection, "POST / HTTP/1.1\r\nHost: proxy.test\r\nContent-Length: 10\r\n\r\nx"); err != nil {
		t.Fatalf("write partial request: %v", err)
	}

	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("slow partial body unexpectedly completed")
		}
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("body read error is not a timeout: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow request body was not terminated by the server read deadline")
	}
}

// TestProxyHTTPServerReadTimeoutDoesNotBoundLongResponse verifies the inbound
// read deadline does not become a write deadline for long-lived SSE responses.
func TestProxyHTTPServerReadTimeoutDoesNotBoundLongResponse(t *testing.T) {
	const readTimeout = 100 * time.Millisecond
	server, address := startProxyTestServer(t, readTimeout, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(2 * readTimeout)
		_, _ = io.WriteString(w, "data: complete\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	response, err := http.Get("http://" + address) //nolint:gosec // loopback test server
	if err != nil {
		t.Fatalf("request long response: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read long response: %v", err)
	}
	if string(body) != "data: complete\n\n" {
		t.Fatalf("unexpected SSE body: %q", body)
	}
	if server.WriteTimeout != 0 {
		t.Fatalf("write timeout must remain disabled for SSE, got %s", server.WriteTimeout)
	}
}

// TestProxyHTTPServerTimesOutSlowResponseReader verifies a client that stops
// reading cannot pin a provider-stream goroutine indefinitely once the socket
// send buffer fills.
func TestProxyHTTPServerTimesOutSlowResponseReader(t *testing.T) {
	const idleWriteTimeout = 100 * time.Millisecond
	writeErr := make(chan error, 1)
	server, address := startProxyTestServer(t, idleWriteTimeout, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		chunk := make([]byte, 1<<20)
		for range 128 {
			if _, err := w.Write(chunk); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- nil
	}))
	defer server.Close()

	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("connect to proxy test server: %v", err)
	}
	defer connection.Close()
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(1024)
	}
	if _, err := io.WriteString(connection, "GET / HTTP/1.1\r\nHost: proxy.test\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case err := <-writeErr:
		if err == nil {
			t.Fatal("slow reader accepted the entire response without hitting a write deadline")
		}
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("response write error is not a timeout: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("slow response reader did not trigger the sliding write deadline")
	}
}

func startProxyTestServer(t *testing.T, readTimeout time.Duration, handler http.Handler) (*http.Server, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := newProxyHTTPServer(listener.Addr().String(), handler, readTimeout)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve proxy test server: %v", err)
		}
	}()
	return server, listener.Addr().String()
}

func TestWriteDeadlineResponseWriterOptionalInterfaces(t *testing.T) {
	base := &proxyControllerWriter{header: make(http.Header)}
	writer := &writeDeadlineResponseWriter{ResponseWriter: base, timeout: time.Second}

	written, err := writer.ReadFrom(&proxyTestReader{data: []byte("streamed")})
	if err != nil {
		t.Fatalf("read from: %v", err)
	}
	if written != int64(len("streamed")) || base.body.String() != "streamed" {
		t.Fatalf("unexpected copied body: bytes=%d body=%q", written, base.body.String())
	}
	if base.deadline.IsZero() || time.Until(base.deadline) <= 0 {
		t.Fatalf("expected a future write deadline, got %v", base.deadline)
	}

	writer.Flush()
	if base.flushes != 1 {
		t.Fatalf("expected one flush, got %d", base.flushes)
	}
	if err := writer.Push("/asset.js", &http.PushOptions{Method: http.MethodGet}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if base.pushed != "/asset.js" {
		t.Fatalf("unexpected pushed target: %q", base.pushed)
	}
	if writer.Unwrap() != base {
		t.Fatal("unwrap did not return the underlying response writer")
	}

	connection, _, err := writer.Hijack()
	if err != nil {
		t.Fatalf("hijack: %v", err)
	}
	_ = connection.Close()
	_ = base.hijackPeer.Close()
}

func TestWriteDeadlineResponseWriterPropagatesDeadlineErrors(t *testing.T) {
	errDeadline := errors.New("deadline unavailable")
	base := &proxyControllerWriter{header: make(http.Header), deadlineErr: errDeadline}
	writer := &writeDeadlineResponseWriter{ResponseWriter: base, timeout: time.Second}

	if written, err := writer.Write([]byte("body")); written != 0 || !errors.Is(err, errDeadline) {
		t.Fatalf("write = (%d, %v), want (0, %v)", written, err, errDeadline)
	}
	if written, err := writer.ReadFrom(&proxyTestReader{data: []byte("body")}); written != 0 || !errors.Is(err, errDeadline) {
		t.Fatalf("read from = (%d, %v), want (0, %v)", written, err, errDeadline)
	}
	writer.Flush()
	if base.flushes != 0 {
		t.Fatalf("flush reached the underlying writer after a deadline error: %d", base.flushes)
	}

	withoutPusher := &writeDeadlineResponseWriter{ResponseWriter: &statusOnlyWriter{header: make(http.Header)}}
	if err := withoutPusher.Push("/asset.js", nil); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unsupported push error = %v", err)
	}
}

func TestCDRLevel(t *testing.T) {
	tests := map[string]cdrslog.Level{
		"debug":      cdrslog.LevelDebug,
		" WARN ":     cdrslog.LevelWarn,
		"warning":    cdrslog.LevelWarn,
		"ERROR":      cdrslog.LevelError,
		"info":       cdrslog.LevelInfo,
		"unexpected": cdrslog.LevelInfo,
	}
	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if got := cdrLevel(input); got != expected {
				t.Fatalf("cdrLevel(%q) = %v, want %v", input, got, expected)
			}
		})
	}
}

type proxyControllerWriter struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	deadline    time.Time
	deadlineErr error
	flushes     int
	pushed      string
	hijackPeer  net.Conn
}

func (w *proxyControllerWriter) Header() http.Header { return w.header }

func (w *proxyControllerWriter) Write(raw []byte) (int, error) { return w.body.Write(raw) }

func (w *proxyControllerWriter) WriteHeader(statusCode int) { w.status = statusCode }

func (w *proxyControllerWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return w.deadlineErr
}

func (w *proxyControllerWriter) Flush() { w.flushes++ }

func (w *proxyControllerWriter) Push(target string, _ *http.PushOptions) error {
	w.pushed = target
	return nil
}

func (w *proxyControllerWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	connection, peer := net.Pipe()
	w.hijackPeer = peer
	return connection, bufio.NewReadWriter(bufio.NewReader(connection), bufio.NewWriter(connection)), nil
}

type statusOnlyWriter struct {
	header http.Header
}

func (w *statusOnlyWriter) Header() http.Header           { return w.header }
func (w *statusOnlyWriter) Write(raw []byte) (int, error) { return len(raw), nil }
func (w *statusOnlyWriter) WriteHeader(int)               {}

type proxyTestReader struct {
	data []byte
}

func (r *proxyTestReader) Read(raw []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	written := copy(raw, r.data)
	r.data = r.data[written:]
	return written, nil
}
