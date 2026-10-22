package clientip

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type contextKey struct{}

// Options controls how client IP addresses are resolved from requests.
type Options struct {
	TrustForwardHeaders bool
	TrustedProxies      []netip.Prefix
	Debug               bool
	Logger              *slog.Logger
}

type resolution struct {
	clientIP    string
	peerIP      string
	source      string
	peerTrusted bool
}

// Resolve extracts the client IP, optionally trusting forwarding headers.
func Resolve(r *http.Request, trustForwardHeaders bool) string {
	return ResolveWithOptions(r, Options{TrustForwardHeaders: trustForwardHeaders})
}

// ResolveWithOptions extracts the client IP using the configured proxy trust policy.
func ResolveWithOptions(r *http.Request, options Options) string {
	return resolveWithOptions(r, options).clientIP
}

func resolveWithOptions(r *http.Request, options Options) resolution {
	remote := remoteAddrHost(r.RemoteAddr)
	if options.TrustForwardHeaders {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first, _, _ := strings.Cut(forwarded, ",")
			if first = normalizedHeaderIP(first); first != "" {
				return resolution{clientIP: first, peerIP: remote, source: "x-forwarded-for", peerTrusted: true}
			}
		}
		if realIP := normalizedHeaderIP(r.Header.Get("X-Real-IP")); realIP != "" {
			return resolution{clientIP: realIP, peerIP: remote, source: "x-real-ip", peerTrusted: true}
		}
		return resolution{clientIP: remote, peerIP: remote, source: "remote", peerTrusted: true}
	}

	if len(options.TrustedProxies) > 0 {
		trusted := isTrustedProxy(remote, options.TrustedProxies)
		if trusted {
			if forwarded := trustedForwardedFor(r.Header.Get("X-Forwarded-For"), options.TrustedProxies); forwarded != "" {
				return resolution{clientIP: forwarded, peerIP: remote, source: "x-forwarded-for", peerTrusted: true}
			}
			if realIP := normalizedHeaderIP(r.Header.Get("X-Real-IP")); realIP != "" {
				return resolution{clientIP: realIP, peerIP: remote, source: "x-real-ip", peerTrusted: true}
			}
		}
		return resolution{clientIP: remote, peerIP: remote, source: "remote", peerTrusted: trusted}
	}
	return resolution{clientIP: remote, peerIP: remote, source: "remote", peerTrusted: false}
}

func remoteAddrHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func trustedForwardedFor(value string, trustedProxies []netip.Prefix) string {
	parts := strings.Split(value, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		addr := parseAddr(parts[i])
		if !addr.IsValid() {
			continue
		}
		if isTrustedAddr(addr, trustedProxies) {
			continue
		}
		return addr.String()
	}
	return ""
}

func normalizedHeaderIP(value string) string {
	addr := parseAddr(value)
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

func isTrustedProxy(value string, trustedProxies []netip.Prefix) bool {
	addr := parseAddr(value)
	return addr.IsValid() && isTrustedAddr(addr, trustedProxies)
}

func isTrustedAddr(addr netip.Addr, trustedProxies []netip.Prefix) bool {
	addr = addr.Unmap()
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseAddr(value string) netip.Addr {
	host := strings.TrimSpace(value)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
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
	return MiddlewareWithOptions(Options{TrustForwardHeaders: trustForwardHeaders})
}

// MiddlewareWithOptions resolves and stores the client IP for downstream request handlers.
func MiddlewareWithOptions(options Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resolved := resolveWithOptions(r, options)
			if options.Debug {
				logger := options.Logger
				if logger == nil {
					logger = slog.Default()
				}
				logger.Info("client IP resolved",
					"peer_ip", resolved.peerIP,
					"client_ip", resolved.clientIP,
					"source", resolved.source,
					"peer_trusted", resolved.peerTrusted,
				)
			}
			ctx := ContextWithClientIP(r.Context(), resolved.clientIP)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
