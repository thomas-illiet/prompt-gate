package auth

import "context"

type userContextKey struct{}
type principalContextKey struct{}

// ContextWithUser stores a UserProfile in the context.
func ContextWithUser(ctx context.Context, user UserProfile) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// ContextWithPrincipal stores an authenticated proxy principal in the context.
func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	ctx = ContextWithUser(ctx, principal.User)
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext retrieves the authenticated proxy principal.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

// UserFromContext retrieves a UserProfile from the context.
func UserFromContext(ctx context.Context) (UserProfile, bool) {
	user, ok := ctx.Value(userContextKey{}).(UserProfile)
	return user, ok
}
