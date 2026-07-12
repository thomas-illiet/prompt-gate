package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setRequiredAPIEnv sets required API env.
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

// setRequiredScheduleEnv sets required schedule env.
func setRequiredScheduleEnv(t *testing.T) {
	t.Helper()

	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable")
	t.Setenv("PROMPTGATE_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_SECRETS_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_REDIS_URL", "redis://localhost:6379/0")
}

// setRequiredWorkerEnv sets required worker env.
func setRequiredWorkerEnv(t *testing.T) {
	t.Helper()

	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable")
	t.Setenv("PROMPTGATE_REDIS_URL", "redis://localhost:6379/0")
}

// TestLoadApiDefaultSessionTTL verifies load API default session TTL.
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

// TestLoadApiCustomSessionTTL verifies load API custom session TTL.
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

// TestLoadApiAdminAPIKey verifies the optional administration API key is trimmed and accepts any non-empty length.
func TestLoadApiAdminAPIKey(t *testing.T) {
	for _, test := range []struct {
		name  string
		value *string
		want  string
	}{
		{name: "absent", want: ""},
		{name: "whitespace only", value: stringPointer(" \t\n "), want: ""},
		{name: "trimmed", value: stringPointer("  admin-secret\t"), want: "admin-secret"},
		{name: "single character", value: stringPointer("x"), want: "x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setRequiredAPIEnv(t)
			if test.value == nil {
				unsetEnv(t, "PROMPTGATE_ADMIN_API_KEY")
			} else {
				t.Setenv("PROMPTGATE_ADMIN_API_KEY", *test.value)
			}

			cfg, err := LoadApi()
			if err != nil {
				t.Fatalf("load api config: %v", err)
			}
			if cfg.AdminAPIKey != test.want {
				t.Fatalf("expected admin API key %q, got %q", test.want, cfg.AdminAPIKey)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	previous, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	})
}

// TestLoadApiProxyBaseURLOverride verifies load API proxy base URL override.
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

// TestLoadApiProxyBaseURLFallback verifies load API proxy base URL fallback.
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

// TestLoadApiUsageCostDefaults verifies load API usage cost defaults.
func TestLoadApiUsageCostDefaults(t *testing.T) {
	setRequiredAPIEnv(t)

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if !cfg.UsageCost.Enabled {
		t.Fatal("expected usage cost estimates to be enabled by default")
	}
	if cfg.UsageCost.Input != 5 || cfg.UsageCost.Output != 30 || cfg.UsageCost.Embedding != 0.02 {
		t.Fatalf("unexpected usage cost defaults: %#v", cfg.UsageCost)
	}
}

// TestLoadApiUsageCostOverrideAndDisable verifies load API usage cost override and disable.
func TestLoadApiUsageCostOverrideAndDisable(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_USAGE_COST_ENABLED", "false")
	t.Setenv("PROMPTGATE_USAGE_COST_INPUT", "1.25")
	t.Setenv("PROMPTGATE_USAGE_COST_OUTPUT", "2.5")
	t.Setenv("PROMPTGATE_USAGE_COST_EMBEDDING", "0.13")

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.UsageCost.Enabled {
		t.Fatal("expected usage cost estimates to be disabled")
	}
	if cfg.UsageCost.Input != 1.25 || cfg.UsageCost.Output != 2.5 || cfg.UsageCost.Embedding != 0.13 {
		t.Fatalf("unexpected usage cost overrides: %#v", cfg.UsageCost)
	}
}

// TestLoadApiRejectsInvalidUsageCostRates verifies load API rejects invalid usage cost rates.
func TestLoadApiRejectsInvalidUsageCostRates(t *testing.T) {
	for _, test := range []struct {
		name  string
		env   string
		value string
		want  string
	}{
		{name: "input non numeric", env: "PROMPTGATE_USAGE_COST_INPUT", value: "not-a-number", want: "PROMPTGATE_USAGE_COST_INPUT"},
		{name: "output negative", env: "PROMPTGATE_USAGE_COST_OUTPUT", value: "-1", want: "PROMPTGATE_USAGE_COST_OUTPUT"},
		{name: "embedding nan", env: "PROMPTGATE_USAGE_COST_EMBEDDING", value: "NaN", want: "PROMPTGATE_USAGE_COST_EMBEDDING"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setRequiredAPIEnv(t)
			t.Setenv(test.env, test.value)

			_, err := LoadApi()
			if err == nil {
				t.Fatal("expected invalid usage cost rate to fail")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error mentioning %s, got %v", test.want, err)
			}
		})
	}
}

// TestLoadApiReadsStaticAssetsDir verifies load API reads static assets dir.
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

// TestLoadApiReadsCAFile verifies load API reads the global CA file.
func TestLoadApiReadsCAFile(t *testing.T) {
	setRequiredAPIEnv(t)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("not validated here"), 0o600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}
	t.Setenv("PROMPTGATE_CA_FILE", caFile)

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.CAFile != caFile {
		t.Fatalf("expected CA file %q, got %q", caFile, cfg.CAFile)
	}
}

// TestLoadApiRejectsInvalidCAFile verifies load API rejects invalid CA file.
func TestLoadApiRejectsInvalidCAFile(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_CA_FILE", "/definitely/not/a/ca.pem")

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected invalid CA file to fail")
	}
}

// TestLoadApiRejectsCAFileDirectory verifies load API rejects CA file directories.
func TestLoadApiRejectsCAFileDirectory(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_CA_FILE", t.TempDir())

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected CA file directory to fail")
	}
}

// TestLoadApiIgnoresLegacyKeycloakCAEnv verifies the old Keycloak CA env var is not a fallback.
func TestLoadApiIgnoresLegacyKeycloakCAEnv(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_KEYCLOAK_CA_CERT_PATH", "/definitely/not/a/keycloak-ca.pem")

	cfg, err := LoadApi()
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.CAFile != "" {
		t.Fatalf("expected legacy Keycloak CA env to be ignored, got %q", cfg.CAFile)
	}
}

// TestLoadApiRejectsInvalidStaticAssetsDir verifies load API rejects invalid static assets dir.
func TestLoadApiRejectsInvalidStaticAssetsDir(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_STATIC_ASSETS_DIR", "/definitely/not/a/static/assets/dir")

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected invalid static assets dir to fail")
	}
}

// TestLoadApiRejectsNonPositiveSessionTTL verifies load API rejects non-positive session TTL.
func TestLoadApiRejectsNonPositiveSessionTTL(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_SESSION_TTL", "0")

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected non-positive session ttl to fail")
	}
}

// TestLoadApiRequiresRedisURL verifies load API requires Redis URL.
func TestLoadApiRequiresRedisURL(t *testing.T) {
	setRequiredAPIEnv(t)
	t.Setenv("PROMPTGATE_REDIS_URL", "")

	if _, err := LoadApi(); err == nil {
		t.Fatal("expected missing redis url to fail")
	}
}

// TestLoadScheduleReadsCAFile verifies load schedule reads the global CA file.
func TestLoadScheduleReadsCAFile(t *testing.T) {
	setRequiredScheduleEnv(t)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("not validated here"), 0o600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}
	t.Setenv("PROMPTGATE_CA_FILE", caFile)

	cfg, err := LoadSchedule()
	if err != nil {
		t.Fatalf("load schedule config: %v", err)
	}
	if cfg.CAFile != caFile {
		t.Fatalf("expected CA file %q, got %q", caFile, cfg.CAFile)
	}
}

// TestLoadScheduleUsageRawCleanupDefaults verifies load schedule raw usage cleanup defaults.
func TestLoadScheduleUsageRawCleanupDefaults(t *testing.T) {
	setRequiredScheduleEnv(t)

	cfg, err := LoadSchedule()
	if err != nil {
		t.Fatalf("load schedule config: %v", err)
	}
	if cfg.UsageRawRetention != 90*24*time.Hour {
		t.Fatalf("expected raw usage retention 2160h, got %s", cfg.UsageRawRetention)
	}
	if cfg.UsageRawCleanupInterval != time.Hour {
		t.Fatalf("expected raw usage cleanup interval 1h, got %s", cfg.UsageRawCleanupInterval)
	}
}

// TestLoadScheduleRejectsInvalidCAFile verifies load schedule rejects invalid CA file.
func TestLoadScheduleRejectsInvalidCAFile(t *testing.T) {
	setRequiredScheduleEnv(t)
	t.Setenv("PROMPTGATE_CA_FILE", "/definitely/not/a/ca.pem")

	if _, err := LoadSchedule(); err == nil {
		t.Fatal("expected invalid CA file to fail")
	}
}

// TestLoadScheduleRejectsCAFileDirectory verifies load schedule rejects CA file directories.
func TestLoadScheduleRejectsCAFileDirectory(t *testing.T) {
	setRequiredScheduleEnv(t)
	t.Setenv("PROMPTGATE_CA_FILE", t.TempDir())

	if _, err := LoadSchedule(); err == nil {
		t.Fatal("expected CA file directory to fail")
	}
}

// TestLoadWorkerDefaultsAndOverrides verifies worker-specific config values.
func TestLoadWorkerDefaultsAndOverrides(t *testing.T) {
	setRequiredWorkerEnv(t)
	t.Setenv("PROMPTGATE_WORKER_BATCH_SIZE", "42")
	t.Setenv("PROMPTGATE_WORKER_BLOCK_TIMEOUT", "2s")
	t.Setenv("PROMPTGATE_WORKER_PENDING_IDLE_TIMEOUT", "3s")
	t.Setenv("PROMPTGATE_WORKER_CONSUMER_NAME", "worker-a")

	cfg, err := LoadWorker()
	if err != nil {
		t.Fatalf("load worker config: %v", err)
	}
	if cfg.WorkerBatchSize != 42 || cfg.WorkerBlockTimeout != 2*time.Second || cfg.WorkerPendingIdleTimeout != 3*time.Second || cfg.WorkerConsumerName != "worker-a" {
		t.Fatalf("unexpected worker config: %#v", cfg)
	}
}

// TestLoadWorkerRequiresRedisURL verifies worker startup requires Redis.
func TestLoadWorkerRequiresRedisURL(t *testing.T) {
	setRequiredWorkerEnv(t)
	t.Setenv("PROMPTGATE_REDIS_URL", "")

	if _, err := LoadWorker(); err == nil {
		t.Fatal("expected missing redis url to fail")
	}
}

// TestLoadProxyReadsSessionAndCORSConfig verifies load proxy reads session and CORS config.
func TestLoadProxyReadsSessionAndCORSConfig(t *testing.T) {
	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable")
	t.Setenv("PROMPTGATE_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_SECRETS_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("PROMPTGATE_FRONTEND_BASE_URL", "http://localhost:3000")
	t.Setenv("PROMPTGATE_SESSION_COOKIE_NAME", "custom_session")
	t.Setenv("PROMPTGATE_SESSION_TTL", "12h")
	t.Setenv("PROMPTGATE_PROXY_MAX_BUFFERED_REQUEST_BYTES", "1024")
	t.Setenv("PROMPTGATE_PROXY_MAX_BUFFERED_RESPONSE_BYTES", "2048")
	t.Setenv("PROMPTGATE_PROXY_UPSTREAM_TIMEOUT", "3m")

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
	if cfg.ProxyMaxBufferedRequestBytes != 1024 {
		t.Fatalf("unexpected request buffer limit: %d", cfg.ProxyMaxBufferedRequestBytes)
	}
	if cfg.ProxyMaxBufferedResponseBytes != 2048 {
		t.Fatalf("unexpected response buffer limit: %d", cfg.ProxyMaxBufferedResponseBytes)
	}
	if cfg.ProxyUpstreamTimeout != 3*time.Minute {
		t.Fatalf("expected custom upstream timeout 3m, got %s", cfg.ProxyUpstreamTimeout)
	}
}

// TestLoadProxyReadsTrustedProxies verifies load proxy reads explicit trusted proxy CIDRs.
func TestLoadProxyReadsTrustedProxies(t *testing.T) {
	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable")
	t.Setenv("PROMPTGATE_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_SECRETS_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("PROMPTGATE_PROXY_TRUSTED_PROXIES", "10.0.0.0/8, 192.168.0.0/16")

	cfg, err := LoadProxy()
	if err != nil {
		t.Fatalf("load proxy config: %v", err)
	}
	if len(cfg.ProxyTrustedProxies) != 2 {
		t.Fatalf("expected two trusted proxies, got %#v", cfg.ProxyTrustedProxies)
	}
	if cfg.ProxyTrustedProxies[0].String() != "10.0.0.0/8" || cfg.ProxyTrustedProxies[1].String() != "192.168.0.0/16" {
		t.Fatalf("unexpected trusted proxies: %#v", cfg.ProxyTrustedProxies)
	}
}

// TestLoadProxyRejectsInvalidTrustedProxyCIDR verifies invalid CIDRs fail startup validation.
func TestLoadProxyRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable")
	t.Setenv("PROMPTGATE_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_SECRETS_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PROMPTGATE_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("PROMPTGATE_PROXY_TRUSTED_PROXIES", "10.0.0.0/8,not-a-cidr")

	_, err := LoadProxy()
	if err == nil {
		t.Fatal("expected invalid trusted proxy CIDR to fail")
	}
	if !strings.Contains(err.Error(), "PROMPTGATE_PROXY_TRUSTED_PROXIES") {
		t.Fatalf("expected trusted proxies error, got %v", err)
	}
}
