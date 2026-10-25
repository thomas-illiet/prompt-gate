package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const validConfigYAML = `
common:
  database:
    url: postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable
  redis:
    url: redis://localhost:6379/0
  security:
    jwt_secret: 0123456789abcdef0123456789abcdef
    secrets_key: MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=
  public_urls:
    frontend: http://localhost:3000
    backend: http://localhost:8080
    proxy: https://proxy.example.com
api:
  oidc:
    issuer_url: https://keycloak.example.com/realms/promptgate
    jwks_url: https://keycloak.example.com/realms/promptgate/protocol/openid-connect/certs
    client_id: promptgate-backend
proxy:
  opa:
    url: http://opa:8181
`

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "promptgate.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadRuntimeConfigsFromYAML(t *testing.T) {
	path := writeConfig(t, validConfigYAML)
	api, err := LoadApi(path)
	if err != nil {
		t.Fatalf("load api: %v", err)
	}
	proxy, err := LoadProxy(path)
	if err != nil {
		t.Fatalf("load proxy: %v", err)
	}
	worker, err := LoadWorker(path)
	if err != nil {
		t.Fatalf("load worker: %v", err)
	}
	schedule, err := LoadSchedule(path)
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	migration, err := LoadMigration(path)
	if err != nil {
		t.Fatalf("load migration: %v", err)
	}

	if api.Port != "8080" || proxy.Port != "8081" {
		t.Fatalf("unexpected ports: api=%q proxy=%q", api.Port, proxy.Port)
	}
	if api.SessionTTL != 8*time.Hour || api.ModelLab.TicketTTL != time.Minute {
		t.Fatalf("unexpected API defaults: %#v", api)
	}
	if proxy.UsageCost.Input != 5 || proxy.UsageCost.Output != 30 || proxy.UsageCost.Embedding != 0.02 {
		t.Fatalf("unexpected usage cost defaults: %#v", proxy.UsageCost)
	}
	if worker.WorkerBatchSize != 100 || schedule.UsageRawRetention != 90*24*time.Hour {
		t.Fatalf("unexpected worker/schedule defaults: worker=%#v schedule=%#v", worker, schedule)
	}
	if migration.DatabaseURL != api.DatabaseURL {
		t.Fatalf("expected common database URL, got %q", migration.DatabaseURL)
	}
	if api.ConnMaxLifetime != 30*time.Minute || api.MaxIdleConnections != 5 || api.MaxOpenConnections != 10 {
		t.Fatalf("unexpected database pool defaults: %#v", api.DatabaseURLConfig)
	}
}

func TestShippedConfigsLoadForEveryRuntime(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "..", "promptgate.example.yaml"),
		filepath.Join("..", "..", "..", "deploy", "config", "promptgate.compose.yaml"),
		filepath.Join("..", "..", "..", "testdata", "promptgate.integration.yaml"),
	}
	loads := []struct {
		name string
		load func(string) error
	}{
		{"api", func(path string) error { _, err := LoadApi(path); return err }},
		{"proxy", func(path string) error { _, err := LoadProxy(path); return err }},
		{"worker", func(path string) error { _, err := LoadWorker(path); return err }},
		{"schedule", func(path string) error { _, err := LoadSchedule(path); return err }},
		{"migrate", func(path string) error { _, err := LoadMigration(path); return err }},
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			loadPath := path
			if filepath.Base(path) == "promptgate.compose.yaml" {
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				contents = []byte(strings.Replace(string(contents), "/app/public", strconv.Quote(t.TempDir()), 1))
				loadPath = writeConfig(t, string(contents))
			}
			for _, runtime := range loads {
				t.Run(runtime.name, func(t *testing.T) {
					if err := runtime.load(loadPath); err != nil {
						t.Fatalf("load %s for %s: %v", path, runtime.name, err)
					}
				})
			}
		})
	}
}

func TestLoadYAMLOverridesAndLists(t *testing.T) {
	contents := strings.Replace(validConfigYAML, `  database:
    url: postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable
`, `  database:
    url: postgres://custom:custom@db.example.com:5432/custom
    connection_max_lifetime: 45m
    max_idle_connections: 7
    max_open_connections: 12
`, 1)
	contents = strings.Replace(contents, `  redis:
    url: redis://localhost:6379/0
`, `  redis:
    url: redis://localhost:6379/0
    cache_ttl: 7m
    pool_size: 20
    min_idle_connections: 2
    pool_timeout: 4s
    read_timeout: 3s
    write_timeout: 2s
`, 1)
	contents = strings.Replace(contents, "  security:\n", `  cors:
    allowed_origins:
      - https://app.example.com
  session:
    cookie_name: custom_session
    ttl: 9h
  model_lab:
    ticket_ttl: 90s
    max_prompt_bytes: 65536
    max_output_tokens: 2048
    max_targets: 2
    max_concurrent: 2
    catalog_fresh_ttl: 10m
    catalog_stale_ttl: 25m
  usage_cost:
    input: 1.25
    output: 2.5
    embedding: 0.13
  maintenance:
    token_cleanup_interval: 2h
    user_access_expiration_interval: 3h
    quota_reset_distribution_interval: 4h
  proxy_reload_debounce: 500ms
  security:
`, 1)
	contents = strings.Replace(contents, `api:
  oidc:
`, `api:
  port: 9080
  admin_api_key: admin-test-key
  oidc:
`, 1)
	contents = strings.Replace(contents, `    client_id: promptgate-backend
`, `    client_id: promptgate-backend
    client_secret: oidc-test-secret
`, 1)
	contents = strings.Replace(contents, `proxy:
  opa:
    url: http://opa:8181
`, `proxy:
  port: ":9090"
  trusted_proxies:
    - 10.0.0.0/8
    - 192.168.0.0/16
  opa:
    url: http://opa:8181
    timeout: 4s
    cache_ttl: 11m
  otel:
    enabled: true
    endpoint: http://otel.example.com:4318
    api_key: otel-test-key
    project_name: custom-project
    service_name: custom-service
    environment: test
    capture_prompts: true
    capture_output: true
    capture_thinking: true
    export_timeout: 7s
    batch_timeout: 3s
    max_queue_size: 64
    max_export_batch_size: 32
    insecure: true
`, 1)
	contents += `
worker:
  batch_size: 42
  block_timeout: 2s
  pending_idle_timeout: 3s
  consumer_name: worker-a
schedule:
  usage_raw_retention: 720h
  usage_raw_cleanup_interval: 2h
  subscription_quota_sync_interval: 7m
`
	path := writeConfig(t, contents)

	api, err := LoadApi(path)
	if err != nil {
		t.Fatalf("load api: %v", err)
	}
	proxy, err := LoadProxy(path)
	if err != nil {
		t.Fatalf("load proxy: %v", err)
	}
	worker, err := LoadWorker(path)
	if err != nil {
		t.Fatalf("load worker: %v", err)
	}
	schedule, err := LoadSchedule(path)
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	migration, err := LoadMigration(path)
	if err != nil {
		t.Fatalf("load migration: %v", err)
	}
	if api.Port != "9080" || api.AdminAPIKey != "admin-test-key" || api.KeycloakClientSecret != "oidc-test-secret" {
		t.Fatalf("unexpected API overrides: %#v", api)
	}
	if api.SessionCookieName != "custom_session" || api.SessionTTL != 9*time.Hour || api.TokenCleanupInterval != 2*time.Hour || api.UserAccessExpirationInterval != 3*time.Hour || api.QuotaResetDistributionInterval != 4*time.Hour {
		t.Fatalf("unexpected common API overrides: %#v", api)
	}
	if api.ModelLab.TicketTTL != 90*time.Second || api.ModelLab.MaxTargets != 2 || api.ModelLab.CatalogStaleTTL != 25*time.Minute {
		t.Fatalf("unexpected model lab overrides: %#v", api.ModelLab)
	}
	if proxy.Port != ":9090" || len(proxy.ProxyTrustedProxies) != 2 {
		t.Fatalf("unexpected proxy overrides: %#v", proxy)
	}
	if proxy.RedisCacheTTL != 7*time.Minute || proxy.RedisPoolSize != 20 || proxy.RedisMinIdleConnections != 2 || proxy.RedisPoolTimeout != 4*time.Second || proxy.RedisReadTimeout != 3*time.Second || proxy.RedisWriteTimeout != 2*time.Second {
		t.Fatalf("unexpected Redis overrides: %#v", proxy.RedisConfig)
	}
	if proxy.ConnMaxLifetime != 45*time.Minute || proxy.MaxIdleConnections != 7 || proxy.MaxOpenConnections != 12 {
		t.Fatalf("unexpected database pool overrides: %#v", proxy.DatabaseURLConfig)
	}
	if proxy.OPATimeout != 4*time.Second || proxy.OPACacheTTL != 11*time.Minute || !proxy.OTel.Enabled || !proxy.OTel.Insecure || proxy.OTel.MaxQueueSize != 64 || proxy.OTel.MaxExportBatchSize != 32 {
		t.Fatalf("unexpected proxy integration overrides: %#v", proxy)
	}
	if proxy.UsageCost.Input != 1.25 || proxy.UsageCost.Output != 2.5 || proxy.UsageCost.Embedding != 0.13 {
		t.Fatalf("unexpected costs: %#v", proxy.UsageCost)
	}
	if worker.WorkerBatchSize != 42 || worker.WorkerBlockTimeout != 2*time.Second || worker.WorkerPendingIdleTimeout != 3*time.Second || worker.WorkerConsumerName != "worker-a" {
		t.Fatalf("unexpected worker overrides: %#v", worker)
	}
	if schedule.UsageRawRetention != 720*time.Hour || schedule.UsageRawCleanupInterval != 2*time.Hour || schedule.SubscriptionQuotaSyncInterval != 7*time.Minute {
		t.Fatalf("unexpected schedule overrides: %#v", schedule)
	}
	if migration.DatabaseURL != "postgres://custom:custom@db.example.com:5432/custom" || migration.ConnMaxLifetime != 45*time.Minute || migration.MaxIdleConnections != 7 || migration.MaxOpenConnections != 12 {
		t.Fatalf("unexpected migration overrides: %#v", migration)
	}
}

func TestLoadConfigStrictYAML(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unknown key", content: validConfigYAML + "unknown: true\n", want: "field unknown not found"},
		{name: "unknown usage enabled", content: "common:\n  usage_cost:\n    enabled: true\n", want: "field enabled not found"},
		{name: "duplicate key", content: "common:\n  log:\n    level: info\n    level: debug\n", want: "already defined"},
		{name: "multiple documents", content: validConfigYAML + "\n---\ncommon: {}\n", want: "exactly one YAML document"},
		{name: "wrong integer type", content: "worker:\n  batch_size: nope\n", want: "worker.batch_size must be an integer"},
		{name: "wrong string type", content: "common:\n  log:\n    level: false\n", want: "common.log.level must be a string"},
		{name: "wrong boolean type", content: "proxy:\n  opa:\n    enabled: \"false\"\n", want: "proxy.opa.enabled must be a boolean"},
		{name: "wrong duration type", content: "common:\n  session:\n    ttl: 8\n", want: "common.session.ttl must be a duration string"},
		{name: "wrong number type", content: "common:\n  usage_cost:\n    input: \"5.0\"\n", want: "common.usage_cost.input must be a number"},
		{name: "null defaulted integer", content: "common:\n  database:\n    max_open_connections: null\n", want: "common.database.max_open_connections must not be null"},
		{name: "null defaulted boolean", content: "proxy:\n  opa:\n    enabled: null\n", want: "proxy.opa.enabled must not be null"},
		{name: "null defaulted duration", content: "common:\n  session:\n    ttl: null\n", want: "common.session.ttl must not be null"},
		{name: "OTel CA removed", content: "proxy:\n  otel:\n    ca_file: /tmp/ca.pem\n", want: "field ca_file not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadApi(writeConfig(t, test.content))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLoadConfigRequiresReadableSingleFile(t *testing.T) {
	if _, err := LoadMigration(""); err == nil || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("expected missing config error, got %v", err)
	}
	if _, err := LoadMigration(filepath.Join(t.TempDir(), "missing.yaml")); err == nil || !strings.Contains(err.Error(), "open config") {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if _, err := LoadMigration(writeConfig(t, "")); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty config error, got %v", err)
	}
}

func TestLoadConfigResolvesRelativePaths(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	assetsPath := filepath.Join(dir, "public")
	if err := os.WriteFile(caPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	if err := os.Mkdir(assetsPath, 0o700); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	content := strings.Replace(validConfigYAML, "  security:\n", "  tls:\n    ca_file: ca.pem\n  security:\n", 1)
	content = strings.Replace(content, "api:\n  oidc:\n", "api:\n  static_assets_dir: public\n  oidc:\n", 1)
	path := filepath.Join(dir, "promptgate.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadApi(path)
	if err != nil {
		t.Fatalf("load api: %v", err)
	}
	if cfg.CAFile != caPath || cfg.StaticAssetsDir != assetsPath {
		t.Fatalf("relative paths not resolved: ca=%q assets=%q", cfg.CAFile, cfg.StaticAssetsDir)
	}
}

func TestLegacyEnvironmentDoesNotOverrideYAML(t *testing.T) {
	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://legacy.invalid/ignored")
	t.Setenv("PROMPTGATE_WORKER_BATCH_SIZE", "999")
	cfg, err := LoadWorker(writeConfig(t, validConfigYAML))
	if err != nil {
		t.Fatalf("load worker: %v", err)
	}
	if strings.Contains(cfg.DatabaseURL, "legacy.invalid") || cfg.WorkerBatchSize != 100 {
		t.Fatalf("legacy environment affected YAML config: %#v", cfg)
	}
}

func TestMigrationOnlyValidatesConsumedCommonConfig(t *testing.T) {
	path := writeConfig(t, `
common:
  database:
    url: postgres://postgres:postgres@localhost:5432/promptgate
`)
	if _, err := LoadMigration(path); err != nil {
		t.Fatalf("load migration: %v", err)
	}
	if _, err := LoadWorker(path); err == nil || !strings.Contains(err.Error(), "common.redis.url") {
		t.Fatalf("expected worker Redis requirement, got %v", err)
	}
}

func TestLoadProxyValidatesCostsCIDRsAndOTel(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "negative cost", content: strings.Replace(validConfigYAML, "  security:\n", "  usage_cost:\n    input: -1\n  security:\n", 1), want: "common.usage_cost.input"},
		{name: "zero Redis minimum idle", content: strings.Replace(validConfigYAML, "    url: redis://localhost:6379/0\n", "    url: redis://localhost:6379/0\n    min_idle_connections: 0\n", 1), want: "common.redis.min_idle_connections"},
		{name: "invalid CIDR", content: strings.Replace(validConfigYAML, "proxy:\n", "proxy:\n  trusted_proxies: [10.0.0.0/8, not-a-cidr]\n", 1), want: "proxy.trusted_proxies"},
		{name: "invalid model lab limit", content: strings.Replace(validConfigYAML, "  security:\n", "  model_lab:\n    max_targets: 4\n  security:\n", 1), want: "common.model_lab.max_targets"},
		{name: "stale catalog shorter than fresh", content: strings.Replace(validConfigYAML, "  security:\n", "  model_lab:\n    catalog_fresh_ttl: 10m\n    catalog_stale_ttl: 5m\n  security:\n", 1), want: "common.model_lab.catalog_stale_ttl"},
		{name: "invalid OPA URL", content: strings.Replace(validConfigYAML, "url: http://opa:8181", "url: file:///tmp/policy", 1), want: "proxy.opa.url"},
		{name: "empty OPA policy path", content: strings.Replace(validConfigYAML, "    url: http://opa:8181\n", "    url: http://opa:8181\n    policy_path: \"\"\n", 1), want: "proxy.opa.policy_path"},
		{name: "zero OPA timeout", content: strings.Replace(validConfigYAML, "    url: http://opa:8181\n", "    url: http://opa:8181\n    timeout: 0s\n", 1), want: "proxy.opa.timeout"},
		{name: "OTel missing endpoint", content: strings.Replace(validConfigYAML, "proxy:\n", "proxy:\n  otel:\n    enabled: true\n", 1), want: "proxy.otel.endpoint"},
		{name: "OTel HTTP requires insecure", content: strings.Replace(validConfigYAML, "proxy:\n", "proxy:\n  otel:\n    enabled: true\n    endpoint: http://otel.example.com:4318\n", 1), want: "proxy.otel.endpoint must use https"},
		{name: "OTel batch exceeds queue", content: strings.Replace(validConfigYAML, "proxy:\n", "proxy:\n  otel:\n    enabled: true\n    endpoint: https://otel.example.com:4318\n    max_queue_size: 10\n    max_export_batch_size: 11\n", 1), want: "proxy.otel.max_export_batch_size"},
		{name: "OTel zero export timeout", content: strings.Replace(validConfigYAML, "proxy:\n", "proxy:\n  otel:\n    enabled: true\n    endpoint: https://otel.example.com:4318\n    export_timeout: 0s\n", 1), want: "proxy.otel.export_timeout"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadProxy(writeConfig(t, test.content))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLoadProxyAllowsDisabledOPAWithoutURL(t *testing.T) {
	content := strings.Replace(validConfigYAML, `proxy:
  opa:
    url: http://opa:8181
`, `proxy:
  opa:
    enabled: false
    url: ""
`, 1)
	cfg, err := LoadProxy(writeConfig(t, content))
	if err != nil {
		t.Fatalf("load proxy with OPA disabled: %v", err)
	}
	if cfg.OPAEnabled {
		t.Fatal("expected OPA to remain disabled")
	}
}

func TestRuntimeValidationErrorsUseYAMLPaths(t *testing.T) {
	tests := []struct {
		name    string
		content string
		load    func(string) error
		want    string
	}{
		{
			name:    "API maintenance duration",
			content: strings.Replace(validConfigYAML, "  security:\n", "  maintenance:\n    token_cleanup_interval: 0s\n  security:\n", 1),
			load:    func(path string) error { _, err := LoadApi(path); return err },
			want:    "common.maintenance.token_cleanup_interval",
		},
		{
			name:    "proxy upstream duration",
			content: strings.Replace(validConfigYAML, "proxy:\n", "proxy:\n  upstream_timeout: 0s\n", 1),
			load:    func(path string) error { _, err := LoadProxy(path); return err },
			want:    "proxy.upstream_timeout",
		},
		{
			name:    "worker block duration",
			content: validConfigYAML + "worker:\n  block_timeout: 0s\n",
			load:    func(path string) error { _, err := LoadWorker(path); return err },
			want:    "worker.block_timeout",
		},
		{
			name:    "schedule retention duration",
			content: validConfigYAML + "schedule:\n  usage_raw_retention: 0s\n",
			load:    func(path string) error { _, err := LoadSchedule(path); return err },
			want:    "schedule.usage_raw_retention",
		},
		{
			name:    "migration database pool",
			content: strings.Replace(validConfigYAML, "    url: postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable\n", "    url: postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable\n    max_idle_connections: 11\n    max_open_connections: 10\n", 1),
			load:    func(path string) error { _, err := LoadMigration(path); return err },
			want:    "common.database.max_idle_connections",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.load(writeConfig(t, test.content))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestPublicURLHelpers(t *testing.T) {
	urls := PublicURLConfig{BackendBaseURL: "https://api.example.com"}
	if got := urls.OIDCCallbackURL(); got != "https://api.example.com/auth/callback" {
		t.Fatalf("unexpected callback URL %q", got)
	}
	if !urls.SessionCookieSecure() {
		t.Fatal("expected HTTPS cookie to be secure")
	}
	if got := deriveProxyBaseURL("http://127.0.0.1:8080", "9090"); got != "http://127.0.0.1:9090" {
		t.Fatalf("unexpected derived proxy URL %q", got)
	}
}
