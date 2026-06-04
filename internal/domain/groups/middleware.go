package groups

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/proxylimits"
)

type MiddlewareOptions struct {
	MaxBufferedRequestBytes int64
}

// Middleware enforces group-based provider and model access from an in-memory snapshot.
func Middleware(snapshot *SnapshotStore, logger *slog.Logger) func(http.Handler) http.Handler {
	return MiddlewareWithOptions(snapshot, logger, MiddlewareOptions{})
}

// MiddlewareWithOptions enforces group-based provider and model access with explicit buffering limits.
func MiddlewareWithOptions(snapshot *SnapshotStore, logger *slog.Logger, opts MiddlewareOptions) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.MaxBufferedRequestBytes <= 0 {
		opts.MaxBufferedRequestBytes = proxylimits.DefaultMaxBufferedRequestBytes
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "missing_authenticated_user"})
				return
			}
			providerName := requestProviderName(r.URL.Path)
			if providerName == "" || snapshot == nil || !snapshot.KnownProvider(providerName) {
				logger.Warn("group access denied for unknown provider", "provider", providerName, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
				return
			}
			providerType, _ := snapshot.ProviderType(providerName)
			if isModelBypassRoute(providerType, r.URL.Path) {
				if !snapshot.AllowsProvider(user.ID, providerName) {
					logger.Warn("group access denied for provider route", "provider", providerName, "path", r.URL.Path, "user_id", user.ID)
					writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
					return
				}
				if isFilterableModelListRoute(providerType, r.URL.Path, r.Method) {
					filterModelListResponse(w, r, next, snapshot, user.ID, providerName)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			model, err := requestModel(r, opts.MaxBufferedRequestBytes)
			if err != nil {
				if errors.Is(err, proxylimits.ErrExceeded) {
					logger.Warn("group access denied for oversized request body", "provider", providerName, "user_id", user.ID, "limit_bytes", opts.MaxBufferedRequestBytes)
					writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request_body_too_large"})
					return
				}
				logger.Warn("group access denied", "provider", providerName, "model", model, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
				return
			}
			if model == "" {
				logger.Warn("group access denied for request without model", "provider", providerName, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_model_required"})
				return
			}
			if !snapshot.Allows(user.ID, providerName, model) {
				logger.Warn("group access denied", "provider", providerName, "model", model, "user_id", user.ID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "group_access_denied"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestProviderName extracts the provider route prefix from the request path.
func requestProviderName(path string) string {
	path = strings.TrimPrefix(path, "/")
	providerName, _, _ := strings.Cut(path, "/")
	return strings.TrimSpace(providerName)
}

// requestProviderPath extracts the route suffix after the provider name.
func requestProviderPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	_, suffix, ok := strings.Cut(path, "/")
	if !ok {
		return "/"
	}
	return "/" + strings.TrimPrefix(suffix, "/")
}

// isModelBypassRoute reports whether a provider route can be checked without a request model.
func isModelBypassRoute(providerType provider.ProviderType, path string) bool {
	providerPath := requestProviderPath(path)
	switch providerType {
	case provider.ProviderTypeOpenAI:
		return routeExactOrSubtree(providerPath, "/v1/models") ||
			routeExactOrSubtree(providerPath, "/v1/conversations") ||
			routeSubtree(providerPath, "/v1/responses/")
	case provider.ProviderTypeOllama:
		return routeExactOrSubtree(providerPath, "/v1/models")
	case provider.ProviderTypeAnthropic:
		return routeExactOrSubtree(providerPath, "/v1/models") ||
			providerPath == "/v1/messages/count_tokens" ||
			routeSubtree(providerPath, "/api/event_logging/")
	default:
		return false
	}
}

// isFilterableModelListRoute reports whether a provider model list response should be masked.
func isFilterableModelListRoute(providerType provider.ProviderType, path, method string) bool {
	if method != http.MethodGet {
		return false
	}
	if providerType != provider.ProviderTypeOpenAI && providerType != provider.ProviderTypeOllama {
		return false
	}
	providerPath := requestProviderPath(path)
	return providerPath == "/v1/models" || providerPath == "/v1/models/"
}

// routeExactOrSubtree reports whether path matches route or a child route.
func routeExactOrSubtree(path, route string) bool {
	return path == route || strings.HasPrefix(path, route+"/")
}

// routeSubtree reports whether path is under routePrefix.
func routeSubtree(path, routePrefix string) bool {
	return strings.HasPrefix(path, routePrefix)
}

// requestModel reads and restores the request body while extracting the JSON model field.
func requestModel(r *http.Request, maxBufferedRequestBytes int64) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	raw, err := proxylimits.ReadAll(r.Body, maxBufferedRequestBytes)
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", nil
	}

	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Model), nil
}

type responseCapture struct {
	header     http.Header
	statusCode int
	body       bytes.Buffer
}

func newResponseCapture() *responseCapture {
	return &responseCapture{
		header: http.Header{},
	}
}

func (c *responseCapture) Header() http.Header {
	return c.header
}

func (c *responseCapture) WriteHeader(statusCode int) {
	if c.statusCode == 0 {
		c.statusCode = statusCode
	}
}

func (c *responseCapture) Write(raw []byte) (int, error) {
	if c.statusCode == 0 {
		c.statusCode = http.StatusOK
	}
	return c.body.Write(raw)
}

func (c *responseCapture) status() int {
	if c.statusCode == 0 {
		return http.StatusOK
	}
	return c.statusCode
}

// filterModelListResponse masks upstream model list entries not allowed by group rules.
func filterModelListResponse(w http.ResponseWriter, r *http.Request, next http.Handler, snapshot *SnapshotStore, userID, providerName string) {
	capture := newResponseCapture()
	next.ServeHTTP(capture, r)

	statusCode := capture.status()
	body := capture.body.Bytes()
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		writeCapturedResponse(w, capture.header, statusCode, body, false)
		return
	}

	filtered, changed := filteredModelListBody(body, snapshot, userID, providerName)
	writeCapturedResponse(w, capture.header, statusCode, filtered, changed)
}

func writeCapturedResponse(w http.ResponseWriter, header http.Header, statusCode int, body []byte, changed bool) {
	for key, values := range header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if changed {
		w.Header().Del("Content-Length")
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func filteredModelListBody(raw []byte, snapshot *SnapshotStore, userID, providerName string) ([]byte, bool) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw, false
	}

	changed := false
	for _, key := range []string{"data", "models"} {
		models, ok := payload[key]
		if !ok {
			continue
		}
		filtered, nextChanged, recognized := filterModelArray(models, snapshot, userID, providerName)
		if !recognized {
			return raw, false
		}
		if nextChanged {
			payload[key] = filtered
			changed = true
		}
	}
	if !changed {
		return raw, false
	}

	filtered, err := json.Marshal(payload)
	if err != nil {
		return raw, false
	}
	return filtered, true
}

func filterModelArray(raw json.RawMessage, snapshot *SnapshotStore, userID, providerName string) (json.RawMessage, bool, bool) {
	var models []json.RawMessage
	if err := json.Unmarshal(raw, &models); err != nil {
		return raw, false, false
	}

	filtered := make([]json.RawMessage, 0, len(models))
	for _, model := range models {
		identifier := modelListIdentifier(model)
		if identifier == "" || !snapshot.Allows(userID, providerName, identifier) {
			continue
		}
		filtered = append(filtered, model)
	}
	if len(filtered) == len(models) {
		return raw, false, true
	}

	next, err := json.Marshal(filtered)
	if err != nil {
		return raw, false, false
	}
	return next, true, true
}

func modelListIdentifier(raw json.RawMessage) string {
	var payload struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	for _, value := range []string{payload.ID, payload.Name, payload.DisplayName} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

// writeJSON sends a JSON response for group middleware errors.
func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
