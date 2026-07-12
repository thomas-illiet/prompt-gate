package proxy

import "testing"

func TestIsEmbeddingTokenUsageSupportsCurrentAndLegacyMetadata(t *testing.T) {
	tests := []struct {
		name      string
		tokenType string
		metadata  string
		want      bool
	}{
		{name: "typed embedding", tokenType: tokenUsageTypeEmbedding, want: true},
		{name: "metadata type", metadata: `{"type":"embedding"}`, want: true},
		{name: "legacy endpoint", metadata: `{"endpoint":"/embeddings"}`, want: true},
		{name: "completion", tokenType: tokenUsageTypeCompletion, metadata: `{}`, want: false},
		{name: "invalid metadata", metadata: `{`, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isEmbeddingTokenUsage(test.tokenType, test.metadata); got != test.want {
				t.Fatalf("isEmbeddingTokenUsage(%q, %q) = %v, want %v", test.tokenType, test.metadata, got, test.want)
			}
		})
	}
}
