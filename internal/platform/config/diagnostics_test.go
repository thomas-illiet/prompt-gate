package config

import "testing"

func TestHealthAddressValidation(t *testing.T) {
	for _, address := range []string{":8082", "127.0.0.1:8083", "[::1]:8082"} {
		if err := validateHealthAddress("health", address); err != nil {
			t.Fatal(err)
		}
	}
	for _, address := range []string{"", ":0", ":-1", ":65536", "8082", "localhost:http"} {
		if err := validateHealthAddress("health", address); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
}
