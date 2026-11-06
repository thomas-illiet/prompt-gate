package policy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	platformhttp "promptgate/backend/internal/platform/httpclient"
)

var (
	ErrUnavailable     = errors.New("IP validation unavailable")
	ErrInvalidResponse = errors.New("unexpected IP validation status")
	ErrInvalidIP       = errors.New("invalid client IP")
)

const DeniedReason = "ip_not_allowed"

type Decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

type Client struct {
	httpClient  *http.Client
	urlTemplate string
	bearerToken string
}

func NewClient(urlTemplate, bearerToken string, httpClient *http.Client) (*Client, error) {
	urlTemplate, bearerToken = strings.TrimSpace(urlTemplate), strings.TrimSpace(bearerToken)
	if err := platformhttp.ValidateIPValidation(urlTemplate, bearerToken); err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	// Clone the client and never redirect credentials, even to the same host.
	client := *httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if client.Timeout <= 0 {
		client.Timeout = 2 * time.Second
	}
	return &Client{httpClient: &client, urlTemplate: urlTemplate, bearerToken: bearerToken}, nil
}

func normalizeIP(value string) (string, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || addr.Zone() != "" {
		return "", fmt.Errorf("%w: %w", ErrUnavailable, ErrInvalidIP)
	}
	return addr.Unmap().String(), nil
}

func (c *Client) Decide(ctx context.Context, clientIP string) (Decision, error) {
	ip, err := normalizeIP(clientIP)
	if err != nil {
		return Decision{}, err
	}
	target := strings.Replace(c.urlTemplate, "{ip}", url.PathEscape(ip), 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Decision{}, ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Transport errors may contain URLs, IPs or secrets. Expose categories only.
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return Decision{}, fmt.Errorf("%w: %w", ErrUnavailable, context.DeadlineExceeded)
		case errors.Is(err, context.Canceled):
			return Decision{}, fmt.Errorf("%w: %w", ErrUnavailable, context.Canceled)
		default:
			return Decision{}, ErrUnavailable
		}
	}
	defer resp.Body.Close()
	// Status is the entire contract: never parse or wait for a response body.
	switch resp.StatusCode {
	case http.StatusOK:
		return Decision{Allow: true}, nil
	case http.StatusNotFound:
		return Decision{Reason: DeniedReason}, nil
	default:
		return Decision{}, fmt.Errorf("%w: %w (%d)", ErrUnavailable, ErrInvalidResponse, resp.StatusCode)
	}
}
