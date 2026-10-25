package cli

import (
	"strings"
	"testing"
)

func TestRootCommandIncludesRuntimeSubcommands(t *testing.T) {
	root := NewRootCommand()
	for _, name := range []string{"api", "proxy", "migrate", "schedule", "worker"} {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Fatalf("expected %q subcommand: %v", name, err)
		}
	}
}

func TestRootCommandRequiresConfigFlag(t *testing.T) {
	root := NewRootCommand()
	if flag := root.PersistentFlags().Lookup("config"); flag == nil {
		t.Fatal("expected config persistent flag to exist")
	}
	if flag := root.PersistentFlags().Lookup("env-file"); flag != nil {
		t.Fatal("did not expect legacy env-file flag")
	}
	root.SetArgs([]string{"migrate"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "required flag") || !strings.Contains(err.Error(), "config") {
		t.Fatalf("expected missing config error, got %v", err)
	}
}
