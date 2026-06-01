package auth

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewKeycloakHTTPClientTrustsCACertPath(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	caCertPath := filepath.Join(t.TempDir(), "keycloak-ca.pem")
	caCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: server.Certificate().Raw,
	})
	if err := os.WriteFile(caCertPath, caCert, 0o600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}

	client, err := NewKeycloakHTTPClient(caCertPath)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request with custom CA: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
}

func TestNewKeycloakHTTPClientRejectsInvalidPEM(t *testing.T) {
	caCertPath := filepath.Join(t.TempDir(), "keycloak-ca.pem")
	if err := os.WriteFile(caCertPath, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}

	if _, err := NewKeycloakHTTPClient(caCertPath); err == nil {
		t.Fatal("expected invalid PEM to fail")
	}
}
