package modellab

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidConfiguration = errors.New("invalid_model_lab_configuration")
	ErrInvalidTargets       = errors.New("invalid_model_lab_targets")
	ErrTargetNotAllowed     = errors.New("model_lab_target_not_allowed")
	ErrCatalogUnavailable   = errors.New("model_lab_catalog_unavailable")
	ErrTicketUnavailable    = errors.New("model_lab_ticket_unavailable")
	ErrTicketIssueLimit     = errors.New("model_lab_ticket_issue_limit")
	ErrInvalidTicket        = errors.New("invalid_playground_ticket")
	ErrInvalidPayload       = errors.New("invalid_model_lab_payload")
	ErrParallelLimit        = errors.New("model_lab_parallel_limit")
)

type Service struct {
	providers ProviderRepository
	pricing   PricingRepository
	store     RedisStore

	ticketSecret       []byte
	proxyBaseURL       string
	ticketTTL          time.Duration
	leaseTTL           time.Duration
	catalogFreshFor    time.Duration
	catalogLastGoodFor time.Duration
	catalogConcurrency int
	maxTargets         int
	maxConcurrent      int
	maxOutstanding     int
	maxPromptBytes     int64
	maxOutputTokens    int
	httpClient         *http.Client
	now                func() time.Time

	flightsMu sync.Mutex
	flights   map[string]*catalogFlight
}

type catalogFlight struct {
	done   chan struct{}
	result cachedCatalog
	err    error
}

func NewService(providers ProviderRepository, prices PricingRepository, store RedisStore, options Options) (*Service, error) {
	if providers == nil || store == nil || !store.Enabled() {
		return nil, ErrInvalidConfiguration
	}
	secret := strings.TrimSpace(options.TicketSecret)
	if len(secret) < 32 {
		return nil, ErrInvalidConfiguration
	}
	baseURL := strings.TrimRight(strings.TrimSpace(options.ProxyBaseURL), "/")
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil || (parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https") ||
		parsedBaseURL.Host == "" || parsedBaseURL.User != nil || parsedBaseURL.RawPath != "" ||
		parsedBaseURL.EscapedPath() != parsedBaseURL.Path ||
		(parsedBaseURL.Path != "" && path.Clean(parsedBaseURL.Path) != parsedBaseURL.Path) ||
		parsedBaseURL.RawQuery != "" || parsedBaseURL.ForceQuery ||
		parsedBaseURL.Fragment != "" {
		return nil, ErrInvalidConfiguration
	}

	ticketTTL := options.TicketTTL
	if ticketTTL <= 0 {
		ticketTTL = defaultTicketTTL
	}
	leaseTTL := options.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = defaultLeaseTTL
	}
	freshFor := options.CatalogFreshFor
	if freshFor <= 0 {
		freshFor = defaultCatalogTTL
	}
	lastGoodFor := options.CatalogLastGoodFor
	if lastGoodFor < freshFor {
		lastGoodFor = defaultCatalogLKG
	}
	concurrency := options.CatalogConcurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	maxTargets := options.MaxTargets
	if maxTargets <= 0 {
		maxTargets = MaxTargets
	}
	if maxTargets > MaxTargets {
		return nil, ErrInvalidConfiguration
	}
	maxConcurrent := options.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}
	if maxConcurrent > MaxTargets {
		return nil, ErrInvalidConfiguration
	}
	maxPromptBytes := options.MaxPromptBytes
	if maxPromptBytes <= 0 {
		maxPromptBytes = DefaultMaxRequestBytes
	}
	maxOutputTokens := options.MaxOutputTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Service{
		providers:          providers,
		pricing:            prices,
		store:              store,
		ticketSecret:       []byte(secret),
		proxyBaseURL:       baseURL,
		ticketTTL:          ticketTTL,
		leaseTTL:           leaseTTL,
		catalogFreshFor:    freshFor,
		catalogLastGoodFor: lastGoodFor,
		catalogConcurrency: concurrency,
		maxTargets:         maxTargets,
		maxConcurrent:      maxConcurrent,
		maxOutstanding:     maxConcurrent * maxTargets,
		maxPromptBytes:     maxPromptBytes,
		maxOutputTokens:    maxOutputTokens,
		httpClient:         client,
		now:                now,
		flights:            map[string]*catalogFlight{},
	}, nil
}
