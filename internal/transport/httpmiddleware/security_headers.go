package middleware

import "net/http"

const (
	contentSecurityPolicy = "frame-ancestors 'none'; base-uri 'self'; object-src 'none'"
	permissionsPolicy     = "camera=(), microphone=(), geolocation=()"
)

// SecurityHeaders adds baseline browser hardening headers to every response.
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := w.Header()
			headers.Set("Content-Security-Policy", contentSecurityPolicy)
			headers.Set("Permissions-Policy", permissionsPolicy)
			headers.Set("Referrer-Policy", "no-referrer")
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("X-Frame-Options", "DENY")

			next.ServeHTTP(w, r)
		})
	}
}
