package openapi

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"promptgate/backend/internal/platform/config"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	defaultSessionCookieName = "promptgate_session"
	sessionSecurityScheme    = "sessionCookie"
)

//go:embed openapi.yaml
var specificationYAML []byte

// NewHandler returns an HTTP handler serving the runtime OpenAPI document.
func NewHandler(cfg config.APIConfig) http.HandlerFunc {
	document, err := buildDocument(cfg)

	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("{\"error\":\"openapi_document_unavailable\"}\n"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(document)
	}
}

// buildDocument parses and validates the embedded specification after applying
// the configured browser session cookie name.
func buildDocument(cfg config.APIConfig) ([]byte, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	document, err := loader.LoadFromData(specificationYAML)
	if err != nil {
		return nil, err
	}

	cookieName := strings.TrimSpace(cfg.SessionCookieName)
	if cookieName == "" {
		cookieName = defaultSessionCookieName
	}
	if document.Components == nil {
		return nil, errors.New("OpenAPI document has no components")
	}
	securityScheme, ok := document.Components.SecuritySchemes[sessionSecurityScheme]
	if !ok || securityScheme == nil || securityScheme.Value == nil {
		return nil, errors.New("OpenAPI document has no session cookie security scheme")
	}
	securityScheme.Value.Name = cookieName

	if err := document.Validate(context.Background()); err != nil {
		return nil, err
	}

	return json.Marshal(document)
}
