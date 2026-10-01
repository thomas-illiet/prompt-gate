package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var (
	ErrUnavailable     = errors.New("policy unavailable")
	ErrInvalidResponse = errors.New("invalid policy response")
)

const maxResponseBytes = 1 << 20

type IdentityInput struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Role           string `json:"role"`
	CredentialID   string `json:"credential_id"`
	CredentialName string `json:"credential_name"`
}

type RequestInput struct {
	ClientIP string `json:"client_ip"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Host     string `json:"host"`
}

type Input struct {
	Identity IdentityInput `json:"identity"`
	Request  RequestInput  `json:"request"`
}

type Decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

type Client struct {
	httpClient  *http.Client
	decisionURL string
	healthURL   string
}

func NewClient(baseURL, policyPath string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("OPA URL must be an absolute HTTP(S) URL")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	path := strings.Trim(strings.TrimSpace(policyPath), "/")
	if path == "" {
		return nil, errors.New("OPA policy path must not be empty")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	base := strings.TrimRight(parsed.String(), "/")
	return &Client{
		httpClient:  httpClient,
		decisionURL: base + "/v1/data/" + path,
		healthURL:   base + "/health",
	}, nil
}

func (c *Client) Decide(ctx context.Context, input Input) (Decision, error) {
	payload, err := json.Marshal(struct {
		Input Input `json:"input"`
	}{Input: input})
	if err != nil {
		return Decision{}, fmt.Errorf("encode OPA input: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.decisionURL, bytes.NewReader(payload))
	if err != nil {
		return Decision{}, fmt.Errorf("create OPA request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Decision{}, fmt.Errorf("%w: request decision: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return Decision{}, fmt.Errorf("%w: OPA returned status %d", ErrUnavailable, resp.StatusCode)
	}
	var envelope struct {
		Result *struct {
			Allow  *bool   `json:"allow"`
			Reason *string `json:"reason"`
		} `json:"result"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := decoder.Decode(&envelope); err != nil || envelope.Result == nil || envelope.Result.Allow == nil || envelope.Result.Reason == nil {
		return Decision{}, fmt.Errorf("%w: %w", ErrUnavailable, ErrInvalidResponse)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Decision{}, fmt.Errorf("%w: %w", ErrUnavailable, ErrInvalidResponse)
	}
	return Decision{Allow: *envelope.Result.Allow, Reason: *envelope.Result.Reason}, nil
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: health request: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: health status %d", ErrUnavailable, resp.StatusCode)
	}
	return nil
}
