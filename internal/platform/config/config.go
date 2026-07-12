package config

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LogConfig contains process-wide logging settings shared by every runtime.
type LogConfig struct {
	LogLevel string
}

// DatabaseURLConfig contains the database connection string shared by database-backed runtimes.
type DatabaseURLConfig struct {
	DatabaseURL string
}

// TLSConfig contains the optional global CA file used by outbound HTTP clients.
type TLSConfig struct {
	CAFile string
}

// RedisConfig contains Redis connectivity and cache settings.
type RedisConfig struct {
	RedisURL      string
	RedisCacheTTL time.Duration
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
}

// PublicURLConfig contains externally visible application URLs.
type PublicURLConfig struct {
	FrontendBaseURL string
	BackendBaseURL  string
	ProxyBaseURL    string
}

// APIHTTPConfig contains settings specific to the management API transport.
type APIHTTPConfig struct {
	StaticAssetsDir string
	AdminAPIKey     string
}

// ScheduleIntervals contains intervals used by scheduled background jobs.
type ScheduleIntervals struct {
	TokenCleanupInterval          time.Duration
	UserAccessExpirationInterval  time.Duration
	UsageRawRetention             time.Duration
	UsageRawCleanupInterval       time.Duration
	SubscriptionQuotaSyncInterval time.Duration
}

// ProxyRuntimeConfig contains proxy reload, trust, and buffering settings.
type ProxyRuntimeConfig struct {
	ProxyTrustForwardHeaders      bool
	ProxyTrustedProxies           []netip.Prefix
	ProxyReloadDebounce           time.Duration
	ProxyMaxBufferedRequestBytes  int64
	ProxyMaxBufferedResponseBytes int64
	ProxyUpstreamTimeout          time.Duration
}

// WorkerRuntimeConfig contains Redis stream worker settings.
type WorkerRuntimeConfig struct {
	WorkerBatchSize          int64
	WorkerBlockTimeout       time.Duration
	WorkerPendingIdleTimeout time.Duration
	WorkerConsumerName       string
}

type UsageCostConfig struct {
	Enabled   bool
	Input     float64
	Output    float64
	Embedding float64
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
	envName string
	value   time.Duration
}

// validatePositiveDurations validates durations before they reach tickers,
// HTTP clients, or cache stores that require strictly positive values.
func validatePositiveDurations(values ...positiveDuration) error {
	for _, item := range values {
		if item.value <= 0 {
			return fmt.Errorf("%s must be greater than zero", item.envName)
		}
	}
	return nil
}

func loadLogConfig(v *viper.Viper) LogConfig {
	return LogConfig{LogLevel: v.GetString("log_level")}
}

func loadDatabaseURLConfig(v *viper.Viper) DatabaseURLConfig {
	return DatabaseURLConfig{DatabaseURL: strings.TrimSpace(v.GetString("database_url"))}
}

func loadTLSConfig(v *viper.Viper) TLSConfig {
	return TLSConfig{CAFile: strings.TrimSpace(v.GetString("ca_file"))}
}

func loadRedisConfig(v *viper.Viper) RedisConfig {
	return RedisConfig{
		RedisURL:      strings.TrimSpace(v.GetString("redis_url")),
		RedisCacheTTL: v.GetDuration("redis_cache_ttl"),
	}
}

func loadServerConfig(v *viper.Viper, portKey string) ServerConfig {
	return ServerConfig{
		Port:               v.GetString(portKey),
		CORSAllowedOrigins: v.GetStringSlice("cors_allowed_origins"),
	}
}

func loadSessionConfig(v *viper.Viper) SessionConfig {
	return SessionConfig{
		SessionCookieName: v.GetString("session_cookie_name"),
		SessionTTL:        v.GetDuration("session_ttl"),
	}
}

func loadSecretsConfig(v *viper.Viper) SecretsConfig {
	return SecretsConfig{
		JWTSecret:  strings.TrimSpace(v.GetString("jwt_secret")),
		SecretsKey: strings.TrimSpace(v.GetString("secrets_key")),
	}
}

// loadUsageCostConfig reads dashboard-only usage cost settings.
func loadUsageCostConfig(v *viper.Viper) (UsageCostConfig, error) {
	input, err := parseNonNegativeFloat(v, "usage_cost_input", "PROMPTGATE_USAGE_COST_INPUT")
	if err != nil {
		return UsageCostConfig{}, err
	}
	output, err := parseNonNegativeFloat(v, "usage_cost_output", "PROMPTGATE_USAGE_COST_OUTPUT")
	if err != nil {
		return UsageCostConfig{}, err
	}
	embedding, err := parseNonNegativeFloat(v, "usage_cost_embedding", "PROMPTGATE_USAGE_COST_EMBEDDING")
	if err != nil {
		return UsageCostConfig{}, err
	}
	return UsageCostConfig{
		Enabled:   v.GetBool("usage_cost_enabled"),
		Input:     input,
		Output:    output,
		Embedding: embedding,
	}, nil
}

// parseNonNegativeFloat parses a non-negative float from a viper key.
func parseNonNegativeFloat(v *viper.Viper, key string, envName string) (float64, error) {
	raw := strings.TrimSpace(v.GetString(key))
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s must be a valid number", envName)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to zero", envName)
	}
	return value, nil
}

func parseCIDRList(raw string, envName string) ([]netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	values := strings.Split(raw, ",")
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("%s contains invalid CIDR %q", envName, value)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func validateOptionalFile(envName string, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("%s is not accessible: %w", envName, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s must be a file", envName)
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
