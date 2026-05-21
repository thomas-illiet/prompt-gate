package auth

import "context"

type userContextKey struct{}

// ContextWithUser stores a UserProfile in the context.
func ContextWithUser(ctx context.Context, user UserProfile) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// UserFromContext retrieves a UserProfile from the context.
func UserFromContext(ctx context.Context) (UserProfile, bool) {
	user, ok := ctx.Value(userContextKey{}).(UserProfile)
	return user, ok
}
