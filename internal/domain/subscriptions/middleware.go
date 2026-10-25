package subscriptions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/proxylimits"
)

type MiddlewareOptions struct {
	MaxBufferedRequestBytes  int64
	MaxBufferedResponseBytes int64
	ReservationTTL           time.Duration
}

type quotaReservationContextKey struct{}

type quotaRequestState struct {
	reservation QuotaReservation

	mu                    sync.Mutex
	recorderTokens        int64
	responseTokens        int64
	responseUsageObserved bool
	streaming             bool
	streamCompleted       bool
}

func (s *quotaRequestState) addRecorderTokens(tokens int64) {
	if s == nil || tokens <= 0 {
		return
	}
	s.mu.Lock()
	s.recorderTokens = saturatingAdd(s.recorderTokens, tokens)
	s.mu.Unlock()
}

func (s *quotaRequestState) setResponseTokens(tokens int64) {
	if s == nil || tokens <= 0 {
		return
	}
	s.mu.Lock()
	s.responseTokens = tokens
	s.responseUsageObserved = true
	s.mu.Unlock()
}

func (s *quotaRequestState) observedTokens() (int64, bool) {
	if s == nil {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.streaming && !s.streamCompleted {
		return 0, false
	}
	if s.responseUsageObserved {
		return s.responseTokens, true
	}
	return s.recorderTokens, s.recorderTokens > 0
}

func (s *quotaRequestState) markStreaming() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.streaming = true
	s.mu.Unlock()
}

func (s *quotaRequestState) markStreamCompleted() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.streaming = true
	s.streamCompleted = true
	s.mu.Unlock()
}

// Middleware blocks proxy traffic when the authenticated identity has no effective
// subscription plan or has exhausted one of its quota windows.
func Middleware(store *RedisStore, logger *slog.Logger) func(http.Handler) http.Handler {
	return MiddlewareWithOptions(store, logger, MiddlewareOptions{})
}

// MiddlewareWithOptions atomically reserves finite quota before dispatching a
// billable request. The recorder later replaces the hold with observed usage;
// explicit upstream failures release it when the handler returns. A cancelled
// client is charged conservatively because the provider may already have
// accepted and billed the request before Prompt Gate observed a response.
func MiddlewareWithOptions(store *RedisStore, logger *slog.Logger, options MiddlewareOptions) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	maxRequestBytes := options.MaxBufferedRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = proxylimits.DefaultMaxBufferedRequestBytes
	}
	maxResponseBytes := options.MaxBufferedResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = proxylimits.DefaultMaxBufferedResponseBytes
	}
	reservationTTL := options.ReservationTTL
	if reservationTTL <= 0 {
		reservationTTL = quotaReservationTTL
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isQuotaConsumingRoute(r) {
				next.ServeHTTP(w, r)
				return
			}

			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing_authenticated_user"})
				return
			}
			if store == nil || !store.Enabled() {
				logger.Error("subscription quota store unavailable", "user_id", user.ID)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "subscription_quota_unavailable"})
				return
			}

			if err := store.RefreshIfStale(r.Context()); err != nil {
				logger.Error("subscription quota refresh failed", "user_id", user.ID, "error", err)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "subscription_quota_unavailable"})
				return
			}
			plan, hasSubscription, err := store.ResolveEffectivePlan(r.Context(), user.ID)
			if err != nil {
				logger.Error("subscription quota check failed", "user_id", user.ID, "error", err)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "subscription_quota_unavailable"})
				return
			}
			if !hasSubscription {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "subscription_required"})
				return
			}
			if plan.Quota5HTokens == nil && plan.Quota7DTokens == nil {
				next.ServeHTTP(w, r)
				return
			}

			requested, err := estimateQuotaReservation(r, maxRequestBytes)
			if err != nil {
				if errors.Is(err, proxylimits.ErrExceeded) {
					writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request_body_too_large"})
					return
				}
				logger.Warn("failed to read request for quota reservation", "user_id", user.ID, "error", err)
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request_body"})
				return
			}
			decision, err := store.reserveQuota(r.Context(), user.ID, plan, requested, reservationTTL)
			if err != nil {
				logger.Error("subscription quota reservation failed", "user_id", user.ID, "error", err)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "subscription_quota_unavailable"})
				return
			}
			if decision.Exceeded != nil {
				writeQuotaExceeded(w, *decision.Exceeded)
				return
			}

			reservation := decision.Reservation
			state := &quotaRequestState{reservation: reservation}
			ctx := context.WithValue(r.Context(), quotaReservationContextKey{}, state)
			responseWriter := newQuotaResponseWriter(w, state, maxResponseBytes)
			defer func() {
				releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
				defer cancel()
				if responseWriter.successful() || ctx.Err() != nil {
					tokens := reservation.Tokens
					if observed, ok := state.observedTokens(); ok {
						tokens = observed
					}
					committed, err := store.CommitQuotaReservation(releaseCtx, reservation, tokens)
					if err != nil {
						logger.Error("failed to finalize subscription quota reservation", "user_id", user.ID, "reservation_id", reservation.ID, "error", err)
					} else if !committed {
						// A long-running stream can outlive its reservation TTL. The hold is
						// gone in that case, but its successful usage must still be charged.
						if err := store.IncrementUsage(releaseCtx, user.ID, tokens, time.Now().UTC()); err != nil {
							logger.Error("failed to record expired subscription quota reservation", "user_id", user.ID, "reservation_id", reservation.ID, "error", err)
						}
					}
					return
				}
				if err := store.ReleaseQuotaReservation(releaseCtx, reservation); err != nil {
					logger.Error("failed to release subscription quota reservation", "user_id", user.ID, "reservation_id", reservation.ID, "error", err)
				}
			}()
			next.ServeHTTP(responseWriter, r.WithContext(ctx))
		})
	}
}

type quotaResponseWriter struct {
	http.ResponseWriter
	statusCode int
	observer   *responsesSSEUsageObserver
	state      *quotaRequestState
}

func newQuotaResponseWriter(w http.ResponseWriter, state *quotaRequestState, maxResponseBytes int64) *quotaResponseWriter {
	writer := &quotaResponseWriter{ResponseWriter: w, state: state}
	writer.observer = newResponsesSSEUsageObserver(state, maxResponseBytes)
	return writer
}

func (w *quotaResponseWriter) WriteHeader(statusCode int) {
	if statusCode >= 100 && statusCode < 200 && statusCode != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(statusCode)
		return
	}
	if w.statusCode != 0 {
		return
	}
	w.statusCode = statusCode
	w.observeStreamingHeader()
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *quotaResponseWriter) Write(raw []byte) (int, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.observeStreamingHeader() && w.observer != nil {
		w.observer.Write(raw)
	}
	return w.ResponseWriter.Write(raw)
}

func (w *quotaResponseWriter) successful() bool {
	return w.statusCode >= http.StatusOK && w.statusCode < http.StatusMultipleChoices
}

func (w *quotaResponseWriter) Flush() {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	w.observeStreamingHeader()
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *quotaResponseWriter) observeStreamingHeader() bool {
	streaming := strings.Contains(strings.ToLower(w.Header().Get("Content-Type")), "text/event-stream")
	if streaming {
		w.state.markStreaming()
	}
	return streaming
}

func (w *quotaResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *quotaResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return io.Copy(w.ResponseWriter, reader)
}

func (w *quotaResponseWriter) Push(target string, options *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

func (w *quotaResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func writeQuotaExceeded(w http.ResponseWriter, exceeded quotaExceededResponse) {
	if exceeded.ResetAt != nil {
		retryAfter := time.Until(*exceeded.ResetAt)
		if retryAfter > 0 {
			w.Header().Set("Retry-After", strconv.FormatInt(int64(retryAfter.Seconds()), 10))
		}
	}
	writeJSON(w, http.StatusTooManyRequests, exceeded)
}

func quotaRequestStateFromContext(ctx context.Context) (*quotaRequestState, bool) {
	state, ok := ctx.Value(quotaReservationContextKey{}).(*quotaRequestState)
	return state, ok && state != nil && state.reservation.ID != "" && state.reservation.UserID != ""
}

// estimateQuotaReservation conservatively bounds input/cache tokens by three
// times the UTF-8 request size and adds the caller's maximum output bound. A
// negative result asks Redis to reserve all remaining quota, serializing calls
// whose output is unbounded instead of admitting them concurrently.
func estimateQuotaReservation(r *http.Request, maxRequestBytes int64) (int64, error) {
	if r.Body == nil {
		return -1, nil
	}
	raw, err := proxylimits.ReadAll(r.Body, maxRequestBytes)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}

	inputEstimate := saturatingMultiply(int64(len(raw)), 3)
	if inputEstimate < 1 {
		inputEstimate = 1
	}
	if requestProviderPath(r.URL.Path) == "/v1/embeddings" {
		return inputEstimate, nil
	}

	var bounds struct {
		MaxTokens           *int64           `json:"max_tokens"`
		MaxCompletionTokens *int64           `json:"max_completion_tokens"`
		MaxOutputTokens     *int64           `json:"max_output_tokens"`
		N                   *int64           `json:"n"`
		WebSearchOptions    *json.RawMessage `json:"web_search_options"`
		MCPServers          json.RawMessage  `json:"mcp_servers"`
		Tools               []struct {
			Type string `json:"type"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &bounds); err != nil {
		return -1, nil
	}
	providerPath := requestProviderPath(r.URL.Path)
	if requestHasUnboundedProviderToolUsage(providerPath, bounds.WebSearchOptions, bounds.MCPServers, bounds.Tools) {
		// Hosted tools can add search results, container output, connector
		// content, or other provider-side context that is absent from the HTTP
		// request body. max_output_tokens therefore does not bound total billed
		// tokens; reserve all remaining quota and serialize these calls.
		return -1, nil
	}
	maxOutput := maxPositive(bounds.MaxTokens, bounds.MaxCompletionTokens, bounds.MaxOutputTokens)
	if maxOutput <= 0 {
		return -1, nil
	}
	choices := int64(1)
	if bounds.N != nil && *bounds.N > 1 {
		choices = *bounds.N
	}
	return saturatingAdd(inputEstimate, saturatingMultiply(maxOutput, choices)), nil
}

func requestHasUnboundedProviderToolUsage(providerPath string, webSearchOptions *json.RawMessage, mcpServers json.RawMessage, tools []struct {
	Type string `json:"type"`
}) bool {
	switch providerPath {
	case "/v1/chat/completions":
		return webSearchOptions != nil
	case "/v1/messages":
		if len(mcpServers) > 0 && string(bytes.TrimSpace(mcpServers)) != "null" && string(bytes.TrimSpace(mcpServers)) != "[]" {
			return true
		}
		for _, tool := range tools {
			switch strings.ToLower(strings.TrimSpace(tool.Type)) {
			case "", "custom":
				// Anthropic client tools omit type (or explicitly say custom).
			default:
				return true
			}
		}
		return false
	case "/v1/responses":
		// Continue below.
	default:
		return false
	}
	for _, tool := range tools {
		switch strings.ToLower(strings.TrimSpace(tool.Type)) {
		case "", "function", "custom", "local_shell", "apply_patch":
			// Caller-executed tools cannot inject provider-side context.
		default:
			return true
		}
	}
	return false
}

func maxPositive(values ...*int64) int64 {
	var maximum int64
	for _, value := range values {
		if value != nil && *value > maximum {
			maximum = *value
		}
	}
	return maximum
}

func saturatingMultiply(left, right int64) int64 {
	if left <= 0 || right <= 0 {
		return 0
	}
	if left > math.MaxInt64/right {
		return math.MaxInt64
	}
	return left * right
}

func saturatingAdd(left, right int64) int64 {
	if right > math.MaxInt64-left {
		return math.MaxInt64
	}
	return left + right
}

type quotaExceededResponse struct {
	Error       string     `json:"error"`
	Window      string     `json:"window"`
	UsedTokens  int64      `json:"usedTokens"`
	QuotaTokens int64      `json:"quotaTokens"`
	ResetAt     *time.Time `json:"resetAt"`
}

func isQuotaConsumingRoute(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	providerPath := requestProviderPath(r.URL.Path)
	switch {
	case providerPath == "/v1/chat/completions":
		return true
	case providerPath == "/v1/embeddings":
		return true
	case providerPath == "/v1/messages":
		return true
	case providerPath == "/v1/responses":
		return true
	default:
		return false
	}
}

func requestProviderPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	_, suffix, ok := strings.Cut(path, "/")
	if !ok {
		return "/"
	}
	return "/" + strings.TrimPrefix(suffix, "/")
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
