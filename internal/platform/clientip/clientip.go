package clientip

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type contextKey struct{}

// Resolve extracts the client IP, optionally trusting forwarding headers.
func Resolve(r *http.Request, trustForwardHeaders bool) string {
	if trustForwardHeaders {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first, _, _ := strings.Cut(forwarded, ",")
			if first = strings.TrimSpace(first); first != "" {
				return first
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// ContextWithClientIP stores the resolved client IP in a context.
func ContextWithClientIP(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, contextKey{}, strings.TrimSpace(value))
}

// FromContext retrieves a resolved client IP from a context.
func FromContext(ctx context.Context) string {
	value, _ := ctx.Value(contextKey{}).(string)
	return value
}

// Middleware resolves and stores the client IP for downstream request handlers.
func Middleware(trustForwardHeaders bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := ContextWithClientIP(r.Context(), Resolve(r, trustForwardHeaders))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
