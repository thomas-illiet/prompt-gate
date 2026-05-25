package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

// Chain wraps handler with the given middlewares in order (first middleware is outermost).
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	chained := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		chained = middlewares[i](chained)
	}

	return chained
}
