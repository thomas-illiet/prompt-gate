package config

import (
	"strings"
	"testing"
	"time"
)

func TestIPValidationConfigurationContract(t *testing.T) {
	cfg, err := LoadProxy(writeConfig(t, validConfigYAML))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IPValidationEnabled || cfg.IPValidationTimeout != 2*time.Second || cfg.IPValidationCacheTTL != 10*time.Minute || cfg.IPValidationBearerToken != "test-service-token" {
		t.Fatal("incorrect validation defaults")
	}
	for _, tc := range []struct{ name, old, replacement, want string }{
		{"missing placeholder", "https://validation.example/ips/{ip}", "https://validation.example/ips/static", "url_template"},
		{"query placeholder", "https://validation.example/ips/{ip}", "https://validation.example/ips?ip={ip}", "url_template"},
		{"duplicate placeholder", "https://validation.example/ips/{ip}", "https://validation.example/{ip}/{ip}", "url_template"},
		{"unknown placeholder", "https://validation.example/ips/{ip}", "https://validation.example/{account}/{ip}", "url_template"},
		{"credentials", "https://validation.example/ips/{ip}", "https://user:secret@validation.example/ips/{ip}", "url_template"},
		{"fragment", "https://validation.example/ips/{ip}", "https://validation.example/ips/{ip}#fragment", "url_template"},
		{"invalid token", "test-service-token", "'Bearer service-token'", "bearer_token"},
		{"missing token", "test-service-token", "''", "bearer_token"},
		{"zero ttl", "  ip_validation:", "  ip_validation:\n    cache_ttl: 0s", "cache_ttl"},
		{"negative timeout", "  ip_validation:", "  ip_validation:\n    timeout: -1s", "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadProxy(writeConfig(t, strings.Replace(validConfigYAML, tc.old, tc.replacement, 1)))
			if err == nil || !strings.Contains(err.Error(), "proxy.ip_validation."+tc.want) {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if strings.Contains(err.Error(), "user:secret") {
				t.Fatal("configuration secret leaked")
			}
		})
	}
}

func TestLegacyPolicyConfigurationRejectedByEveryRuntime(t *testing.T) {
	content := strings.Replace(validConfigYAML, "  ip_validation:", "  opa:\n    enabled: false\n  ip_validation:", 1)
	loaders := map[string]func(string) error{
		"api":      func(p string) error { _, e := LoadApi(p); return e },
		"proxy":    func(p string) error { _, e := LoadProxy(p); return e },
		"worker":   func(p string) error { _, e := LoadWorker(p); return e },
		"schedule": func(p string) error { _, e := LoadSchedule(p); return e },
		"migrate":  func(p string) error { _, e := LoadMigration(p); return e },
	}
	for name, load := range loaders {
		t.Run(name, func(t *testing.T) {
			if err := load(writeConfig(t, content)); err == nil || !strings.Contains(err.Error(), "field opa not found") {
				t.Fatalf("old configuration accepted: %v", err)
			}
		})
	}
}
