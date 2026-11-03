package auth

import (
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestRegressionEncodedRedirectMustRemainOnFrontend(t *testing.T) {
	s := OIDCService{frontendBaseURL: "https://app.example.com", sessionStore: NewSessionStore(nil, time.Hour), oauthConfig: oauth2.Config{ClientID: "audit", Endpoint: oauth2.Endpoint{AuthURL: "https://issuer.example.com/authorize"}}}
	login, err := s.AuthorizationURL("/%252Fevil.example", "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(login)
	if err != nil {
		t.Fatal(err)
	}
	pending, ok := s.ConsumeAuthorizationRequest(parsed.Query().Get("state"))
	if !ok {
		t.Fatal("missing pending request")
	}
	next, err := s.FrontendRedirectURL(pending.RedirectPath, pending.FrontendBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err = url.Parse(next)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "app.example.com" {
		t.Fatalf("login redirect escaped frontend origin: %s", next)
	}
}
