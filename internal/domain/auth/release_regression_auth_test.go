package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"promptgate/backend/internal/platform/redisstore"
)

type regressionResolver struct{ entered, resume chan struct{} }

func (r regressionResolver) UserByID(context.Context, string) (UserProfile, error) {
	close(r.entered)
	<-r.resume
	return testUser("user-1"), nil
}
func TestRegressionLogoutMustNotResurrectSession(t *testing.T) {
	server, a, b := newRedisSessionStorePair(t, time.Hour)
	session, err := a.CreateSession(testUser("user-1"), "id-token")
	if err != nil {
		t.Fatal(err)
	}
	resolver := regressionResolver{make(chan struct{}), make(chan struct{})}
	a.userResolver = resolver
	done := make(chan struct{})
	go func() { a.Session(context.Background(), session.ID); close(done) }()
	<-resolver.entered
	b.DeleteSession(session.ID)
	close(resolver.resume)
	<-done
	if server.Exists(redisstore.AuthSessionKey(session.ID)) {
		t.Fatal("logged-out session was recreated by concurrent profile refresh")
	}
}

func TestTransientProfileFailurePreservesSession(t *testing.T) {
	for _, redis := range []bool{false, true} {
		t.Run(fmt.Sprint(redis), func(t *testing.T) {
			store := NewSessionStore(testUserResolver{user: testUser("user-1")}, time.Hour)
			if redis {
				_, store, _ = newRedisSessionStorePair(t, time.Hour)
			}
			session, err := store.CreateSession(testUser("user-1"), "id-token")
			if err != nil {
				t.Fatal(err)
			}
			store.userResolver = testUserResolver{err: errors.New("database temporarily unavailable")}
			if _, ok := store.Session(context.Background(), session.ID); ok {
				t.Fatal("failed profile refresh must reject request")
			}
			store.userResolver = testUserResolver{user: testUser("user-1")}
			if _, ok := store.Session(context.Background(), session.ID); !ok {
				t.Fatal("transient failure destroyed the session")
			}
		})
	}
}
