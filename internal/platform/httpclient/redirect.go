package httpclient

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

var ErrCrossOriginRedirect = errors.New("provider redirect outside configured origin is forbidden")

func SameOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if strings.EqualFold(u.Scheme, "https") {
			return "443"
		}
		return "80"
	}
	return a != nil && b != nil && strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b) && b.User == nil
}

// CheckProviderRedirect runs before any credentials or body reach a new origin.
func CheckProviderRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("too many provider redirects")
	}
	if len(via) > 0 && !SameOrigin(via[0].URL, req.URL) {
		return ErrCrossOriginRedirect
	}
	return nil
}

func CheckRedirectRequest(req *http.Request) error {
	if req.Response != nil && req.Response.Request != nil &&
		!SameOrigin(req.Response.Request.URL, req.URL) {
		return ErrCrossOriginRedirect
	}
	return nil
}
