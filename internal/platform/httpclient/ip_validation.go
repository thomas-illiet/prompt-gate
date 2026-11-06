package httpclient

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateIPValidation validates configuration without disclosing its values.
func ValidateIPValidation(urlTemplate, bearerToken string) error {
	parsed, err := url.Parse(urlTemplate)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Opaque != "" || parsed.User != nil || strings.Contains(urlTemplate, "#") {
		return errors.New("proxy.ip_validation.url_template must be an absolute HTTP(S) URL without credentials or fragment")
	}
	if strings.Count(urlTemplate, "{ip}") != 1 || strings.Count(parsed.Path, "{ip}") != 1 || strings.ContainsAny(strings.Replace(urlTemplate, "{ip}", "", 1), "{}") {
		return errors.New("proxy.ip_validation.url_template must contain exactly one {ip} placeholder in its path")
	}
	if bearerToken == "" {
		return errors.New("proxy.ip_validation.bearer_token is required when validation is enabled")
	}
	for _, char := range bearerToken {
		if char <= ' ' || char >= 127 {
			return errors.New("proxy.ip_validation.bearer_token must contain only non-whitespace ASCII characters")
		}
	}
	return nil
}
