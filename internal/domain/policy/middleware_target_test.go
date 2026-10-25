package policy

import (
	"testing"

	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
)

func TestPolicyRequestTargetUsesProviderPrefixedProxyPath(t *testing.T) {
	snapshot := groups.NewSnapshotStore(nil)
	if err := snapshot.SetSnapshot(groups.Snapshot{
		KnownProviders: []string{"openai-main"},
		ProviderTypes:  map[string]provider.ProviderType{"openai-main": provider.ProviderTypeOpenAI},
		Users:          map[string]groups.UserAccess{},
		UserGroups:     map[string][]string{},
	}); err != nil {
		t.Fatalf("set group snapshot: %v", err)
	}

	name, providerType, relativePath := policyRequestTarget("/openai-main/v1/chat/completions", snapshot)
	if name != "openai-main" || providerType != "openai" || relativePath != "/v1/chat/completions" {
		t.Fatalf("unexpected structured target: name=%q type=%q path=%q", name, providerType, relativePath)
	}
}

func TestPolicyRequestTargetKeepsUnknownProviderStructured(t *testing.T) {
	name, providerType, relativePath := policyRequestTarget("/unknown/v1/responses/response-id", groups.NewSnapshotStore(nil))
	if name != "unknown" || providerType != "" || relativePath != "/v1/responses/response-id" {
		t.Fatalf("unexpected unknown target: name=%q type=%q path=%q", name, providerType, relativePath)
	}
}
