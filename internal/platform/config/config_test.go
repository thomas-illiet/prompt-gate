package config

import (
	"testing"
	"time"
)

func setRequiredAPIEnv(t *testing.T) {
	t.Helper()

	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable")
	t.Setenv("PROMPTGATE_KEYCLOAK_ISSUER_URL", "https://keycloak.example.com/realms/promptgate")
	t.Setenv("PROMPTGATE_KEYCLOAK_JWKS_URL", "https://keycloak.example.com/realms/promptgate/protocol/openid-connect/certs")
	t.Setenv("PROMPTGATE_KEYCLOAK_CLIENT_ID", "promptgate-backend")
	t.Setenv("PROMPTGATE_FRONTEND_BASE_URL", "http://localhost:3000")
	t.Setenv("PROMPTGATE_BACKEND_BASE_URL", "http://localhost:8080")
	t.Setenv("PROMPTGATE_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_SECRETS_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_REDIS_URL", "redis://localhost:6379/0")
}

func TestLoadApiDefaultSessionTTL(t *testing.T) {
	setRequiredAPIEnv(t)

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.SessionTTL != 8*time.Hour {
		t.Fatalf("expected default session ttl 8h, got %s", cfg.SessionTTL)
	}
}

func TestLoadApiCustomSessionTTL(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_SESSION_TTL", "12h")

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Fatalf("expected custom session ttl 12h, got %s", cfg.SessionTTL)
	}
}

func TestLoadApiProxyBaseURLOverride(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_PROXY_BASE_URL", "https://proxy.example.com/promptgate/")

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.ProxyBaseURL != "https://proxy.example.com/promptgate" {
		t.Fatalf("expected proxy base url override, got %q", cfg.ProxyBaseURL)
	}
}

func TestLoadApiProxyBaseURLFallback(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_BACKEND_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("PROMPTGATE_PROXY_PORT", "9090")

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.ProxyBaseURL != "http://127.0.0.1:9090" {
		t.Fatalf("expected proxy fallback url, got %q", cfg.ProxyBaseURL)
	}
}

func TestLoadApiReadsStaticAssetsDir(t *testing.T) {
	setRequiredAPIEnv(t)
	staticDir := t.TempDir()
	t.Setenv("PROMPTGATE_STATIC_ASSETS_DIR", staticDir)

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.StaticAssetsDir != staticDir {
		t.Fatalf("expected static assets dir %q, got %q", staticDir, cfg.StaticAssetsDir)
	}
}

func TestLoadApiRejectsInvalidStaticAssetsDir(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_STATIC_ASSETS_DIR", "/definitely/not/a/static/assets/dir")

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected invalid static assets dir to fail")
	}
}

func TestLoadApiRejectsNonPositiveSessionTTL(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_SESSION_TTL", "0")

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected non-positive session ttl to fail")
	}
}

func TestLoadApiRequiresRedisURL(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_REDIS_URL", "")

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected missing redis url to fail")
	}
}

func TestLoadProxyReadsSessionAndCORSConfig(t *testing.T) {
	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable")
	t.Setenv("PROMPTGATE_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_SECRETS_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("PROMPTGATE_FRONTEND_BASE_URL", "http://localhost:3000")
	t.Setenv("PROMPTGATE_SESSION_COOKIE_NAME", "custom_session")
	t.Setenv("PROMPTGATE_SESSION_TTL", "12h")

	cfg, err := LoadProxy()
	if err != nil {
		t.Fatalf("load proxy config: %v", err)
	}
	if cfg.SessionCookieName != "custom_session" {
		t.Fatalf("unexpected cookie name: %q", cfg.SessionCookieName)
	}
	if cfg.SessionTTL != 12*time.Hour {
		t.Fatalf("expected custom session ttl 12h, got %s", cfg.SessionTTL)
	}
	if len(cfg.CORSAllowedOrigins) == 0 || cfg.CORSAllowedOrigins[0] != "http://localhost:3000" {
		t.Fatalf("unexpected cors origins: %#v", cfg.CORSAllowedOrigins)
	}
}
