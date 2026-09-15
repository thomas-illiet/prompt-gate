package runtime

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type responseTimingContextKey struct{}

type responseTiming struct {
	started time.Time
	once    sync.Once
	mu      sync.RWMutex
	span    trace.Span
}

// ResponseTimingMiddleware measures client-visible time to first byte without buffering responses.
func ResponseTimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timing := &responseTiming{started: time.Now()}
		wrapped := &timingResponseWriter{ResponseWriter: w, timing: timing}
		next.ServeHTTP(wrapped, r.WithContext(context.WithValue(r.Context(), responseTimingContextKey{}, timing)))
	})
}

// BindResponseTiming attaches the interception span to the response observer.
func BindResponseTiming(ctx context.Context, span trace.Span) {
	timing, _ := ctx.Value(responseTimingContextKey{}).(*responseTiming)
	if timing == nil {
		return
	}
	timing.mu.Lock()
	timing.span = span
	timing.mu.Unlock()
}

func (t *responseTiming) firstByte(statusCode int) {
	t.once.Do(func() {
		t.mu.RLock()
		span := t.span
		t.mu.RUnlock()
		if span != nil {
			span.SetAttributes(
				attribute.Int64("promptgate.latency.time_to_first_byte_ms", time.Since(t.started).Milliseconds()),
				attribute.Int("http.response.status_code", statusCode),
			)
		}
	})
}

type timingResponseWriter struct {
	http.ResponseWriter
	timing *responseTiming
}

func (w *timingResponseWriter) WriteHeader(statusCode int) {
	w.timing.firstByte(statusCode)
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *timingResponseWriter) Write(body []byte) (int, error) {
	w.timing.firstByte(http.StatusOK)
	return w.ResponseWriter.Write(body)
}

func (w *timingResponseWriter) Flush() {
	w.timing.firstByte(http.StatusOK)
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *timingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *timingResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
