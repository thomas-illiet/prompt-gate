package runtime

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const maxNativeSessionValueLength = 512

var nativeSessionHeaders = [...]string{
	"x-claude-code-session-id",
	"x-openwebui-chat-id",
	"x-coder-chat-id",
	"x-kilocode-taskid",
	"x-client-session-id",
	"x-interaction-id",
	"x-mux-workspace-id",
	"x-session-id",
	"session-id",
	"session_id",
}

type nativeSessionContextKey struct{}

// NativeSession identifies conversation metadata supplied by a supported client.
type NativeSession struct {
	SessionID       string
	SessionSource   string
	ParentSessionID string
	MessageID       string
}

// NativeSessionFromContext returns client-supplied conversation metadata.
func NativeSessionFromContext(ctx context.Context) (NativeSession, bool) {
	session, ok := ctx.Value(nativeSessionContextKey{}).(NativeSession)
	return session, ok
}

// WithNativeSession returns a context carrying client-supplied conversation metadata.
func WithNativeSession(ctx context.Context, session NativeSession) context.Context {
	return context.WithValue(ctx, nativeSessionContextKey{}, session)
}

// SessionContextMiddleware extracts W3C trace context and allowlisted native session headers.
func SessionContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		session := nativeSessionFromHeaders(r.Header)
		if session != (NativeSession{}) {
			ctx = WithNativeSession(ctx, session)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func nativeSessionFromHeaders(headers http.Header) NativeSession {
	session := NativeSession{}
	for _, name := range nativeSessionHeaders {
		if value, ok := validNativeSessionValue(headers.Get(name)); ok {
			session.SessionID = value
			session.SessionSource = name
			break
		}
	}
	if value, ok := validNativeSessionValue(headers.Get("x-parent-session-id")); ok {
		session.ParentSessionID = value
	}
	if value, ok := validNativeSessionValue(headers.Get("x-openwebui-message-id")); ok {
		session.MessageID = value
	}
	return session
}

func validNativeSessionValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxNativeSessionValueLength {
		return "", false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] == 0x7f {
			return "", false
		}
	}
	return value, true
}
