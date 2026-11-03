package httpclient

import (
	"net/http"
	"net/url"
	"testing"
)

func TestProviderRedirectOriginBoundary(t *testing.T) {
	origin, _ := url.Parse("https://provider.test/start")
	for _, tc := range []struct {
		target  string
		allowed bool
	}{
		{"https://provider.test/next", true}, {"https://PROVIDER.test:443/next", true},
		{"https://other.test/next", false}, {"http://provider.test/next", false},
		{"https://provider.test:444/next", false}, {"https://user@provider.test/next", false},
	} {
		t.Run(tc.target, func(t *testing.T) {
			target, _ := url.Parse(tc.target)
			req := &http.Request{URL: target, Response: &http.Response{Request: &http.Request{URL: origin}}}
			if got := CheckProviderRedirect(req, []*http.Request{{URL: origin}}) == nil; got != tc.allowed {
				t.Fatalf("redirect allowed=%v", got)
			}
			if got := CheckRedirectRequest(req) == nil; got != tc.allowed {
				t.Fatalf("transport redirect allowed=%v", got)
			}
		})
	}
}
