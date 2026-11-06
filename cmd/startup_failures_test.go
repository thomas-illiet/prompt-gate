package cli

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"promptgate/backend/internal/platform/health"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/testconfig"
)

func TestEveryCommandFailsFastOnInvalidConfigurationOrDatabase(t *testing.T) {
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)
	raw, err := os.ReadFile(testconfig.Path(t))
	if err != nil {
		t.Fatal(err)
	}
	// Hold the port without serving PostgreSQL: the connection is refused after
	// closing it, and connect_timeout still bounds a possible local port reuse.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	raw = []byte(strings.ReplaceAll(string(raw), testconfig.PostgresURL(t), "postgres://postgres:postgres@"+address+"/missing?connect_timeout=1&sslmode=disable"))
	path := filepath.Join(t.TempDir(), "unavailable.yaml")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"api", "proxy", "worker", "schedule", "migrate"} {
		t.Run(command, func(t *testing.T) {
			for _, tc := range []struct{ path, expected string }{{filepath.Join(t.TempDir(), "missing.yaml"), ""}, {path, "postgres"}} {
				root := NewRootCommand()
				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				root.SetArgs([]string{command, "--config", tc.path})
				if err := root.Execute(); err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.expected) {
					t.Fatalf("command did not reject unavailable dependency: %v", err)
				}
			}
		})
	}
}

func TestBackgroundDiagnosticsReflectDependenciesAndProgress(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	redis := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+redis.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	progress := health.NewProgress()
	ctx, cancel := context.WithCancel(health.WithProgress(context.Background(), progress))
	defer cancel()
	closeServer, err := startDiagnostics(ctx, address, db, store, progress)
	if err != nil {
		t.Fatal(err)
	}
	defer closeServer()
	client := &http.Client{Timeout: time.Second}
	check := func(path string, status int) {
		t.Helper()
		response, err := client.Get("http://" + address + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("%s = %d want %d", path, response.StatusCode, status)
		}
	}
	check("/live", 200)
	check("/ready", 200)
	if _, err := startDiagnostics(ctx, address, db, store, progress); err == nil {
		t.Fatal("occupied diagnostics port accepted")
	}
	// A stalled job must affect liveness independently of dependency availability.
	health.Beat(ctx, "test-job", -time.Second)
	check("/live", 503)
	check("/ready", 503)
	health.Beat(ctx, "test-job", time.Minute)
	check("/live", 200)
	check("/ready", 200)
	_ = sqlDB.Close()
	check("/ready", 503)
	check("/live", 200)
	cancel()
	check("/ready", 503)
}
