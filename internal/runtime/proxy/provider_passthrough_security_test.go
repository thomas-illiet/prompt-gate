package runtime

import (
	"reflect"
	"testing"
)

func TestProviderPassthroughRoutesAreFailClosed(t *testing.T) {
	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "openai",
			got:  newOpenAIProvider("openai", "https://api.openai.example/v1", "key").PassthroughRoutes(),
			want: []string{"/models"},
		},
		{
			name: "ollama",
			got:  newOllamaProvider("ollama", "https://ollama.example/v1", "key").PassthroughRoutes(),
			want: []string{"/models"},
		},
		{
			name: "anthropic",
			got:  newAnthropicProvider("anthropic", "https://api.anthropic.example", "key").PassthroughRoutes(),
			want: []string{"/v1/models", "/v1/messages/count_tokens"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !reflect.DeepEqual(test.got, test.want) {
				t.Fatalf("passthrough routes: got %#v, want %#v", test.got, test.want)
			}
		})
	}
}
