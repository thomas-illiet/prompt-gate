package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"promptgate/backend/internal/platform/config"
)

func TestOIDCAuthorizationRequestLifecycle(t *testing.T) {
	store := NewSessionStore(nil, time.Hour)
	service := OIDCService{
		oauthConfig: oauth2.Config{
			ClientID:    "prompt-gate",
			RedirectURL: "https://api.example.com/api/v1/auth/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL: "https://issuer.example.com/authorize",
			},
			Scopes: []string{"openid", "profile"},
		},
		frontendBaseURL: "http://localhost:3000",
		sessionStore:    store,
	}

	authorizationURL, err := service.AuthorizationURL("/admin?tab=users", "http://127.0.0.1:3000")
	if err != nil {
		t.Fatalf("authorization URL: %v", err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	query := parsed.Query()
	if parsed.Host != "issuer.example.com" || query.Get("client_id") != "prompt-gate" || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || query.Get("nonce") == "" || query.Get("state") == "" {
		t.Fatalf("unexpected authorization URL: %s", authorizationURL)
	}

	request, ok := service.ConsumeAuthorizationRequest(query.Get("state"))
	if !ok {
		t.Fatal("expected authorization request to be consumed")
	}
	if request.RedirectPath != "/admin?tab=users" || request.FrontendBaseURL != "http://127.0.0.1:3000" || request.Nonce != query.Get("nonce") {
		t.Fatalf("unexpected authorization request: %#v", request)
	}
	if _, ok := service.ConsumeAuthorizationRequest(query.Get("state")); ok {
		t.Fatal("authorization request must be one-shot")
	}

	session, err := store.CreateSession(UserProfile{ID: "user-id"}, "id-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	service.DeleteSession(session.ID)
	if _, ok := store.Session(context.Background(), session.ID); ok {
		t.Fatal("expected OIDC service to delete the session")
	}
}

func TestNewOIDCServiceUsesConfiguredRedirectURL(t *testing.T) {
	const issuerURL = "https://issuer.example.com"
	providerMetadata := `{
		"issuer":"https://issuer.example.com",
		"authorization_endpoint":"https://issuer.example.com/authorize",
		"token_endpoint":"https://issuer.example.com/token",
		"jwks_uri":"https://issuer.example.com/jwks",
		"id_token_signing_alg_values_supported":["RS256"]
	}`
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(providerMetadata)),
			Request:    request,
		}, nil
	})}

	const redirectURL = "https://public.example.com/oidc/complete"
	service, err := NewOIDCService(
		context.Background(),
		config.Config{
			KeycloakConfig: config.KeycloakConfig{
				KeycloakIssuerURL: issuerURL,
				KeycloakClientID:  "prompt-gate",
				OIDCRedirectURL:   redirectURL,
			},
			PublicURLConfig: config.PublicURLConfig{FrontendBaseURL: "https://app.example.com"},
		},
		nil,
		NewSessionStore(nil, time.Hour),
		nil,
		client,
	)
	if err != nil {
		t.Fatalf("create OIDC service: %v", err)
	}

	authorizationURL, err := service.AuthorizationURL("", "")
	if err != nil {
		t.Fatalf("authorization URL: %v", err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	if got := parsed.Query().Get("redirect_uri"); got != redirectURL {
		t.Fatalf("redirect_uri = %q, want %q", got, redirectURL)
	}
}

func TestOIDCRedirectURLs(t *testing.T) {
	service := OIDCService{frontendBaseURL: "https://app.example.com/base"}

	loginErrorURL, err := service.LoginErrorRedirectURL("/dashboard?period=7d", "access_denied", "No access", "")
	if err != nil {
		t.Fatalf("login error redirect: %v", err)
	}
	parsed, _ := url.Parse(loginErrorURL)
	if parsed.Path != "/login" || parsed.Query().Get("redirect") != "/dashboard?period=7d" || parsed.Query().Get("authError") != "access_denied" || parsed.Query().Get("authErrorDescription") != "No access" {
		t.Fatalf("unexpected login error URL: %s", loginErrorURL)
	}

	loginErrorURL, err = service.LoginErrorRedirectURL("", "", "", "")
	if err != nil {
		t.Fatalf("minimal login error redirect: %v", err)
	}
	parsed, _ = url.Parse(loginErrorURL)
	if parsed.Query().Get("redirect") != defaultLoginRedirectPath || parsed.Query().Has("authError") || parsed.Query().Has("authErrorDescription") {
		t.Fatalf("unexpected minimal login error URL: %s", loginErrorURL)
	}

	fallback, err := service.LogoutRedirectURL(Session{}, "", "")
	if err != nil || fallback != "https://app.example.com/dashboard" {
		t.Fatalf("logout fallback = %q, %v", fallback, err)
	}

	service.endSessionEndpoint = "https://issuer.example.com/logout?client=prompt-gate"
	logoutURL, err := service.LogoutRedirectURL(Session{IDToken: "raw-id-token"}, "/signed-out", "")
	if err != nil {
		t.Fatalf("logout redirect: %v", err)
	}
	parsed, _ = url.Parse(logoutURL)
	if parsed.Query().Get("client") != "prompt-gate" || parsed.Query().Get("id_token_hint") != "raw-id-token" || parsed.Query().Get("post_logout_redirect_uri") != "https://app.example.com/signed-out" {
		t.Fatalf("unexpected logout URL: %s", logoutURL)
	}

	logoutURL, err = service.LogoutRedirectURL(Session{}, "//evil.example.com", "")
	if err != nil {
		t.Fatalf("logout redirect without token: %v", err)
	}
	parsed, _ = url.Parse(logoutURL)
	if parsed.Query().Has("id_token_hint") || parsed.Query().Get("post_logout_redirect_uri") != "https://app.example.com/login" {
		t.Fatalf("unexpected safe logout URL: %s", logoutURL)
	}

	service.endSessionEndpoint = "%"
	if _, err := service.LogoutRedirectURL(Session{}, "/", ""); err == nil {
		t.Fatal("expected invalid end-session URL error")
	}
	service = OIDCService{frontendBaseURL: "%"}
	if _, err := service.LoginErrorRedirectURL("/", "", "", ""); err == nil {
		t.Fatal("expected invalid frontend base URL error")
	}
	if _, err := service.FrontendRedirectURL("/%", ""); err == nil {
		t.Fatal("expected invalid frontend path URL error")
	}
}

func TestFrontendOriginAndPathValidation(t *testing.T) {
	for name, testCase := range map[string]struct {
		base     string
		override string
		want     string
	}{
		"empty override":     {base: "https://app.example.com/base", want: "https://app.example.com/base"},
		"same origin":        {base: "https://APP.example.com/base?old=1#fragment", override: "https://app.example.com", want: "https://app.example.com/base"},
		"loopback alias":     {base: "http://localhost:3000/base", override: "http://127.0.0.1:3000", want: "http://127.0.0.1:3000/base"},
		"IPv6 loopback":      {base: "http://localhost:3000/base", override: "http://[::1]:3000", want: "http://[::1]:3000/base"},
		"different scheme":   {base: "http://localhost:3000", override: "https://127.0.0.1:3000", want: "http://localhost:3000"},
		"different port":     {base: "http://localhost:3000", override: "http://127.0.0.1:4000", want: "http://localhost:3000"},
		"non-loopback alias": {base: "https://app.example.com", override: "https://other.example.com", want: "https://app.example.com"},
		"invalid base":       {base: "%", override: "http://localhost", want: "%"},
		"invalid override":   {base: "https://app.example.com", override: "not-an-origin", want: "https://app.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := resolveFrontendBaseURL(testCase.base, testCase.override); got != testCase.want {
				t.Fatalf("resolve frontend base URL = %q, want %q", got, testCase.want)
			}
		})
	}

	for name, raw := range map[string]string{
		"missing scheme": "localhost:3000",
		"path":           "http://localhost:3000/path",
		"query":          "http://localhost:3000?x=1",
		"fragment":       "http://localhost:3000#fragment",
		"malformed":      "%",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseOriginURL(raw); err == nil {
				t.Fatalf("expected %q to be rejected", raw)
			}
		})
	}
	parsedOrigin, err := parseOriginURL(" HTTPS://LOCALHOST:443/ ")
	if err != nil || parsedOrigin.Path != "" || parsedOrigin.RawQuery != "" || parsedOrigin.Fragment != "" {
		t.Fatalf("valid origin = %#v, %v", parsedOrigin, err)
	}

	for name, testCase := range map[string]struct {
		value    string
		fallback string
		want     string
	}{
		"empty":             {value: "", fallback: "/fallback", want: "/fallback"},
		"relative":          {value: "dashboard", fallback: "/fallback", want: "/fallback"},
		"protocol relative": {value: "//evil.example.com", fallback: "/fallback", want: "/fallback"},
		"absolute":          {value: "https://evil.example.com", fallback: "/fallback", want: "/fallback"},
		"malformed":         {value: "/%", fallback: "/fallback", want: "/fallback"},
		"path query hash":   {value: "/dashboard?x=1#top", fallback: "/fallback", want: "/dashboard?x=1#top"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := normalizeFrontendPath(testCase.value, testCase.fallback); got != testCase.want {
				t.Fatalf("normalize frontend path = %q, want %q", got, testCase.want)
			}
		})
	}

	base, _ := url.Parse("https://example.com")
	other, _ := url.Parse("https://EXAMPLE.com:443")
	if !sameOrigin(base, other) || originPort(base) != "443" || originPort(&url.URL{Scheme: "http"}) != "80" || originPort(&url.URL{Scheme: "http", Host: "localhost:8080"}) != "8080" {
		t.Fatal("origin normalization or default ports are incorrect")
	}
	if !isLoopbackHostname("[::1]") || !isLoopbackHostname(" LOCALHOST ") || isLoopbackHostname("example.com") {
		t.Fatal("loopback hostname classification is incorrect")
	}
	if normalizeOriginHostname(" [ABCD::1] ") != "abcd::1" {
		t.Fatal("IPv6 hostname normalization failed")
	}
}

func TestOIDCHelpersContextAndActorMiddleware(t *testing.T) {
	if got := pkceChallengeS256(""); got != "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU" {
		t.Fatalf("unexpected empty PKCE challenge: %q", got)
	}
	if SessionCookieExpiry(time.Time{}) != 0 || SessionCookieExpiry(time.Now().Add(-time.Second)) != 0 || SessionCookieExpiry(time.Now().Add(2*time.Second)) <= 0 {
		t.Fatal("unexpected session cookie expiry")
	}
	ctx := context.Background()
	if got := oidcHTTPClientContext(ctx, nil); got != ctx {
		t.Fatal("nil HTTP client must preserve context")
	}
	if got := oidcHTTPClientContext(ctx, &http.Client{}); got == ctx {
		t.Fatal("custom HTTP client must derive a context")
	}

	user := UserProfile{ID: "user-id", Email: "user@example.com", Name: "User", PreferredUsername: "user", Type: UserTypeUser, Role: RoleAdmin}
	principal := Principal{User: user, CredentialID: "credential-id", CredentialName: "local"}
	principalCtx := ContextWithPrincipal(context.Background(), principal)
	if got, ok := PrincipalFromContext(principalCtx); !ok || got != principal {
		t.Fatalf("principal context = %#v, %v", got, ok)
	}
	if got, ok := UserFromContext(principalCtx); !ok || got != user {
		t.Fatalf("user context = %#v, %v", got, ok)
	}
	if _, ok := PrincipalFromContext(context.Background()); ok {
		t.Fatal("unexpected principal in empty context")
	}
	if _, ok := UserFromContext(context.Background()); ok {
		t.Fatal("unexpected user in empty context")
	}

	nextCalled := false
	handler := ActorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if got, ok := PrincipalFromContext(r.Context()); !ok || got.CredentialID != principal.CredentialID {
			t.Fatalf("principal not preserved: %#v, %v", got, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(principalCtx)
	handler.ServeHTTP(recorder, request)
	if !nextCalled || recorder.Code != http.StatusNoContent {
		t.Fatalf("actor middleware response = %d, called=%v", recorder.Code, nextCalled)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusInternalServerError || nextCalled == false && recorder.Body.Len() == 0 {
		t.Fatalf("missing-principal response = %d %q", recorder.Code, recorder.Body.String())
	}
	var payload map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil || payload["error"] != "missing_authenticated_user" || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected middleware error payload: %#v, %v", payload, err)
	}

	for _, role := range []AppRole{RoleNone, RoleLimited, RoleUser, RoleAdmin} {
		if !role.IsValid() {
			t.Fatalf("role %q should be valid", role)
		}
	}
	if AppRole("owner").IsValid() || !UserTypeUser.IsValid() || !UserTypeService.IsValid() || UserType("robot").IsValid() {
		t.Fatal("role or user-type validation mismatch")
	}
	claims := keycloakClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: "subject"}, PreferredUsername: "preferred", Email: "user@example.com", Name: "User"}
	if identity := claims.Identity(); identity.Sub != "subject" || identity.PreferredUsername != "preferred" || identity.Email != "user@example.com" || identity.Name != "User" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestResolveFrontendURL(t *testing.T) {
	got, err := resolveFrontendURL("https://app.example.com/base/", "../dashboard?tab=usage")
	if err != nil || got != "https://app.example.com/dashboard?tab=usage" {
		t.Fatalf("resolved URL = %q, %v", got, err)
	}
	for _, testCase := range []struct {
		base string
		path string
	}{
		{base: "%", path: "/dashboard"},
		{base: "https://app.example.com", path: "/%"},
	} {
		if _, err := resolveFrontendURL(testCase.base, testCase.path); err == nil || !strings.Contains(err.Error(), "parse frontend") {
			t.Fatalf("expected parsing error for %#v, got %v", testCase, err)
		}
	}
}
