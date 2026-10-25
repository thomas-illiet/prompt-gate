package config

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// LogConfig contains process-wide logging settings shared by every runtime.
type LogConfig struct {
	LogLevel string
}

// DatabaseURLConfig contains the database connection string shared by database-backed runtimes.
type DatabaseURLConfig struct {
	DatabaseURL        string
	ConnMaxLifetime    time.Duration
	MaxIdleConnections int
	MaxOpenConnections int
}

// TLSConfig contains the optional global CA file used by outbound HTTP clients.
type TLSConfig struct {
	CAFile string
}

// RedisConfig contains Redis connectivity and cache settings.
type RedisConfig struct {
	RedisURL                string
	RedisCacheTTL           time.Duration
	RedisPoolSize           int
	RedisMinIdleConnections int
	RedisPoolTimeout        time.Duration
	RedisReadTimeout        time.Duration
	RedisWriteTimeout       time.Duration
}

// ServerConfig contains settings shared by HTTP servers.
type ServerConfig struct {
	Port               string
	CORSAllowedOrigins []string
}

// SessionConfig contains browser session settings.
type SessionConfig struct {
	SessionCookieName string
	SessionTTL        time.Duration
}

// SecretsConfig contains secrets used by application services.
type SecretsConfig struct {
	JWTSecret  string
	SecretsKey string
}

// KeycloakConfig contains OIDC client settings.
type KeycloakConfig struct {
	KeycloakIssuerURL    string
	KeycloakJWKSURL      string
	KeycloakClientID     string
	KeycloakClientSecret string
	OIDCRedirectURL      string
}

// PublicURLConfig contains externally visible application URLs.
type PublicURLConfig struct {
	FrontendBaseURL string
	BackendBaseURL  string
	ProxyBaseURL    string
}

// APIHTTPConfig contains settings specific to the management API transport.
type APIHTTPConfig struct {
	StaticAssetsDir   string
	AdminAPIKey       string
	AdminContactEmail string
}

// ScheduleIntervals contains intervals used by scheduled background jobs.
type ScheduleIntervals struct {
	TokenCleanupInterval           time.Duration
	UserAccessExpirationInterval   time.Duration
	UsageRawRetention              time.Duration
	UsageRawCleanupInterval        time.Duration
	SubscriptionQuotaSyncInterval  time.Duration
	QuotaResetDistributionInterval time.Duration
}

// ProxyRuntimeConfig contains proxy reload, trust, and buffering settings.
type ProxyRuntimeConfig struct {
	ProxyBasePath                 string
	ProxyTrustForwardHeaders      bool
	ProxyTrustedProxies           []netip.Prefix
	ProxyClientIPDebug            bool
	ProxyDebugRequests            bool
	ProxyReloadDebounce           time.Duration
	ProxyMaxBufferedRequestBytes  int64
	ProxyMaxBufferedResponseBytes int64
	ProxyUpstreamTimeout          time.Duration
	OPAEnabled                    bool
	OPAURL                        string
	OPAPolicyPath                 string
	OPATimeout                    time.Duration
	OPACacheTTL                   time.Duration
}

// WorkerRuntimeConfig contains Redis stream worker settings.
type WorkerRuntimeConfig struct {
	WorkerBatchSize          int64
	WorkerBlockTimeout       time.Duration
	WorkerPendingIdleTimeout time.Duration
	WorkerConsumerName       string
}

type UsageCostConfig struct {
	Input     float64
	Output    float64
	Embedding float64
}

// ModelLabConfig controls the user-facing model catalog and ephemeral playground.
// The same values are loaded by the API and proxy processes so tickets issued by
// the API can be validated and constrained by the proxy.
type ModelLabConfig struct {
	TicketTTL       time.Duration
	MaxPromptBytes  int64
	MaxOutputTokens int
	MaxTargets      int
	MaxConcurrent   int
	CatalogFreshTTL time.Duration
	CatalogStaleTTL time.Duration
}

// OTelConfig controls OpenTelemetry trace export from the proxy process.
type OTelConfig struct {
	Enabled            bool
	Endpoint           string
	APIKey             string
	ProjectName        string
	ServiceName        string
	Environment        string
	CapturePrompts     bool
	CaptureOutput      bool
	CaptureThinking    bool
	ExportTimeout      time.Duration
	BatchTimeout       time.Duration
	MaxQueueSize       int
	MaxExportBatchSize int
	Insecure           bool
}

// APIConfig contains the settings loaded by the management API runtime.
type APIConfig struct {
	LogConfig
	DatabaseURLConfig
	TLSConfig
	RedisConfig
	ServerConfig
	SessionConfig
	SecretsConfig
	KeycloakConfig
	PublicURLConfig
	APIHTTPConfig
	ScheduleIntervals
	ProxyRuntimeConfig
	ModelLab  ModelLabConfig
	UsageCost UsageCostConfig
}

// Config is retained as an APIConfig alias for domain APIs that consume OIDC
// settings. New runtime entry points should use the explicit runtime type.
type Config = APIConfig

// ScheduleConfig contains the settings loaded by scheduled jobs.
type ScheduleConfig struct {
	LogConfig
	DatabaseURLConfig
	TLSConfig
	RedisConfig
	SecretsConfig
	ScheduleIntervals
	ProxyRuntimeConfig
}

// ProxyConfig contains the settings loaded by the proxy server.
type ProxyConfig struct {
	LogConfig
	DatabaseURLConfig
	TLSConfig
	RedisConfig
	ServerConfig
	SessionConfig
	SecretsConfig
	PublicURLConfig
	ProxyRuntimeConfig
	ModelLab  ModelLabConfig
	OTel      OTelConfig
	UsageCost UsageCostConfig
}

// WorkerConfig contains the settings loaded by the Redis worker.
type WorkerConfig struct {
	LogConfig
	DatabaseURLConfig
	RedisConfig
	WorkerRuntimeConfig
}

// MigrationConfig contains the settings loaded by database migrations.
type MigrationConfig struct {
	LogConfig
	DatabaseURLConfig
}

type positiveDuration struct {
	path  string
	value time.Duration
}

// validatePositiveDurations validates durations before they reach tickers,
// HTTP clients, or cache stores that require strictly positive values.
func validatePositiveDurations(values ...positiveDuration) error {
	for _, item := range values {
		if item.value <= 0 {
			return fmt.Errorf("%s must be greater than zero", item.path)
		}
	}
	return nil
}

func validateModelLabConfig(cfg ModelLabConfig) error {
	if cfg.TicketTTL <= 0 || cfg.TicketTTL > 5*time.Minute {
		return errors.New("common.model_lab.ticket_ttl must be greater than zero and at most 5m")
	}
	if cfg.MaxPromptBytes <= 0 {
		return errors.New("common.model_lab.max_prompt_bytes must be greater than zero")
	}
	if cfg.MaxOutputTokens <= 0 {
		return errors.New("common.model_lab.max_output_tokens must be greater than zero")
	}
	if cfg.MaxTargets <= 0 || cfg.MaxTargets > 3 {
		return errors.New("common.model_lab.max_targets must be between 1 and 3")
	}
	if cfg.MaxConcurrent <= 0 || cfg.MaxConcurrent > 3 {
		return errors.New("common.model_lab.max_concurrent must be between 1 and 3")
	}
	if cfg.CatalogFreshTTL <= 0 {
		return errors.New("common.model_lab.catalog_fresh_ttl must be greater than zero")
	}
	if cfg.CatalogStaleTTL < cfg.CatalogFreshTTL {
		return errors.New("common.model_lab.catalog_stale_ttl must be greater than or equal to common.model_lab.catalog_fresh_ttl")
	}
	return nil
}

func validateModelLabProxyBaseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" || parsed.User != nil || parsed.RawPath != "" ||
		parsed.EscapedPath() != parsed.Path ||
		(parsed.Path != "" && path.Clean(parsed.Path) != parsed.Path) ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return errors.New("common.public_urls.proxy must be an HTTP(S) public URL with an optional normalized path and without credentials, encoded path, query, or fragment")
	}
	return nil
}

// validateProxyBasePath accepts an optional normalized, unescaped URL path
// used to mount the proxy behind a path-based ingress.
func validateProxyBasePath(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || !strings.HasPrefix(value, "/") || value == "/" ||
		parsed.IsAbs() || parsed.Host != "" || parsed.User != nil ||
		parsed.Path != value || parsed.RawPath != "" || parsed.EscapedPath() != parsed.Path ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		path.Clean(value) != value {
		return errors.New("proxy.base_path must be empty or a normalized absolute path without a trailing slash, credentials, encoding, query, or fragment")
	}
	return nil
}

func validateProxyPublicURLPath(publicURL string, basePath string) error {
	parsed, err := url.Parse(publicURL)
	if err != nil {
		return errors.New("common.public_urls.proxy must be a valid URL")
	}
	publicPath := parsed.Path
	if publicPath == "/" {
		publicPath = ""
	}
	if publicPath != basePath {
		return errors.New("common.public_urls.proxy path must equal proxy.base_path")
	}
	return nil
}

func validateNonNegativeFloat(value float64, configPath string) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be a valid number", configPath)
	}
	if value < 0 {
		return fmt.Errorf("%s must be greater than or equal to zero", configPath)
	}
	return nil
}

func parseCIDRList(values []string, configPath string) ([]netip.Prefix, error) {
	if len(values) == 0 {
		return nil, nil
	}
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("%s contains invalid CIDR %q", configPath, value)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func validateOptionalFile(configPath string, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("%s is not accessible: %w", configPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s must be a file", configPath)
	}
	return nil
}

// ListenAddress returns the address the server should bind to, ensuring it starts with ":".
func (c ServerConfig) ListenAddress() string {
	if strings.HasPrefix(c.Port, ":") {
		return c.Port
	}

	return fmt.Sprintf(":%s", c.Port)
}

// OIDCCallbackURL returns the full OIDC redirect callback URL.
func (c PublicURLConfig) OIDCCallbackURL() string {
	return c.BackendBaseURL + "/auth/callback"
}

func validateOIDCRedirectURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" ||
		parsed.RawPath != "" || parsed.EscapedPath() != parsed.Path ||
		!isNormalizedOIDCRedirectPath(parsed.Path) ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return errors.New("api.oidc.redirect_url must be an absolute HTTP(S) URL with a host and without credentials, an encoded or non-normalized path, query, or fragment")
	}
	return nil
}

func isNormalizedOIDCRedirectPath(value string) bool {
	if value == "" || value == "/" {
		return true
	}
	cleaned := path.Clean(value)
	if strings.HasSuffix(value, "/") {
		cleaned += "/"
	}
	return cleaned == value
}

// deriveProxyBaseURL returns the API origin with the configured proxy port.
func deriveProxyBaseURL(backendBaseURL string, proxyPort string) string {
	parsed, err := url.Parse(backendBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return ""
	}
	port := strings.TrimPrefix(strings.TrimSpace(proxyPort), ":")
	if port == "" {
		port = "8081"
	}
	parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

// SessionCookieSecure returns true when the backend base URL uses HTTPS.
func (c PublicURLConfig) SessionCookieSecure() bool {
	return strings.HasPrefix(c.BackendBaseURL, "https://")
}

// SlogLevel converts the configured log level string to a slog.Level.
func (c LogConfig) SlogLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// DatabaseLogValue returns a redacted database connection string safe for logging.
func (c DatabaseURLConfig) DatabaseLogValue() string {
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return "unparseable_database_url"
	}

	databaseName := path.Base(parsed.Path)
	if databaseName == "." || databaseName == "/" || databaseName == "" {
		databaseName = "unknown"
	}

	host := parsed.Hostname()
	if host == "" {
		host = "unknown"
	}

	port := parsed.Port()
	if port == "" {
		port = "default"
	}

	return fmt.Sprintf("%s/%s (port=%s)", host, databaseName, port)
}

// expandLoopbackOrigins expands loopback origins to include all loopback aliases (localhost, 127.0.0.1, ::1).
func expandLoopbackOrigins(origins []string) []string {
	values := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))

	for _, origin := range origins {
		normalized := strings.TrimRight(strings.TrimSpace(origin), "/")
		if normalized == "" {
			continue
		}

		addOrigin(&values, seen, normalized)

		parsed, err := url.Parse(normalized)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}

		if !isLoopbackHost(parsed.Hostname()) {
			continue
		}

		if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			continue
		}

		for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
			alias := url.URL{
				Scheme: parsed.Scheme,
				Host:   joinOriginHostPort(host, parsed.Port()),
			}
			addOrigin(&values, seen, alias.String())
		}
	}

	return values
}

// addOrigin appends origin to values if not already in seen.
func addOrigin(values *[]string, seen map[string]struct{}, origin string) {
	if _, ok := seen[origin]; ok {
		return
	}

	seen[origin] = struct{}{}
	*values = append(*values, origin)
}

// joinOriginHostPort formats a host and port into a valid URL host component.
func joinOriginHostPort(host string, port string) string {
	if port == "" {
		if host == "::1" {
			return "[::1]"
		}

		return host
	}

	return net.JoinHostPort(host, port)
}

// isLoopbackHost reports whether the host is a loopback address.
func isLoopbackHost(host string) bool {
	switch strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]") {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
