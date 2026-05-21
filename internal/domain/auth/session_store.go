package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"promptgate/backend/internal/platform/redisstore"
)

const (
	authRequestTTL    = 10 * time.Minute
	DefaultSessionTTL = 8 * time.Hour
	sessionIDBytes    = 32
)

type Session struct {
	ID        string
	UserID    string
	User      UserProfile
	IDToken   string
	ExpiresAt time.Time
}

type AuthorizationRequest struct {
	State           string
	Nonce           string
	CodeVerifier    string
	RedirectPath    string
	FrontendBaseURL string
	ExpiresAt       time.Time
}

type SessionStore struct {
	mu           sync.RWMutex
	userResolver UserResolver
	redis        *redisstore.Store
	sessionTTL   time.Duration
	sessions     map[string]Session
	authRequests map[string]AuthorizationRequest
}

// NewSessionStore creates an in-memory session store backed by the given user resolver.
func NewSessionStore(userResolver UserResolver, sessionTTL time.Duration) *SessionStore {
	return newSessionStore(userResolver, sessionTTL, nil)
}

// NewRedisSessionStore creates a Redis-backed session store backed by the given user resolver.
func NewRedisSessionStore(userResolver UserResolver, sessionTTL time.Duration, redisStore *redisstore.Store) *SessionStore {
	return newSessionStore(userResolver, sessionTTL, redisStore)
}

// newSessionStore initializes shared session storage with optional Redis persistence.
func newSessionStore(userResolver UserResolver, sessionTTL time.Duration, redisStore *redisstore.Store) *SessionStore {
	if sessionTTL <= 0 {
		sessionTTL = DefaultSessionTTL
	}

	return &SessionStore{
		userResolver: userResolver,
		redis:        redisStore,
		sessionTTL:   sessionTTL,
		sessions:     make(map[string]Session),
		authRequests: make(map[string]AuthorizationRequest),
	}
}

// CreateAuthorizationRequest generates a new PKCE-protected OIDC authorization request and stores it.
func (s *SessionStore) CreateAuthorizationRequest(redirectPath string, frontendBaseURL string) (AuthorizationRequest, error) {
	state, err := randomToken(sessionIDBytes)
	if err != nil {
		return AuthorizationRequest{}, err
	}

	nonce, err := randomToken(sessionIDBytes)
	if err != nil {
		return AuthorizationRequest{}, err
	}

	codeVerifier, err := randomToken(sessionIDBytes)
	if err != nil {
		return AuthorizationRequest{}, err
	}

	request := AuthorizationRequest{
		State:           state,
		Nonce:           nonce,
		CodeVerifier:    codeVerifier,
		RedirectPath:    redirectPath,
		FrontendBaseURL: frontendBaseURL,
		ExpiresAt:       time.Now().Add(authRequestTTL),
	}

	if s.redis.Enabled() {
		if err := s.redis.SetJSON(context.Background(), redisstore.AuthRequestKey(state), request, time.Until(request.ExpiresAt)); err != nil {
			return AuthorizationRequest{}, err
		}
		return request, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(time.Now())
	s.authRequests[state] = request

	return request, nil
}

// ConsumeAuthorizationRequest retrieves and removes an authorization request by state, returning false if not found or expired.
func (s *SessionStore) ConsumeAuthorizationRequest(state string) (AuthorizationRequest, bool) {
	if s.redis.Enabled() {
		var request AuthorizationRequest
		key := redisstore.AuthRequestKey(state)
		ok, err := s.redis.GetDelJSON(context.Background(), key, &request)
		if err != nil || !ok || time.Now().After(request.ExpiresAt) {
			return AuthorizationRequest{}, false
		}
		return request, true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(time.Now())

	request, ok := s.authRequests[state]
	if !ok {
		return AuthorizationRequest{}, false
	}

	delete(s.authRequests, state)

	return request, true
}

// CreateSession stores a new authenticated session for the given user and returns it.
func (s *SessionStore) CreateSession(user UserProfile, idToken string) (Session, error) {
	sessionID, err := randomToken(sessionIDBytes)
	if err != nil {
		return Session{}, err
	}

	session := Session{
		ID:        sessionID,
		UserID:    user.ID,
		User:      user,
		IDToken:   idToken,
		ExpiresAt: time.Now().UTC().Add(s.sessionTTL),
	}

	if s.redis.Enabled() {
		if err := s.redis.SetJSON(context.Background(), redisstore.AuthSessionKey(sessionID), session, time.Until(session.ExpiresAt)); err != nil {
			return Session{}, err
		}
		return session, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(time.Now())
	s.sessions[sessionID] = session

	return session, nil
}

// Session retrieves a session by ID, refreshing the user profile from the resolver if available.
func (s *SessionStore) Session(ctx context.Context, sessionID string) (Session, bool) {
	if s.redis.Enabled() {
		var session Session
		key := redisstore.AuthSessionKey(sessionID)
		ok, err := s.redis.GetJSON(ctx, key, &session)
		if err != nil || !ok || time.Now().After(session.ExpiresAt) {
			_ = s.redis.Del(ctx, key)
			return Session{}, false
		}
		return s.refreshSessionUser(ctx, session)
	}

	s.mu.Lock()
	s.cleanupExpiredLocked(time.Now())

	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return Session{}, false
	}

	s.mu.Unlock()

	return s.refreshSessionUser(ctx, session)
}

// refreshSessionUser reloads the session user profile and drops stale sessions.
func (s *SessionStore) refreshSessionUser(ctx context.Context, session Session) (Session, bool) {
	if s.userResolver == nil || session.UserID == "" {
		return session, true
	}

	user, err := s.userResolver.UserByID(ctx, session.UserID)
	if err != nil {
		s.DeleteSession(session.ID)
		return Session{}, false
	}

	session.User = user
	if s.redis.Enabled() {
		if err := s.redis.SetJSON(ctx, redisstore.AuthSessionKey(session.ID), session, time.Until(session.ExpiresAt)); err != nil {
			s.DeleteSession(session.ID)
			return Session{}, false
		}
		return session, true
	}

	s.mu.Lock()
	if existing, exists := s.sessions[session.ID]; exists {
		existing.User = user
		s.sessions[session.ID] = existing
		session = existing
	}
	s.mu.Unlock()

	return session, true
}

// DeleteSession removes a session by ID.
func (s *SessionStore) DeleteSession(sessionID string) {
	if s.redis.Enabled() {
		_ = s.redis.Del(context.Background(), redisstore.AuthSessionKey(sessionID))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
}

// cleanupExpiredLocked removes expired authorization requests and sessions. Caller must hold mu.
func (s *SessionStore) cleanupExpiredLocked(now time.Time) {
	for state, request := range s.authRequests {
		if now.After(request.ExpiresAt) {
			delete(s.authRequests, state)
		}
	}

	for sessionID, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
		}
	}
}

// randomToken generates a cryptographically random base64url-encoded token of the given byte size.
func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
