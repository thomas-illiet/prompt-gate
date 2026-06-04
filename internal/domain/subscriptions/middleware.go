package subscriptions

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
)

// Middleware blocks proxy traffic when the authenticated identity has no effective
// subscription plan or has exhausted one of its quota windows.
func Middleware(store *RedisStore, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
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

			status, err := store.CurrentQuota(r.Context(), user.ID, time.Now().UTC())
			if err != nil {
				logger.Error("subscription quota check failed", "user_id", user.ID, "error", err)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "subscription_quota_unavailable"})
				return
			}
			if !status.HasSubscription {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "subscription_required"})
				return
			}
			if exceeded, ok := exceededQuota(status); ok {
				if exceeded.ResetAt != nil {
					retryAfter := time.Until(*exceeded.ResetAt)
					if retryAfter > 0 {
						w.Header().Set("Retry-After", strconv.FormatInt(int64(retryAfter.Seconds()), 10))
					}
				}
				writeJSON(w, http.StatusTooManyRequests, exceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type quotaExceededResponse struct {
	Error       string     `json:"error"`
	Window      string     `json:"window"`
	UsedTokens  int64      `json:"usedTokens"`
	QuotaTokens int64      `json:"quotaTokens"`
	ResetAt     *time.Time `json:"resetAt"`
}

func exceededQuota(status QuotaStatus) (quotaExceededResponse, bool) {
	if status.Quota5HTokens != nil && status.Used5HTokens >= *status.Quota5HTokens {
		return quotaExceededResponse{
			Error:       "quota_exceeded",
			Window:      Window5H,
			UsedTokens:  status.Used5HTokens,
			QuotaTokens: *status.Quota5HTokens,
			ResetAt:     cloneTimePtr(status.Reset5HAt),
		}, true
	}
	if status.Quota7DTokens != nil && status.Used7DTokens >= *status.Quota7DTokens {
		return quotaExceededResponse{
			Error:       "quota_exceeded",
			Window:      Window7D,
			UsedTokens:  status.Used7DTokens,
			QuotaTokens: *status.Quota7DTokens,
			ResetAt:     cloneTimePtr(status.Reset7DAt),
		}, true
	}
	return quotaExceededResponse{}, false
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
	case strings.HasPrefix(providerPath, "/v1/responses/"):
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
