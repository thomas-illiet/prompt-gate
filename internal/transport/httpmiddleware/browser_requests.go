package middleware

import (
	"mime"
	"net/http"
	"net/url"
)

const MaxAPIRequestBytes int64 = 1 << 20

// BrowserRequests rejects cross-origin mutations and simple form requests before
// they can use ambient session credentials. JSON API clients remain supported.
func BrowserRequests(origins []string) Middleware {
	allowed := make(map[string]struct{})
	for _, raw := range origins {
		if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
			allowed[u.Scheme+"://"+u.Host] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" && r.Referer() != "" {
				if u, err := url.Parse(r.Referer()); err == nil {
					origin = u.Scheme + "://" + u.Host
				}
			}
			if origin != "" {
				if _, ok := allowed[origin]; !ok {
					http.Error(w, "origin not allowed", http.StatusForbidden)
					return
				}
			} else if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
				kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if err != nil || kind != "application/json" {
					http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
					return
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, MaxAPIRequestBytes)
			next.ServeHTTP(w, r)
		})
	}
}
