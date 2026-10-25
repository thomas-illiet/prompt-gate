package cli

import (
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
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
