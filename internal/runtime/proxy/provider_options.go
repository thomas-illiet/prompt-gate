package runtime

import (
	"net/http"

	platformhttp "promptgate/backend/internal/platform/httpclient"
	"promptgate/backend/internal/platform/proxylimits"
)

type providerRuntimeOptions struct {
	httpClient               *http.Client
	maxBufferedRequestBytes  int64
	maxBufferedResponseBytes int64
}

func defaultProviderRuntimeOptions() providerRuntimeOptions {
	return providerRuntimeOptions{
		httpClient:               &http.Client{Timeout: proxylimits.DefaultUpstreamTimeout},
		maxBufferedRequestBytes:  proxylimits.DefaultMaxBufferedRequestBytes,
		maxBufferedResponseBytes: proxylimits.DefaultMaxBufferedResponseBytes,
	}
}

func normalizeProviderRuntimeOptions(opts providerRuntimeOptions) providerRuntimeOptions {
	defaults := defaultProviderRuntimeOptions()
	if opts.httpClient == nil {
		opts.httpClient = defaults.httpClient
	}
	client := *opts.httpClient
	previous := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := platformhttp.CheckProviderRedirect(req, via); err != nil {
			return err
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	opts.httpClient = &client
	if opts.maxBufferedRequestBytes <= 0 {
		opts.maxBufferedRequestBytes = defaults.maxBufferedRequestBytes
	}
	if opts.maxBufferedResponseBytes <= 0 {
		opts.maxBufferedResponseBytes = defaults.maxBufferedResponseBytes
	}
	return opts
}

func writeRequestBodyTooLarge(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_, _ = w.Write([]byte(`{"error":"request_body_too_large"}`))
}
