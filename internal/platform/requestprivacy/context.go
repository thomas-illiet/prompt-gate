// Package requestprivacy carries per-request content capture policy through the
// proxy pipeline.
package requestprivacy

import (
	"context"
	"net/http"
	"sync/atomic"
)

type contextKey struct{}

// State is shared by every context derived from a request. It is intentionally
// mutable so outer observability middleware can see a stricter policy selected
// later by authentication middleware, before request or response content is
// persisted or exported.
type State struct {
	metadataOnly atomic.Bool
}

// Middleware installs a request-scoped privacy state. It must wrap any debug or
// output-capture middleware that needs to observe policy changes made by an
// inner authentication middleware.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if stateFromContext(r.Context()) != nil {
			next.ServeHTTP(w, r)
			return
		}
		state := &State{}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, state)))
	})
}

// MarkMetadataOnly selects the strict no-content-capture policy. The returned
// context must be installed on the request. When Middleware already installed
// shared state, outer middleware observes the change immediately.
func MarkMetadataOnly(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	state := stateFromContext(ctx)
	if state == nil {
		state = &State{}
		ctx = context.WithValue(ctx, contextKey{}, state)
	}
	state.metadataOnly.Store(true)
	return ctx
}

// IsMetadataOnly reports whether request and response content must not be
// captured, persisted, or exported.
func IsMetadataOnly(ctx context.Context) bool {
	state := stateFromContext(ctx)
	return state != nil && state.metadataOnly.Load()
}

func stateFromContext(ctx context.Context) *State {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(contextKey{}).(*State)
	return state
}
