package groups

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/proxylimits"
)

func requestProviderName(path string) string {
	path = strings.TrimPrefix(path, "/")
	providerName, _, _ := strings.Cut(path, "/")
	return strings.TrimSpace(providerName)
}

func requestProviderPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	_, suffix, ok := strings.Cut(path, "/")
	if !ok {
		return "/"
	}
	return "/" + strings.TrimPrefix(suffix, "/")
}

func isModelBypassRoute(providerType provider.ProviderType, path string) bool {
	providerPath := requestProviderPath(path)
	switch providerType {
	case provider.ProviderTypeOpenAI:
		return routeExactOrSubtree(providerPath, "/v1/models") ||
			routeExactOrSubtree(providerPath, "/v1/conversations") ||
			routeSubtree(providerPath, "/v1/responses/")
	case provider.ProviderTypeOllama:
		return routeExactOrSubtree(providerPath, "/v1/models")
	case provider.ProviderTypeAnthropic:
		return routeExactOrSubtree(providerPath, "/v1/models") ||
			providerPath == "/v1/messages/count_tokens" ||
			routeSubtree(providerPath, "/api/event_logging/")
	default:
		return false
	}
}

func isFilterableModelListRoute(providerType provider.ProviderType, path, method string) bool {
	if method != http.MethodGet {
		return false
	}
	if providerType != provider.ProviderTypeOpenAI && providerType != provider.ProviderTypeOllama {
		return false
	}
	providerPath := requestProviderPath(path)
	return providerPath == "/v1/models" || providerPath == "/v1/models/"
}

func routeExactOrSubtree(path, route string) bool {
	return path == route || strings.HasPrefix(path, route+"/")
}

func routeSubtree(path, routePrefix string) bool {
	return strings.HasPrefix(path, routePrefix)
}

func requestModel(r *http.Request, maxBufferedRequestBytes int64) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	raw, err := proxylimits.ReadAll(r.Body, maxBufferedRequestBytes)
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", nil
	}

	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Model), nil
}
