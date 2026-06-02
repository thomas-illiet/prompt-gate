package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// NewWithCAFile returns an HTTP client that trusts the PEM certificates from caFile.
func NewWithCAFile(caFile string, timeout time.Duration) (*http.Client, error) {
	caFile = strings.TrimSpace(caFile)
	if caFile == "" {
		return nil, nil
	}

	transport, err := NewTransportWithCAFile(caFile)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

// NewTransportWithCAFile returns an HTTP transport that trusts the PEM certificates from caFile.
func NewTransportWithCAFile(caFile string) (*http.Transport, error) {
	caFile = strings.TrimSpace(caFile)
	if caFile == "" {
		return nil, nil
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
		return nil, fmt.Errorf("parse CA file %q: no PEM certificates found", caFile)
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}

	tlsConfig := &tls.Config{RootCAs: rootCAs}
	if transport.TLSClientConfig != nil {
		tlsConfig = transport.TLSClientConfig.Clone()
		tlsConfig.RootCAs = rootCAs
	}
	transport.TLSClientConfig = tlsConfig
	return transport, nil
}
