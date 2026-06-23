package httpapi

import (
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
)

type server struct {
	config        config.Config
	db            *gorm.DB
	oidcService   *auth.OIDCService
	sessionStore  *auth.SessionStore
	userService   *users.Service
	tokenService  *tokens.Service
	groups        *groups.Service
	proxyService  *proxy.Service
	providers     *provider.Service
	monitoring    *monitoring.Service
	pricing       *pricing.Service
	subscriptions *subscriptions.Service
	quotaRedis    *subscriptions.RedisStore
}

// handleLogin initiates the OIDC authorization flow and redirects to the provider.
func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	slog.Info(
		"starting oidc login",
		"redirect",
		r.URL.Query().Get("redirect"),
		"frontend_origin",
		r.URL.Query().Get("frontend_origin"),
		"path",
		r.URL.Path,
	)

	loginURL, err := s.oidcService.AuthorizationURL(
		r.URL.Query().Get("redirect"),
		r.URL.Query().Get("frontend_origin"),
	)
	if err != nil {
		slog.Error("failed to build oidc login url", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	http.Redirect(w, r, loginURL, http.StatusFound)
}

// handleCallback processes the OIDC provider callback, exchanges the code for a session, and redirects the user.
func (s *server) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	state := query.Get("state")
	redirectPath := query.Get("redirect")

	if query.Get("error") != "" {
		frontendBaseURL := ""
		slog.Warn(
			"oidc callback returned provider error",
			"error",
			query.Get("error"),
			"description",
			query.Get("error_description"),
		)

		if request, ok := s.oidcService.ConsumeAuthorizationRequest(state); ok {
			redirectPath = request.RedirectPath
			frontendBaseURL = request.FrontendBaseURL
		}

		redirectURL, err := s.oidcService.LoginErrorRedirectURL(
			redirectPath,
			query.Get("error"),
			query.Get("error_description"),
			frontendBaseURL,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	session, nextPath, frontendBaseURL, err := s.oidcService.ExchangeCode(
		r.Context(),
		state,
		query.Get("code"),
	)
	if err != nil {
		slog.Error("oidc callback exchange failed", "error", err)
		redirectURL, redirectErr := s.oidcService.LoginErrorRedirectURL(
			redirectPath,
			"authentication_failed",
			err.Error(),
			frontendBaseURL,
		)
		if redirectErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	slog.Info(
		"oidc callback succeeded",
		"user_id",
		session.User.ID,
		"user_role",
		session.User.Role,
		"next_path",
		nextPath,
	)

	setSessionCookie(w, s.config, session)

	redirectURL, err := s.oidcService.FrontendRedirectURL(
		nextPath,
		frontendBaseURL,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// handleLogout deletes the current session and redirects to the OIDC end-session or frontend login page.
func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	redirectPath := r.URL.Query().Get("redirect")
	frontendOrigin := r.URL.Query().Get("frontend_origin")

	session, ok := sessionFromRequest(r, s.sessionStore, s.config.SessionCookieName)
	if ok {
		slog.Info(
			"logging out session",
			"user_id",
			session.User.ID,
			"user_role",
			session.User.Role,
			"redirect",
			redirectPath,
		)
		s.oidcService.DeleteSession(session.ID)
	} else {
		slog.Info("logout requested without active session", "redirect", redirectPath)
	}

	clearSessionCookie(w, s.config)

	logoutURL, err := s.oidcService.FrontendRedirectURL("/login", frontendOrigin)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	if ok {
		logoutURL, err = s.oidcService.LogoutRedirectURL(
			session,
			redirectPath,
			frontendOrigin,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}
	}

	http.Redirect(w, r, logoutURL, http.StatusFound)
}

// handleSession returns the current user's profile if a valid session cookie is present.
func (s *server) handleSession(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromRequest(r, s.sessionStore, s.config.SessionCookieName)
	if !ok {
		slog.Debug("session lookup failed")
		clearSessionCookie(w, s.config)
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "not authenticated",
		})
		return
	}

	slog.Debug("session lookup succeeded", "user_id", session.User.ID, "user_role", session.User.Role)
	writeJSON(w, http.StatusOK, session.User)
}

// setSessionCookie writes the session ID as an HTTP-only cookie.
func setSessionCookie(w http.ResponseWriter, cfg config.Config, session auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   auth.SessionCookieExpiry(session.ExpiresAt),
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.SessionCookieSecure(),
	})
}

// clearSessionCookie expires the session cookie.
func clearSessionCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.SessionCookieSecure(),
	})
}

// sessionFromRequest extracts and validates the session from the named cookie.
func sessionFromRequest(r *http.Request, sessionStore *auth.SessionStore, cookieName string) (auth.Session, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return auth.Session{}, false
	}

	return sessionStore.Session(r.Context(), cookie.Value)
}
