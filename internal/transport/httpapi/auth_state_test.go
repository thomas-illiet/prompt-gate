package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/config"
)

func TestOIDCCallbackBoundToLoginBrowser(t *testing.T) {
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Errorf("unexpected provider call: %s", r.URL.Path)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q}`, issuer.URL, issuer.URL+"/authorize", issuer.URL+"/token", issuer.URL+"/keys")
	}))
	defer issuer.Close()
	cfg := config.APIConfig{KeycloakConfig: config.KeycloakConfig{KeycloakIssuerURL: issuer.URL, KeycloakClientID: "test"}, PublicURLConfig: config.PublicURLConfig{FrontendBaseURL: "https://app.test", BackendBaseURL: "https://api.test"}}
	sessions := auth.NewSessionStore(nil, time.Hour)
	oidc, err := auth.NewOIDCService(context.Background(), cfg, nil, sessions, nil, issuer.Client())
	if err != nil {
		t.Fatal(err)
	}
	s := server{config: cfg, oidcService: oidc, sessionStore: sessions}
	login := httptest.NewRecorder()
	s.handleLogin(login, httptest.NewRequest(http.MethodGet, "/auth/login?redirect=/dashboard", nil))
	target, _ := url.Parse(login.Header().Get("Location"))
	state := target.Query().Get("state")
	cookies := login.Result().Cookies()
	if state == "" || len(cookies) != 1 || cookies[0].Value != state || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("unbound authorization: %s %#v", target, cookies)
	}
	for _, value := range []string{"", "different-browser"} {
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&error=access_denied", nil)
		if value != "" {
			req.AddCookie(&http.Cookie{Name: oidcStateCookieName(cfg), Value: value})
		}
		rec := httptest.NewRecorder()
		s.handleCallback(rec, req)
		if rec.Code != 400 {
			t.Fatalf("accepted callback from another browser: %d", rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state="+state+"&error=access_denied", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	s.handleCallback(rec, req)
	if rec.Code != 302 || !strings.HasPrefix(rec.Header().Get("Location"), "https://app.test/") {
		t.Fatalf("valid callback: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if _, ok := sessions.ConsumeAuthorizationRequest(state); ok {
		t.Fatal("authorization state not consumed")
	}
	if got := rec.Result().Cookies(); len(got) != 1 || got[0].MaxAge != -1 {
		t.Fatal("binding cookie was not cleared")
	}
}
