package auth

import "testing"

// TestFrontendRedirectURLDefaultsToDashboard verifies frontend redirect URL defaults to dashboard.
func TestFrontendRedirectURLDefaultsToDashboard(t *testing.T) {
	service := OIDCService{frontendBaseURL: "http://localhost:8080"}

	redirectURL, err := service.FrontendRedirectURL("", "")
	if err != nil {
		t.Fatalf("frontend redirect URL: %v", err)
	}

	if redirectURL != "http://localhost:8080/dashboard" {
		t.Fatalf("expected dashboard redirect, got %q", redirectURL)
	}
}
