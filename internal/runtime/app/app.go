package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"gorm.io/gorm"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/faq"
	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/mcp"
	"promptgate/backend/internal/domain/monitoring"
	"promptgate/backend/internal/domain/pricing"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/setupguide"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/database"
	platformhttp "promptgate/backend/internal/platform/httpclient"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/secrets"
)

// App holds all shared resources for the application lifetime.
type App struct {
	Config        config.Config
	DB            *gorm.DB
	Users         *users.Service
	Tokens        *tokens.Service
	Firewall      *firewall.Service
	FAQ           *faq.Service
	Groups        *groups.Service
	Providers     *provider.Service
	MCP           *mcp.Service
	Monitoring    *monitoring.Service
	Pricing       *pricing.Service
	Proxy         *proxy.Service
	Subscriptions *subscriptions.Service
	SetupGuides   *setupguide.Service
	OIDC          *auth.OIDCService
	Validator     *auth.Validator
	Sessions      *auth.SessionStore
	Redis         *redisstore.Store
	QuotaRedis    *subscriptions.RedisStore
}

// New initializes the database, services, and auth components and returns a ready App.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	slog.Info("initializing postgres connection")
	db, err := database.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	slog.Info("postgres connection ready")

	userService := users.NewService(db)
	tokenService := tokens.NewService(db, cfg.JWTSecret)
	userService.SetTokenRevoker(tokenService)
	subscriptionService := subscriptions.NewService(db)
	firewallService := firewall.NewService(db)
	faqService := faq.NewService(db)
	groupService := groups.NewService(db)
	secretCipher, err := secrets.NewCipher(cfg.SecretsKey)
	if err != nil {
		return nil, err
	}
	providerService := provider.NewService(db, secretCipher)
	pricingService := pricing.NewService(db, providerService)
	mcpService := mcp.NewService(db, secretCipher)
	monitoringService := monitoring.NewService(db)
	setupGuideService := setupguide.NewService(db)
	caHTTPClient, err := platformhttp.NewWithCAFile(cfg.CAFile, 0)
	if err != nil {
		return nil, fmt.Errorf("initialize CA HTTP client: %w", err)
	}
	if caHTTPClient != nil {
		slog.Info("loaded CA file", "path", cfg.CAFile)
		monitoringService.SetHTTPClient(&http.Client{
			Transport: caHTTPClient.Transport,
			Timeout:   monitoring.DefaultCheckTimeout,
		})
	}
	proxyService := proxy.NewService(db, proxy.WithUsageCost(proxy.UsageCostConfig{
		Enabled: cfg.UsageCost.Enabled,
		Rates: proxy.CostRates{
			InputUSDPer1MTokens:     cfg.UsageCost.Input,
			OutputUSDPer1MTokens:    cfg.UsageCost.Output,
			EmbeddingUSDPer1MTokens: cfg.UsageCost.Embedding,
		},
	}), proxy.WithPriceResolver(pricingService))
	slog.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.RedisURL, cfg.RedisCacheTTL, slog.Default())
	if err != nil {
		return nil, fmt.Errorf("initialize redis: %w", err)
	}
	slog.Info("redis connection ready")
	userService.SetNotifier(redisStore)
	tokenService.SetNotifier(redisStore)
	firewallService.SetNotifier(redisStore)
	groupService.SetNotifier(redisStore)
	providerService.SetNotifier(redisStore)
	mcpService.SetNotifier(redisStore)
	subscriptionService.SetNotifier(redisStore)
	quotaRedis := subscriptions.NewRedisStore(redisStore, subscriptionService, cfg.RedisCacheTTL, slog.Default())
	quotaRedis.SyncVersion(ctx)
	if err := quotaRedis.WarmSnapshot(ctx); err != nil {
		return nil, fmt.Errorf("warm subscription snapshot: %w", err)
	}

	var validator *auth.Validator
	var sessionStore *auth.SessionStore
	var oidcService *auth.OIDCService
	if cfg.KeycloakIssuerURL != "" || cfg.KeycloakJWKSURL != "" {
		slog.Info("initializing token validator", "issuer", cfg.KeycloakIssuerURL)
		validator, err = auth.NewValidator(ctx, cfg.KeycloakIssuerURL, cfg.KeycloakJWKSURL, auth.WithValidatorHTTPClient(caHTTPClient))
		if err != nil {
			return nil, err
		}
		slog.Info("token validator ready")

		slog.Info("initializing session store")
		if redisStore.Enabled() {
			sessionStore = auth.NewRedisSessionStore(userService, cfg.SessionTTL, redisStore)
		} else {
			sessionStore = auth.NewSessionStore(userService, cfg.SessionTTL)
		}
		slog.Info("session store ready")

		slog.Info("initializing OIDC service")
		oidcService, err = auth.NewOIDCService(ctx, cfg, validator, sessionStore, userService, caHTTPClient)
		if err != nil {
			return nil, err
		}
		slog.Info("OIDC service ready")
	} else {
		slog.Info("skipping OIDC initialization")
	}

	return &App{
		Config:        cfg,
		DB:            db,
		Users:         userService,
		Tokens:        tokenService,
		Firewall:      firewallService,
		FAQ:           faqService,
		Groups:        groupService,
		Providers:     providerService,
		MCP:           mcpService,
		Monitoring:    monitoringService,
		Pricing:       pricingService,
		Proxy:         proxyService,
		Subscriptions: subscriptionService,
		SetupGuides:   setupGuideService,
		OIDC:          oidcService,
		Validator:     validator,
		Sessions:      sessionStore,
		Redis:         redisStore,
		QuotaRedis:    quotaRedis,
	}, nil
}

// StartBackgroundJobs launches long-running goroutines tied to the context lifetime.
func (a *App) StartBackgroundJobs(ctx context.Context) {
	slog.Info("starting token cleanup goroutine", "interval", a.Config.TokenCleanupInterval)
	a.Tokens.StartCleanup(ctx, a.Config.TokenCleanupInterval)
	slog.Info("starting user access expiration goroutine", "interval", a.Config.UserAccessExpirationInterval)
	a.Users.StartAccessExpiration(ctx, a.Config.UserAccessExpirationInterval)
	slog.Info("starting monitoring checker goroutine", "tick", monitoring.DefaultSchedulerTick)
	a.Monitoring.StartScheduler(ctx, monitoring.DefaultSchedulerTick)
	slog.Info("starting raw usage cleanup goroutine", "retention", a.Config.UsageRawRetention, "interval", a.Config.UsageRawCleanupInterval)
	a.Proxy.StartRawUsageCleanup(ctx, a.Config.UsageRawRetention, a.Config.UsageRawCleanupInterval)
	if a.Subscriptions != nil && a.QuotaRedis != nil {
		slog.Info("starting subscription quota sync goroutine", "interval", a.Config.SubscriptionQuotaSyncInterval)
		a.Subscriptions.StartQuotaStateSync(ctx, a.QuotaRedis, a.Config.SubscriptionQuotaSyncInterval)
	}
}
