// Package testconfig loads the repository's shared integration-test YAML.
package testconfig

import (
	"path/filepath"
	"runtime"
	"testing"

	"promptgate/backend/internal/platform/config"
)

const integrationConfigName = "promptgate.integration.yaml"

// Path returns the absolute path to the shared integration-test configuration.
func Path(t testing.TB) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate integration-test configuration helper")
	}
	return filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "testdata", integrationConfigName)
}

// PostgresURL returns the PostgreSQL URL from the shared strict YAML config.
func PostgresURL(t testing.TB) string {
	t.Helper()
	cfg, err := config.LoadMigration(Path(t))
	if err != nil {
		t.Fatalf("load integration-test PostgreSQL configuration: %v", err)
	}
	return cfg.DatabaseURL
}

// RedisURL returns the Redis URL from the shared strict YAML config.
func RedisURL(t testing.TB) string {
	t.Helper()
	cfg, err := config.LoadWorker(Path(t))
	if err != nil {
		t.Fatalf("load integration-test Redis configuration: %v", err)
	}
	return cfg.RedisURL
}
