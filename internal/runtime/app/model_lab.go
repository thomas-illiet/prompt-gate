package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/proxylimits"
	"promptgate/backend/internal/platform/requestprivacy"
)

const (
	maxModelLabSSEFrameBytes      = 1 << 20
	minModelLabStreamBytes        = 1 << 20
	defaultMaxModelLabStreamBytes = 2 << 20
	hardMaxModelLabStreamBytes    = 32 << 20
	modelLabBytesPerOutputToken   = 512
)

var (
	errModelLabSSEFrameTooLarge  = errors.New("model lab SSE frame exceeds limit")
	errModelLabSSEStreamTooLarge = errors.New("model lab SSE stream exceeds limit")
)

func modelLabTicketMiddleware(
	cfg config.ModelLabConfig,
	service *modellab.Service,
	userService *users.Service,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, matched := modelLabBearerToken(r.Header.Get("Authorization"))
			if !matched {
				next.ServeHTTP(w, r)
				return
			}

			setModelLabProxyNoStore(w)
			ctx := requestprivacy.MarkMetadataOnly(r.Context())
			r = r.WithContext(ctx)
			if service == nil || userService == nil || r.URL.RawQuery != "" {
				writeModelLabProxyError(w, http.StatusUnauthorized, "invalid_playground_ticket")
				return
			}

			claims, err := service.ParseTicket(rawToken)
			if err != nil || r.Method != claims.Method || r.URL.Path != claims.Path {
				writeModelLabProxyError(w, http.StatusUnauthorized, "invalid_playground_ticket")
				return
			}

			rawBody, err := proxylimits.ReadAll(r.Body, cfg.MaxPromptBytes)
			if err != nil {
				status := http.StatusBadRequest
				code := "invalid_model_lab_payload"
				if errors.Is(err, proxylimits.ErrExceeded) {
					status, code = http.StatusRequestEntityTooLarge, "model_lab_prompt_too_large"
				}
				writeModelLabProxyError(w, status, code)
				return
			}
			normalized, err := service.ValidateAndNormalizePayload(
				claims.Protocol,
				claims.Model,
				rawBody,
				int(cfg.MaxPromptBytes),
			)
			if err != nil {
				writeModelLabProxyError(w, http.StatusBadRequest, "invalid_model_lab_payload")
				return
			}

			claims, err = service.ConsumeTicket(ctx, modellab.ConsumeTicketInput{
				Token: rawToken, Method: r.Method, Path: r.URL.Path, Origin: r.Header.Get("Origin"),
			})
			if err != nil {
				status, code := modelLabTicketError(err)
				writeModelLabProxyError(w, status, code)
				return
			}

			user, err := userService.UserByID(ctx, claims.Subject)
			if err != nil || !user.IsActive || string(user.Role) != claims.Role ||
				(user.Role != auth.RoleUser && user.Role != auth.RoleAdmin) {
				writeModelLabProxyError(w, http.StatusForbidden, "model_lab_access_denied")
				return
			}
			if err := service.AcquireLease(ctx, claims); err != nil {
				status, code := modelLabTicketError(err)
				writeModelLabProxyError(w, status, code)
				return
			}
			defer func() {
				releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
				defer cancel()
				_ = service.ReleaseLease(releaseCtx, claims)
			}()

			prepareModelLabProxyRequest(r, normalized, claims.Protocol)
			principal := auth.Principal{User: user, CredentialID: "model-lab", CredentialName: "Model Lab"}
			r = r.WithContext(auth.ContextWithPrincipal(r.Context(), principal))

			response := &modelLabResponseWriter{
				ResponseWriter: w,
				maxStreamBytes: modelLabStreamByteLimit(cfg.MaxOutputTokens),
			}
			next.ServeHTTP(response, r)
			response.finish()
		})
	}
}

func modelLabBearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, strings.HasPrefix(token, modellab.TicketPrefix)
}

func prepareModelLabProxyRequest(r *http.Request, body []byte, protocol modellab.Protocol) {
	preserved := make(http.Header)
	// Preserve only the headers needed by Prompt Gate's trusted client-IP
	// resolver. A later boundary removes them before the provider request.
	for _, name := range []string{"X-Forwarded-For", "X-Real-IP"} {
		for _, value := range r.Header.Values(name) {
			preserved.Add(name, value)
		}
	}
	preserved.Set("Accept", "text/event-stream")
	preserved.Set("Content-Type", "application/json")
	preserved.Set("User-Agent", "PromptGate-ModelLab/1.0")
	if protocol == modellab.ProtocolAnthropicMessages {
		preserved.Set("Anthropic-Version", "2023-06-01")
	}
	r.Header = preserved
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Length", strconv.FormatInt(r.ContentLength, 10))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
}

// stripModelLabClientHeaders keeps forwarding metadata available to client-IP
// and OPA checks, then removes it before groups, AIBridge, and the provider.
func stripModelLabClientHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestprivacy.IsMetadataOnly(r.Context()) {
			for _, name := range []string{
				"Forwarded",
				"X-Forwarded-For",
				"X-Forwarded-Host",
				"X-Forwarded-Proto",
				"X-Real-IP",
				"Traceparent",
				"Tracestate",
				"Baggage",
			} {
				r.Header.Del(name)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func modelLabTicketError(err error) (int, string) {
	switch {
	case errors.Is(err, modellab.ErrParallelLimit):
		return http.StatusTooManyRequests, "model_lab_parallel_limit"
	case errors.Is(err, modellab.ErrTicketUnavailable):
		return http.StatusServiceUnavailable, "model_lab_ticket_unavailable"
	default:
		return http.StatusUnauthorized, "invalid_playground_ticket"
	}
}

func setModelLabProxyNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
}

func writeModelLabProxyError(w http.ResponseWriter, status int, code string) {
	setModelLabProxyNoStore(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// modelLabResponseWriter streams successful responses untouched and replaces
// every downstream error body with a stable Prompt Gate code. Provider error
// bodies can contain request content and must never reach the browser.
type modelLabResponseWriter struct {
	http.ResponseWriter
	status             int
	wroteHeader        bool
	wroteError         bool
	streamBuffer       []byte
	streamBytes        int64
	maxStreamBytes     int64
	streamWriteFailure error
}

func (w *modelLabResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
}

func (w *modelLabResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.status < http.StatusOK || w.status >= http.StatusMultipleChoices {
		w.writeSanitizedError()
		return len(body), nil
	}
	if !w.commitSuccessHeader() {
		return len(body), nil
	}
	if w.status == http.StatusNoContent {
		return len(body), nil
	}
	if err := w.writeSanitizedStream(body, false); err != nil {
		return 0, err
	}
	return len(body), nil
}

func (w *modelLabResponseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.status < http.StatusOK || w.status >= http.StatusMultipleChoices {
		w.writeSanitizedError()
	} else {
		w.commitSuccessHeader()
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *modelLabResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *modelLabResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{w}, reader)
}

func (w *modelLabResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *modelLabResponseWriter) finish() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.status < http.StatusOK || w.status >= http.StatusMultipleChoices {
		w.writeSanitizedError()
		return
	}
	if !w.commitSuccessHeader() {
		return
	}
	if len(w.streamBuffer) > 0 && w.streamWriteFailure == nil {
		w.streamWriteFailure = w.writeSanitizedStream(nil, true)
	}
}

// commitSuccessHeader permits only the streaming response contract. A provider
// returning JSON, HTML, or a redirect despite stream=true may include raw error
// details, so it is replaced before any status, header, or body reaches the
// browser. A bodyless 204 remains useful for middleware tests and no-op paths.
func (w *modelLabResponseWriter) commitSuccessHeader() bool {
	if w.wroteHeader {
		return true
	}
	if w.status != http.StatusNoContent && (w.status != http.StatusOK || !modelLabIsEventStream(w.Header().Get("Content-Type"))) {
		w.status = http.StatusBadGateway
		w.writeSanitizedError()
		return false
	}
	sanitizeModelLabSuccessHeaders(w.Header())
	setModelLabProxyNoStore(w)
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(w.status)
	return true
}

// writeSanitizedStream forwards complete SSE frames as soon as they arrive.
// Buffering a frame is required because SSE joins multiple data lines before
// dispatch; inspecting lines independently could leak a split provider error.
func (w *modelLabResponseWriter) writeSanitizedStream(body []byte, final bool) error {
	if w.streamWriteFailure != nil {
		return w.streamWriteFailure
	}
	streamLimit := w.maxStreamBytes
	if streamLimit <= 0 {
		streamLimit = defaultMaxModelLabStreamBytes
	}
	if int64(len(body)) > streamLimit-w.streamBytes {
		return w.rejectOversizedStream(errModelLabSSEStreamTooLarge)
	}
	w.streamBytes += int64(len(body))
	w.streamBuffer = append(w.streamBuffer, body...)
	for len(w.streamBuffer) > 0 {
		frameEnd := modelLabSSEFrameEnd(w.streamBuffer)
		if frameEnd < 0 && !final {
			if len(w.streamBuffer) > maxModelLabSSEFrameBytes {
				return w.rejectOversizedStream(errModelLabSSEFrameTooLarge)
			}
			return nil
		}
		frameLength := frameEnd
		if frameEnd < 0 {
			frameLength = len(w.streamBuffer)
		}
		if frameLength > maxModelLabSSEFrameBytes {
			return w.rejectOversizedStream(errModelLabSSEFrameTooLarge)
		}
		frame := append([]byte(nil), w.streamBuffer[:frameLength]...)
		w.streamBuffer = w.streamBuffer[frameLength:]
		frame = sanitizeModelLabSSEFrame(frame)
		if _, err := w.ResponseWriter.Write(frame); err != nil {
			w.streamWriteFailure = err
			return err
		}
	}
	return nil
}

func (w *modelLabResponseWriter) rejectOversizedStream(limitError error) error {
	w.streamBuffer = nil
	_, writeErr := w.ResponseWriter.Write(modelLabSanitizedStreamErrorFrame())
	if writeErr != nil {
		w.streamWriteFailure = writeErr
		return writeErr
	}
	w.streamWriteFailure = limitError
	return w.streamWriteFailure
}

func modelLabSSEFrameEnd(buffer []byte) int {
	lineStart := 0
	for index := 0; index < len(buffer); {
		if buffer[index] != '\r' && buffer[index] != '\n' {
			index++
			continue
		}
		lineEnd := index + 1
		if buffer[index] == '\r' && lineEnd < len(buffer) && buffer[lineEnd] == '\n' {
			lineEnd++
		}
		if index == lineStart {
			return lineEnd
		}
		lineStart = lineEnd
		index = lineEnd
	}
	return -1
}

func modelLabStreamByteLimit(maxOutputTokens int) int64 {
	if maxOutputTokens <= 0 {
		return defaultMaxModelLabStreamBytes
	}
	if int64(maxOutputTokens) >= hardMaxModelLabStreamBytes/modelLabBytesPerOutputToken {
		return hardMaxModelLabStreamBytes
	}
	limit := int64(maxOutputTokens) * modelLabBytesPerOutputToken
	if limit < minModelLabStreamBytes {
		return minModelLabStreamBytes
	}
	if limit > hardMaxModelLabStreamBytes {
		return hardMaxModelLabStreamBytes
	}
	return limit
}

func sanitizeModelLabSSEFrame(frame []byte) []byte {
	normalized := strings.ReplaceAll(string(frame), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	eventType := ""
	dataLines := make([]string, 0, 1)
	for _, line := range strings.Split(normalized, "\n") {
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			eventType = strings.TrimSpace(value)
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	payload := strings.Join(dataLines, "\n")
	if strings.EqualFold(eventType, "error") || modelLabStreamPayloadIsError(strings.TrimSpace(payload)) {
		return modelLabSanitizedStreamErrorFrame()
	}
	return frame
}

func modelLabSanitizedStreamErrorFrame() []byte {
	return []byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"model_lab_upstream_error\",\"message\":\"The provider stopped the stream.\"}}\n\n")
}

func modelLabIsEventStream(contentType string) bool {
	mediaType, _, found := strings.Cut(strings.ToLower(contentType), ";")
	if !found {
		mediaType = strings.ToLower(contentType)
	}
	return strings.TrimSpace(mediaType) == "text/event-stream"
}

func modelLabStreamPayloadIsError(raw string) bool {
	var payload map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return false
	}
	if _, ok := payload["error"]; ok {
		return true
	}
	var eventType string
	return json.Unmarshal(payload["type"], &eventType) == nil && strings.EqualFold(eventType, "error")
}

func (w *modelLabResponseWriter) writeSanitizedError() {
	if w.wroteError {
		return
	}
	w.wroteError = true
	sanitizeModelLabErrorHeaders(w.Header())
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
	status, code := sanitizedModelLabDownstreamError(w.status)
	w.status = status
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(status)
	}
	payload, _ := json.Marshal(map[string]string{"error": code})
	_, _ = w.ResponseWriter.Write(append(payload, '\n'))
}

func sanitizeModelLabErrorHeaders(headers http.Header) {
	allowed := map[string]struct{}{
		"Access-Control-Allow-Credentials": {},
		"Access-Control-Allow-Origin":      {},
		"Cache-Control":                    {},
		"Content-Security-Policy":          {},
		"Pragma":                           {},
		"Referrer-Policy":                  {},
		"Retry-After":                      {},
		"Vary":                             {},
		"X-Content-Type-Options":           {},
		"X-Frame-Options":                  {},
	}
	for name := range headers {
		if _, ok := allowed[http.CanonicalHeaderKey(name)]; !ok {
			headers.Del(name)
		}
	}
}

func sanitizeModelLabSuccessHeaders(headers http.Header) {
	allowed := map[string]struct{}{
		"Access-Control-Allow-Credentials": {},
		"Access-Control-Allow-Origin":      {},
		"Cache-Control":                    {},
		"Content-Security-Policy":          {},
		"Content-Type":                     {},
		"Pragma":                           {},
		"Referrer-Policy":                  {},
		"Vary":                             {},
		"X-Content-Type-Options":           {},
		"X-Frame-Options":                  {},
	}
	for name := range headers {
		if _, ok := allowed[http.CanonicalHeaderKey(name)]; !ok {
			headers.Del(name)
		}
	}
}

func sanitizedModelLabDownstreamError(status int) (int, string) {
	if status < http.StatusOK || (status >= http.StatusMultipleChoices && status < http.StatusBadRequest) {
		return http.StatusBadGateway, "model_lab_upstream_unavailable"
	}
	switch status {
	case http.StatusForbidden:
		return status, "model_lab_access_denied"
	case http.StatusTooManyRequests:
		return status, "model_lab_quota_exceeded"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return status, "model_lab_upstream_unavailable"
	default:
		if status >= http.StatusInternalServerError {
			return status, "model_lab_upstream_error"
		}
		return status, "invalid_model_lab_request"
	}
}
