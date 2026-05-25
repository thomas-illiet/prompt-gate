package middleware

import "net/http"

// CORS returns a middleware that sets CORS headers for allowed origins.
func CORS(allowedOrigins []string) Middleware {
	allowedOriginMap := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowedOriginMap[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && originAllowed(origin, allowedOriginMap) {
				headers := w.Header()
				headers.Set("Access-Control-Allow-Origin", origin)
				headers.Add("Vary", "Origin")
				headers.Set("Access-Control-Allow-Credentials", "true")
				headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
				headers.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				headers.Set("Access-Control-Max-Age", "600")
			}

			if r.Method == http.MethodOptions {
				if origin == "" || !originAllowed(origin, allowedOriginMap) {
					http.Error(w, "origin not allowed", http.StatusForbidden)
					return
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// originAllowed reports whether origin is present in the allowedOrigins set.
func originAllowed(origin string, allowedOrigins map[string]struct{}) bool {
	if len(allowedOrigins) == 0 {
		return false
	}

	_, ok := allowedOrigins[origin]
	return ok
}
