package runtime

import coderbridge "github.com/coder/aibridge"

// anthropicProvider restricts passthrough to the non-destructive operations
// required by supported clients. In particular, provider telemetry and model
// resource subtrees are not exposed through Prompt Gate.
type anthropicProvider struct {
	coderbridge.Provider
}

func newAnthropicProvider(name, baseURL, key string) *anthropicProvider {
	return &anthropicProvider{Provider: coderbridge.NewAnthropicProvider(coderbridge.AnthropicConfig{
		Name:    name,
		BaseURL: baseURL,
		Key:     key,
	}, nil)}
}

func (*anthropicProvider) PassthroughRoutes() []string {
	return []string{"/v1/models", "/v1/messages/count_tokens"}
}
