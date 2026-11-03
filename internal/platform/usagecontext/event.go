package usagecontext

import "context"

type eventIDKey struct{}

// WithEventID preserves a logical recording attempt across ambiguous retries.
func WithEventID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, eventIDKey{}, id)
}
func EventID(ctx context.Context) string { id, _ := ctx.Value(eventIDKey{}).(string); return id }
