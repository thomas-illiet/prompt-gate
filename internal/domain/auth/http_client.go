package auth

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// NewKeycloakHTTPClient returns an HTTP client that trusts the CA certificate at caCertPath.
func NewKeycloakHTTPClient(caCertPath string) (*http.Client, error) {
	caCertPath = strings.TrimSpace(caCertPath)
	if caCertPath == "" {
		return nil, nil
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read Keycloak CA certificate: %w", err)
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("parse Keycloak CA certificate %q: no PEM certificates found", caCertPath)
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		}
	}

	tlsConfig := &tls.Config{RootCAs: rootCAs}
	if transport.TLSClientConfig != nil {
		tlsConfig = transport.TLSClientConfig.Clone()
		tlsConfig.RootCAs = rootCAs
	}
	transport.TLSClientConfig = tlsConfig

	return &http.Client{Transport: transport}, nil
}
