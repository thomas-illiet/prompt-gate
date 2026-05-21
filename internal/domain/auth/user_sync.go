package auth

import "context"

type UserSynchronizer interface {
	SyncUser(ctx context.Context, identity Identity) (UserProfile, error)
}

type UserResolver interface {
	UserByID(ctx context.Context, id string) (UserProfile, error)
}
