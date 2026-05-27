package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootCommandIncludesRuntimeSubcommands(t *testing.T) {
	root := NewRootCommand()
	for _, name := range []string{"api", "proxy", "migrate", "schedule"} {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Fatalf("expected %q subcommand: %v", name, err)
		}
	}
}

func TestRootCommandIncludesEnvFileFlag(t *testing.T) {
	root := NewRootCommand()
	if flag := root.PersistentFlags().Lookup("env-file"); flag == nil {
		t.Fatal("expected env-file persistent flag to exist")
	}
}

func TestLoadEnvFileSearchesParentDirectories(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "dev", "backend")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}

	envFilePath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envFilePath, []byte("PROMPTGATE_DATABASE_URL=postgres://from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv("PROMPTGATE_DATABASE_URL", "")

	if err := loadEnvFile(nestedDir, ""); err != nil {
		t.Fatalf("load env file: %v", err)
	}

	if got := os.Getenv("PROMPTGATE_DATABASE_URL"); got != "postgres://from-dotenv" {
		t.Fatalf("expected database url from .env, got %q", got)
	}
}

func TestLoadEnvFileDoesNotOverrideNonEmptyEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	envFilePath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envFilePath, []byte("PROMPTGATE_DATABASE_URL=postgres://from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv("PROMPTGATE_DATABASE_URL", "postgres://from-env")

	if err := loadEnvFile(tempDir, ""); err != nil {
		t.Fatalf("load env file: %v", err)
	}

	if got := os.Getenv("PROMPTGATE_DATABASE_URL"); got != "postgres://from-env" {
		t.Fatalf("expected existing env var to win, got %q", got)
	}
}

func TestLoadEnvFileUsesExplicitPath(t *testing.T) {
	tempDir := t.TempDir()
	explicitEnvFilePath := filepath.Join(tempDir, "local.env")
	if err := os.WriteFile(explicitEnvFilePath, []byte("PROMPTGATE_DATABASE_URL=postgres://from-explicit\n"), 0o600); err != nil {
		t.Fatalf("write explicit env file: %v", err)
	}

	t.Setenv("PROMPTGATE_DATABASE_URL", "")

	if err := loadEnvFile(filepath.Join(tempDir, "missing"), explicitEnvFilePath); err != nil {
		t.Fatalf("load env file: %v", err)
	}

	if got := os.Getenv("PROMPTGATE_DATABASE_URL"); got != "postgres://from-explicit" {
		t.Fatalf("expected database url from explicit env file, got %q", got)
	}
}
