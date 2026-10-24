package modellab

import (
	"context"
	"net/http"
	"time"

	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

const (
	TicketPrefix           = "pgml_"
	MaxTargets             = 3
	defaultMaxConcurrent   = 3
	defaultMaxOutputTokens = 4096
	defaultCatalogTTL      = 5 * time.Minute
	defaultCatalogLKG      = 24 * time.Hour
	defaultTicketTTL       = time.Minute
	defaultLeaseTTL        = 15 * time.Minute
	defaultConcurrency     = 4
)

type Protocol string

const (
	ProtocolOpenAIChat        Protocol = "openai-chat"
	ProtocolAnthropicMessages Protocol = "anthropic-messages"
)

type CatalogState string

const (
	CatalogFresh       CatalogState = "fresh"
	CatalogStale       CatalogState = "stale"
	CatalogUnavailable CatalogState = "unavailable"
)

type PriceSource string

const (
	PriceExact       PriceSource = "exact"
	PriceFallback    PriceSource = "fallback"
	PriceUnavailable PriceSource = "unavailable"
)

type AvailabilityState string

const (
	AvailabilityUnknown     AvailabilityState = "unknown"
	AvailabilityAvailable   AvailabilityState = "available"
	AvailabilityDegraded    AvailabilityState = "degraded"
	AvailabilityUnavailable AvailabilityState = "unavailable"
)

type CatalogResponse struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	Limits      ModelLabLimits    `json:"limits"`
	Providers   []CatalogProvider `json:"providers"`
}

type ModelLabLimits struct {
	MaxTargets      int   `json:"maxTargets"`
	MaxPromptBytes  int64 `json:"maxPromptBytes"`
	MaxOutputTokens int   `json:"maxOutputTokens"`
}

type CatalogProvider struct {
	Name        string                `json:"name"`
	DisplayName string                `json:"displayName"`
	Type        provider.ProviderType `json:"type"`
	Catalog     CatalogStatus         `json:"catalog"`
	Models      []CatalogModel        `json:"models"`
}

type CatalogStatus struct {
	State     CatalogState `json:"state"`
	FetchedAt *time.Time   `json:"fetchedAt"`
}

type CatalogModel struct {
	ID                   string               `json:"id"`
	DisplayName          *string              `json:"displayName"`
	Pricing              ModelPricing         `json:"pricing"`
	Playground           PlaygroundCapability `json:"playground"`
	ObservedAvailability ObservedAvailability `json:"observedAvailability"`
}

type ModelPricing struct {
	Currency string      `json:"currency"`
	Unit     string      `json:"unit"`
	Input    *float64    `json:"input"`
	Output   *float64    `json:"output"`
	Source   PriceSource `json:"source"`
}

type PlaygroundCapability struct {
	Supported bool     `json:"supported"`
	Protocol  Protocol `json:"protocol"`
}

type ObservedAvailability struct {
	State          AvailabilityState `json:"state"`
	WindowSeconds  int64             `json:"windowSeconds"`
	SampleSize     int64             `json:"sampleSize"`
	SuccessRate    *float64          `json:"successRate"`
	TTFTP50MS      *int64            `json:"ttftP50Ms"`
	DurationP50MS  *int64            `json:"durationP50Ms"`
	LastObservedAt *time.Time        `json:"lastObservedAt"`
}

type AccessFilter struct {
	ProviderAllowed func(providerName string) bool
	ModelAllowed    func(providerName, model string) bool
}

func (f AccessFilter) allowsProvider(providerName string) bool {
	return f.ProviderAllowed == nil || f.ProviderAllowed(providerName)
}

func (f AccessFilter) allowsModel(providerName, model string) bool {
	return f.ModelAllowed == nil || f.ModelAllowed(providerName, model)
}

type TicketTarget struct {
	ProviderName string `json:"providerName"`
	Model        string `json:"model"`
}

type IssueTicketsInput struct {
	Targets []TicketTarget `json:"targets"`
}

type IssueTicketsResponse struct {
	ComparisonID string         `json:"comparisonId"`
	ExpiresAt    time.Time      `json:"expiresAt"`
	Tickets      []IssuedTicket `json:"tickets"`
}

type IssuedTicket struct {
	ProviderName string   `json:"providerName"`
	Model        string   `json:"model"`
	Protocol     Protocol `json:"protocol"`
	Method       string   `json:"method"`
	Endpoint     string   `json:"endpoint"`
	Token        string   `json:"token"`
}

type TicketClaims struct {
	jwt.RegisteredClaims
	Role         string   `json:"role"`
	ProviderName string   `json:"providerName"`
	Model        string   `json:"model"`
	Protocol     Protocol `json:"protocol"`
	Operation    string   `json:"operation"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	Origin       string   `json:"origin,omitempty"`
	ComparisonID string   `json:"comparisonId"`
}

type ConsumeTicketInput struct {
	Token  string
	Method string
	Path   string
	Origin string
}

type Options struct {
	TicketSecret string
	// ProxyBaseURL is the browser-visible proxy base. Any path is a public
	// routing prefix that the edge must remove before the request reaches the
	// proxy; tickets remain bound to the canonical path observed by the proxy.
	ProxyBaseURL       string
	TicketTTL          time.Duration
	LeaseTTL           time.Duration
	CatalogFreshFor    time.Duration
	CatalogLastGoodFor time.Duration
	CatalogConcurrency int
	MaxTargets         int
	MaxConcurrent      int
	MaxPromptBytes     int64
	MaxOutputTokens    int
	HTTPClient         *http.Client
	Now                func() time.Time
}

// ProviderRepository is the minimum provider surface required by Model Lab.
type ProviderRepository interface {
	ListEnabled(ctx context.Context) ([]provider.Provider, error)
	DecryptAPIKey(record provider.Provider) (string, error)
}

// PricingRepository is the minimum pricing surface required by Model Lab.
type PricingRepository interface {
	Config(ctx context.Context) (pricing.ConfigResponse, error)
}

// RedisStore is the Redis surface used for last-known-good catalogs, tickets,
// and concurrency leases.
type RedisStore interface {
	Enabled() bool
	GetJSON(ctx context.Context, key string, dst any) (bool, error)
	GetDelJSON(ctx context.Context, key string, dst any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Client() *redis.Client
}
