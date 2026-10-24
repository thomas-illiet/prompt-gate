package requestprivacy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareSharesMetadataOnlyStateWithOuterLayers(t *testing.T) {
	var outerContextMetadataOnly bool
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		r = r.WithContext(MarkMetadataOnly(r.Context()))
		if !IsMetadataOnly(r.Context()) {
			t.Fatal("inner request must be metadata-only")
		}
	})
	observer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r)
		outerContextMetadataOnly = IsMetadataOnly(r.Context())
	})

	Middleware(observer).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))

	if !outerContextMetadataOnly {
		t.Fatal("outer middleware did not observe the stricter privacy policy")
	}
}

func TestMiddlewareUsesIndependentStateForEachRequest(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/private" {
			_ = MarkMetadataOnly(r.Context())
			return
		}
		if IsMetadataOnly(r.Context()) {
			t.Fatal("privacy state leaked between requests")
		}
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/private", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/normal", nil))
}
