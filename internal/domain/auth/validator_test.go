package auth

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes the test transport function.
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// TestNewValidatorUsesConfiguredHTTPClient verifies JWKS fetching uses the injected client.
func TestNewValidatorUsesConfiguredHTTPClient(t *testing.T) {
	wantErr := errors.New("custom jwks client used")
	var called atomic.Bool
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			called.Store(true)
			return nil, wantErr
		}),
	}

	validator, err := NewValidator(
		context.Background(),
		"https://keycloak.example.com/realms/promptgate",
		"https://keycloak.example.com/realms/promptgate/protocol/openid-connect/certs",
		WithValidatorHTTPClient(client),
	)
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}
	validator.Close()

	deadline := time.Now().Add(time.Second)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !called.Load() {
		t.Fatal("expected configured HTTP client to be used")
	}
}
