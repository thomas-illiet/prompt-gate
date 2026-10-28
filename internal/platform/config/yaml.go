package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"promptgate/backend/internal/platform/proxylimits"
)

type durationValue time.Duration

func (d *durationValue) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("duration must be a string using Go duration syntax")
	}
	value, err := time.ParseDuration(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	*d = durationValue(value)
	return nil
}

func duration(value time.Duration) durationValue {
	return durationValue(value)
}

type portValue string

func (p *portValue) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return errors.New("port must be an integer or string")
	}
	switch node.Tag {
	case "!!int":
		value, err := strconv.Atoi(node.Value)
		if err != nil || value <= 0 || value > 65535 {
			return fmt.Errorf("invalid port %q", node.Value)
		}
		*p = portValue(strconv.Itoa(value))
	case "!!str":
		value := strings.TrimSpace(node.Value)
		port := strings.TrimPrefix(value, ":")
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed <= 0 || parsed > 65535 {
			return fmt.Errorf("invalid port %q", node.Value)
		}
		*p = portValue(value)
	default:
		return errors.New("port must be an integer or string")
	}
	return nil
}

type fileConfig struct {
	Common   commonFileConfig   `yaml:"common"`
	API      apiFileConfig      `yaml:"api"`
	Proxy    proxyFileConfig    `yaml:"proxy"`
	Worker   workerFileConfig   `yaml:"worker"`
	Schedule scheduleFileConfig `yaml:"schedule"`
	Migrate  migrateFileConfig  `yaml:"migrate"`
}

type commonFileConfig struct {
	Environment         string                `yaml:"environment"`
	Log                 logFileConfig         `yaml:"log"`
	Database            databaseFileConfig    `yaml:"database"`
	Redis               redisFileConfig       `yaml:"redis"`
	TLS                 tlsFileConfig         `yaml:"tls"`
	Security            securityFileConfig    `yaml:"security"`
	PublicURLs          publicURLsFileConfig  `yaml:"public_urls"`
	Session             sessionFileConfig     `yaml:"session"`
	CORS                corsFileConfig        `yaml:"cors"`
	ModelLab            modelLabFileConfig    `yaml:"model_lab"`
	UsageCost           usageCostFileConfig   `yaml:"usage_cost"`
	Maintenance         maintenanceFileConfig `yaml:"maintenance"`
	ProxyReloadDebounce durationValue         `yaml:"proxy_reload_debounce"`
}

type logFileConfig struct {
	Level string `yaml:"level"`
}

type databaseFileConfig struct {
	URL                   string        `yaml:"url"`
	ConnectionMaxLifetime durationValue `yaml:"connection_max_lifetime"`
	MaxIdleConnections    int           `yaml:"max_idle_connections"`
	MaxOpenConnections    int           `yaml:"max_open_connections"`
}

type redisFileConfig struct {
	URL                string         `yaml:"url"`
	CacheTTL           durationValue  `yaml:"cache_ttl"`
	PoolSize           *int           `yaml:"pool_size"`
	MinIdleConnections *int           `yaml:"min_idle_connections"`
	PoolTimeout        *durationValue `yaml:"pool_timeout"`
	ReadTimeout        *durationValue `yaml:"read_timeout"`
	WriteTimeout       *durationValue `yaml:"write_timeout"`
}

type tlsFileConfig struct {
	CAFile string `yaml:"ca_file"`
}

type securityFileConfig struct {
	JWTSecret  string `yaml:"jwt_secret"`
	SecretsKey string `yaml:"secrets_key"`
}

type publicURLsFileConfig struct {
	Frontend string `yaml:"frontend"`
	Backend  string `yaml:"backend"`
	Proxy    string `yaml:"proxy"`
}

type sessionFileConfig struct {
	CookieName string        `yaml:"cookie_name"`
	TTL        durationValue `yaml:"ttl"`
}

type corsFileConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type modelLabFileConfig struct {
	TicketTTL       durationValue `yaml:"ticket_ttl"`
	MaxPromptBytes  int64         `yaml:"max_prompt_bytes"`
	MaxOutputTokens int           `yaml:"max_output_tokens"`
	MaxTargets      int           `yaml:"max_targets"`
	MaxConcurrent   int           `yaml:"max_concurrent"`
	CatalogFreshTTL durationValue `yaml:"catalog_fresh_ttl"`
	CatalogStaleTTL durationValue `yaml:"catalog_stale_ttl"`
}

type usageCostFileConfig struct {
	Input     float64 `yaml:"input"`
	Output    float64 `yaml:"output"`
	Embedding float64 `yaml:"embedding"`
}

type maintenanceFileConfig struct {
	TokenCleanupInterval           durationValue `yaml:"token_cleanup_interval"`
	UserAccessExpirationInterval   durationValue `yaml:"user_access_expiration_interval"`
	QuotaResetDistributionInterval durationValue `yaml:"quota_reset_distribution_interval"`
}

type apiFileConfig struct {
	Port              portValue      `yaml:"port"`
	StaticAssetsDir   string         `yaml:"static_assets_dir"`
	AdminAPIKey       string         `yaml:"admin_api_key"`
	AdminContactEmail string         `yaml:"admin_contact_email"`
	OIDC              oidcFileConfig `yaml:"oidc"`
}

type oidcFileConfig struct {
	IssuerURL    string `yaml:"issuer_url"`
	JWKSURL      string `yaml:"jwks_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
}

type proxyFileConfig struct {
	Port                     portValue      `yaml:"port"`
	BasePath                 string         `yaml:"base_path"`
	TrustForwardHeaders      bool           `yaml:"trust_forward_headers"`
	TrustedProxies           []string       `yaml:"trusted_proxies"`
	ClientIPDebug            bool           `yaml:"client_ip_debug"`
	DebugRequests            bool           `yaml:"debug_requests"`
	MaxBufferedRequestBytes  int64          `yaml:"max_buffered_request_bytes"`
	MaxBufferedResponseBytes int64          `yaml:"max_buffered_response_bytes"`
	UpstreamTimeout          durationValue  `yaml:"upstream_timeout"`
	OPA                      opaFileConfig  `yaml:"opa"`
	OTel                     otelFileConfig `yaml:"otel"`
}

type opaFileConfig struct {
	Enabled    bool          `yaml:"enabled"`
	URL        string        `yaml:"url"`
	PolicyPath string        `yaml:"policy_path"`
	Timeout    durationValue `yaml:"timeout"`
	CacheTTL   durationValue `yaml:"cache_ttl"`
}

type otelFileConfig struct {
	Enabled            bool          `yaml:"enabled"`
	Endpoint           string        `yaml:"endpoint"`
	APIKey             string        `yaml:"api_key"`
	ProjectName        string        `yaml:"project_name"`
	ServiceName        string        `yaml:"service_name"`
	Environment        string        `yaml:"environment"`
	CapturePrompts     bool          `yaml:"capture_prompts"`
	CaptureOutput      bool          `yaml:"capture_output"`
	CaptureThinking    bool          `yaml:"capture_thinking"`
	ExportTimeout      durationValue `yaml:"export_timeout"`
	BatchTimeout       durationValue `yaml:"batch_timeout"`
	MaxQueueSize       int           `yaml:"max_queue_size"`
	MaxExportBatchSize int           `yaml:"max_export_batch_size"`
	Insecure           bool          `yaml:"insecure"`
}

type workerFileConfig struct {
	BatchSize          int64         `yaml:"batch_size"`
	BlockTimeout       durationValue `yaml:"block_timeout"`
	PendingIdleTimeout durationValue `yaml:"pending_idle_timeout"`
	ConsumerName       string        `yaml:"consumer_name"`
}

type scheduleFileConfig struct {
	UsageRawRetention             durationValue `yaml:"usage_raw_retention"`
	UsageRawCleanupInterval       durationValue `yaml:"usage_raw_cleanup_interval"`
	SubscriptionQuotaSyncInterval durationValue `yaml:"subscription_quota_sync_interval"`
}

type migrateFileConfig struct{}

var (
	durationValueType = reflect.TypeOf(durationValue(0))
	portValueType     = reflect.TypeOf(portValue(""))
)

func validateYAMLTypes(node *yaml.Node, target reflect.Type, configPath string) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.AliasNode {
		if node.Alias == nil {
			return yamlTypeError(configPath, "a valid YAML value", node)
		}
		return validateYAMLTypes(node.Alias, target, configPath)
	}
	if node.ShortTag() == "!!null" {
		if target.Kind() == reflect.Pointer {
			return nil
		}
		return fmt.Errorf("%s must not be null (line %d)", yamlPath(configPath), node.Line)
	}
	if target.Kind() == reflect.Pointer {
		target = target.Elem()
	}

	switch target {
	case durationValueType:
		return requireYAMLScalarTag(node, configPath, "!!str", "a duration string")
	case portValueType:
		if node.Kind != yaml.ScalarNode || (node.ShortTag() != "!!int" && node.ShortTag() != "!!str") {
			return yamlTypeError(configPath, "an integer or string port", node)
		}
		return nil
	}

	switch target.Kind() {
	case reflect.Struct:
		if node.Kind != yaml.MappingNode {
			return yamlTypeError(configPath, "a mapping", node)
		}
		fields := make(map[string]reflect.Type, target.NumField())
		for index := 0; index < target.NumField(); index++ {
			field := target.Field(index)
			name := strings.Split(field.Tag.Get("yaml"), ",")[0]
			if name == "" {
				name = strings.ToLower(field.Name)
			}
			if name != "-" {
				fields[name] = field.Type
			}
		}
		for index := 0; index+1 < len(node.Content); index += 2 {
			key, value := node.Content[index], node.Content[index+1]
			if key.Value == "<<" {
				if err := validateYAMLMergeTypes(value, target, configPath); err != nil {
					return err
				}
				continue
			}
			fieldType, ok := fields[key.Value]
			if !ok {
				continue
			}
			if err := validateYAMLTypes(value, fieldType, joinYAMLPath(configPath, key.Value)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if node.Kind != yaml.SequenceNode {
			return yamlTypeError(configPath, "a sequence", node)
		}
		for index, item := range node.Content {
			itemPath := fmt.Sprintf("%s[%d]", yamlPath(configPath), index)
			if err := validateYAMLTypes(item, target.Elem(), itemPath); err != nil {
				return err
			}
		}
		return nil
	case reflect.String:
		return requireYAMLScalarTag(node, configPath, "!!str", "a string")
	case reflect.Bool:
		return requireYAMLScalarTag(node, configPath, "!!bool", "a boolean")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return requireYAMLScalarTag(node, configPath, "!!int", "an integer")
	case reflect.Float32, reflect.Float64:
		if node.Kind != yaml.ScalarNode || (node.ShortTag() != "!!int" && node.ShortTag() != "!!float") {
			return yamlTypeError(configPath, "a number", node)
		}
		return nil
	default:
		return nil
	}
}

func validateYAMLMergeTypes(node *yaml.Node, target reflect.Type, configPath string) error {
	if node.Kind == yaml.AliasNode {
		if node.Alias == nil {
			return yamlTypeError(configPath, "a valid YAML merge", node)
		}
		return validateYAMLMergeTypes(node.Alias, target, configPath)
	}
	if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			if err := validateYAMLMergeTypes(item, target, configPath); err != nil {
				return err
			}
		}
		return nil
	}
	return validateYAMLTypes(node, target, configPath)
}

func requireYAMLScalarTag(node *yaml.Node, configPath string, tag string, expected string) error {
	if node.Kind != yaml.ScalarNode || node.ShortTag() != tag {
		return yamlTypeError(configPath, expected, node)
	}
	return nil
}

func yamlTypeError(configPath string, expected string, node *yaml.Node) error {
	return fmt.Errorf("%s must be %s, got %s (line %d)", yamlPath(configPath), expected, node.ShortTag(), node.Line)
}

func yamlPath(configPath string) string {
	if configPath == "" {
		return "configuration"
	}
	return configPath
}

func joinYAMLPath(parent string, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func defaultFileConfig() fileConfig {
	return fileConfig{
		Common: commonFileConfig{
			Log:      logFileConfig{Level: "info"},
			Database: databaseFileConfig{ConnectionMaxLifetime: duration(30 * time.Minute), MaxIdleConnections: 5, MaxOpenConnections: 10},
			Redis:    redisFileConfig{CacheTTL: duration(5 * time.Minute)},
			Session:  sessionFileConfig{CookieName: "promptgate_session", TTL: duration(8 * time.Hour)},
			ModelLab: modelLabFileConfig{
				TicketTTL:       duration(time.Minute),
				MaxPromptBytes:  32 * 1024,
				MaxOutputTokens: 4096,
				MaxTargets:      3,
				MaxConcurrent:   3,
				CatalogFreshTTL: duration(5 * time.Minute),
				CatalogStaleTTL: duration(24 * time.Hour),
			},
			UsageCost: usageCostFileConfig{Input: 5, Output: 30, Embedding: 0.02},
			Maintenance: maintenanceFileConfig{
				TokenCleanupInterval:           duration(time.Hour),
				UserAccessExpirationInterval:   duration(time.Hour),
				QuotaResetDistributionInterval: duration(time.Hour),
			},
			ProxyReloadDebounce: duration(250 * time.Millisecond),
		},
		API: apiFileConfig{Port: "8080"},
		Proxy: proxyFileConfig{
			Port:                     "8081",
			MaxBufferedRequestBytes:  proxylimits.DefaultMaxBufferedRequestBytes,
			MaxBufferedResponseBytes: proxylimits.DefaultMaxBufferedResponseBytes,
			UpstreamTimeout:          duration(proxylimits.DefaultUpstreamTimeout),
			OPA: opaFileConfig{
				Enabled:    true,
				PolicyPath: "promptgate/proxy/decision",
				Timeout:    duration(2 * time.Second),
				CacheTTL:   duration(10 * time.Minute),
			},
			OTel: otelFileConfig{
				ProjectName:        "prompt-gate",
				ServiceName:        "promptgate-proxy",
				Environment:        "production",
				ExportTimeout:      duration(10 * time.Second),
				BatchTimeout:       duration(5 * time.Second),
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
		},
		Worker: workerFileConfig{
			BatchSize:          100,
			BlockTimeout:       duration(5 * time.Second),
			PendingIdleTimeout: duration(30 * time.Second),
		},
		Schedule: scheduleFileConfig{
			UsageRawRetention:             duration(90 * 24 * time.Hour),
			UsageRawCleanupInterval:       duration(time.Hour),
			SubscriptionQuotaSyncInterval: duration(5 * time.Minute),
		},
	}
}

func loadFile(configPath string) (fileConfig, error) {
	if strings.TrimSpace(configPath) == "" {
		return fileConfig{}, errors.New("--config is required")
	}
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return fileConfig{}, fmt.Errorf("resolve config path %q: %w", configPath, err)
	}
	file, err := os.Open(absPath)
	if err != nil {
		return fileConfig{}, fmt.Errorf("open config %s: %w", absPath, err)
	}
	defer file.Close()
	var document yaml.Node
	nodeDecoder := yaml.NewDecoder(file)
	if err := nodeDecoder.Decode(&document); err != nil {
		if errors.Is(err, io.EOF) {
			return fileConfig{}, fmt.Errorf("config %s is empty", absPath)
		}
		return fileConfig{}, fmt.Errorf("decode config %s: %w", absPath, err)
	}
	if len(document.Content) == 0 {
		return fileConfig{}, fmt.Errorf("config %s is empty", absPath)
	}
	if err := validateYAMLTypes(document.Content[0], reflect.TypeOf(fileConfig{}), ""); err != nil {
		return fileConfig{}, fmt.Errorf("decode config %s: %w", absPath, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fileConfig{}, fmt.Errorf("rewind config %s: %w", absPath, err)
	}

	cfg := defaultFileConfig()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return fileConfig{}, fmt.Errorf("config %s is empty", absPath)
		}
		return fileConfig{}, fmt.Errorf("decode config %s: %w", absPath, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fileConfig{}, fmt.Errorf("decode config %s: %w", absPath, err)
		}
		return fileConfig{}, fmt.Errorf("config %s must contain exactly one YAML document", absPath)
	}

	baseDir := filepath.Dir(absPath)
	cfg.Common.TLS.CAFile = resolveFilePath(baseDir, cfg.Common.TLS.CAFile)
	cfg.API.StaticAssetsDir = resolveFilePath(baseDir, cfg.API.StaticAssetsDir)
	if _, err := parseEnvironment(cfg.Common.Environment); err != nil {
		return fileConfig{}, err
	}
	return cfg, nil
}

func resolveFilePath(baseDir string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func optionalInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func optionalDuration(value *durationValue) time.Duration {
	if value == nil {
		return 0
	}
	return time.Duration(*value)
}

func (cfg fileConfig) logConfig() LogConfig {
	return LogConfig{LogLevel: strings.TrimSpace(cfg.Common.Log.Level)}
}

func (cfg fileConfig) environmentConfig() EnvironmentConfig {
	environment, _ := parseEnvironment(cfg.Common.Environment)
	return EnvironmentConfig{Environment: environment}
}

func (cfg fileConfig) databaseConfig() DatabaseURLConfig {
	return DatabaseURLConfig{
		DatabaseURL:        strings.TrimSpace(cfg.Common.Database.URL),
		ConnMaxLifetime:    time.Duration(cfg.Common.Database.ConnectionMaxLifetime),
		MaxIdleConnections: cfg.Common.Database.MaxIdleConnections,
		MaxOpenConnections: cfg.Common.Database.MaxOpenConnections,
	}
}

func (cfg fileConfig) redisConfig() RedisConfig {
	return RedisConfig{
		RedisURL:                strings.TrimSpace(cfg.Common.Redis.URL),
		RedisCacheTTL:           time.Duration(cfg.Common.Redis.CacheTTL),
		RedisPoolSize:           optionalInt(cfg.Common.Redis.PoolSize),
		RedisMinIdleConnections: optionalInt(cfg.Common.Redis.MinIdleConnections),
		RedisPoolTimeout:        optionalDuration(cfg.Common.Redis.PoolTimeout),
		RedisReadTimeout:        optionalDuration(cfg.Common.Redis.ReadTimeout),
		RedisWriteTimeout:       optionalDuration(cfg.Common.Redis.WriteTimeout),
	}
}

func (cfg fileConfig) tlsConfig() TLSConfig {
	return TLSConfig{CAFile: cfg.Common.TLS.CAFile}
}

func (cfg fileConfig) secretsConfig() SecretsConfig {
	return SecretsConfig{
		JWTSecret:  strings.TrimSpace(cfg.Common.Security.JWTSecret),
		SecretsKey: strings.TrimSpace(cfg.Common.Security.SecretsKey),
	}
}

func (cfg fileConfig) publicURLConfig() PublicURLConfig {
	return PublicURLConfig{
		FrontendBaseURL: strings.TrimRight(strings.TrimSpace(cfg.Common.PublicURLs.Frontend), "/"),
		BackendBaseURL:  strings.TrimRight(strings.TrimSpace(cfg.Common.PublicURLs.Backend), "/"),
		ProxyBaseURL:    strings.TrimRight(strings.TrimSpace(cfg.Common.PublicURLs.Proxy), "/"),
	}
}

func (cfg fileConfig) sessionConfig() SessionConfig {
	return SessionConfig{
		SessionCookieName: strings.TrimSpace(cfg.Common.Session.CookieName),
		SessionTTL:        time.Duration(cfg.Common.Session.TTL),
	}
}

func (cfg fileConfig) modelLabConfig() ModelLabConfig {
	return ModelLabConfig{
		TicketTTL:       time.Duration(cfg.Common.ModelLab.TicketTTL),
		MaxPromptBytes:  cfg.Common.ModelLab.MaxPromptBytes,
		MaxOutputTokens: cfg.Common.ModelLab.MaxOutputTokens,
		MaxTargets:      cfg.Common.ModelLab.MaxTargets,
		MaxConcurrent:   cfg.Common.ModelLab.MaxConcurrent,
		CatalogFreshTTL: time.Duration(cfg.Common.ModelLab.CatalogFreshTTL),
		CatalogStaleTTL: time.Duration(cfg.Common.ModelLab.CatalogStaleTTL),
	}
}

func (cfg fileConfig) usageCostConfig() (UsageCostConfig, error) {
	values := []struct {
		path  string
		value float64
	}{
		{"common.usage_cost.input", cfg.Common.UsageCost.Input},
		{"common.usage_cost.output", cfg.Common.UsageCost.Output},
		{"common.usage_cost.embedding", cfg.Common.UsageCost.Embedding},
	}
	for _, item := range values {
		if err := validateNonNegativeFloat(item.value, item.path); err != nil {
			return UsageCostConfig{}, err
		}
	}
	return UsageCostConfig{
		Input:     cfg.Common.UsageCost.Input,
		Output:    cfg.Common.UsageCost.Output,
		Embedding: cfg.Common.UsageCost.Embedding,
	}, nil
}

func validateDatabaseConfig(cfg DatabaseURLConfig) error {
	if cfg.DatabaseURL == "" {
		return errors.New("common.database.url is required")
	}
	if cfg.ConnMaxLifetime <= 0 {
		return errors.New("common.database.connection_max_lifetime must be greater than zero")
	}
	if cfg.MaxIdleConnections <= 0 {
		return errors.New("common.database.max_idle_connections must be greater than zero")
	}
	if cfg.MaxOpenConnections <= 0 {
		return errors.New("common.database.max_open_connections must be greater than zero")
	}
	if cfg.MaxIdleConnections > cfg.MaxOpenConnections {
		return errors.New("common.database.max_idle_connections must not exceed common.database.max_open_connections")
	}
	return nil
}

func validateRedisConfig(cfg RedisConfig) error {
	if cfg.RedisURL == "" {
		return errors.New("common.redis.url is required")
	}
	if cfg.RedisCacheTTL <= 0 {
		return errors.New("common.redis.cache_ttl must be greater than zero")
	}
	return nil
}

func validateRedisOverrides(cfg redisFileConfig) error {
	if cfg.PoolSize != nil && *cfg.PoolSize <= 0 {
		return errors.New("common.redis.pool_size must be greater than zero when configured")
	}
	if cfg.MinIdleConnections != nil && *cfg.MinIdleConnections <= 0 {
		return errors.New("common.redis.min_idle_connections must be greater than zero when configured")
	}
	values := []struct {
		path  string
		value *durationValue
	}{
		{"common.redis.pool_timeout", cfg.PoolTimeout},
		{"common.redis.read_timeout", cfg.ReadTimeout},
		{"common.redis.write_timeout", cfg.WriteTimeout},
	}
	for _, item := range values {
		if item.value != nil && time.Duration(*item.value) <= 0 {
			return fmt.Errorf("%s must be greater than zero when configured", item.path)
		}
	}
	return nil
}
