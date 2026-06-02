package httpclient

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewWithCAFileTrustsCACert verifies the custom CA file is used for TLS.
func TestNewWithCAFileTrustsCACert(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	if resp, err := http.DefaultClient.Get(server.URL); err == nil {
		resp.Body.Close()
		t.Fatal("expected default client to reject the self-signed server certificate")
	}

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	caCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: server.Certificate().Raw,
	})
	if err := os.WriteFile(caFile, caCert, 0o600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}

	client, err := NewWithCAFile(caFile, time.Second)
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

// TestNewWithCAFileRejectsInvalidPEM verifies invalid PEM content fails.
func TestNewWithCAFileRejectsInvalidPEM(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}

	if _, err := NewWithCAFile(caFile, time.Second); err == nil {
		t.Fatal("expected invalid PEM to fail")
	}
}
