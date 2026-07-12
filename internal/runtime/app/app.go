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
	Config        config.APIConfig
	Schedule      config.ScheduleConfig
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

type serviceConfig struct {
	Database config.DatabaseURLConfig
	TLS      config.TLSConfig
	Redis    config.RedisConfig
	Secrets  config.SecretsConfig
	Usage    config.UsageCostConfig
}

// New initializes the services and authentication components used by the API.
func New(ctx context.Context, cfg config.APIConfig) (*App, error) {
	application, err := newServices(ctx, serviceConfig{
		Database: cfg.DatabaseURLConfig,
		TLS:      cfg.TLSConfig,
		Redis:    cfg.RedisConfig,
		Secrets:  cfg.SecretsConfig,
		Usage:    cfg.UsageCost,
	}, &cfg)
	if err != nil {
		return nil, err
	}
	application.Config = cfg
	return application, nil
}

// NewSchedule initializes the services used by scheduled background jobs.
func NewSchedule(ctx context.Context, cfg config.ScheduleConfig) (*App, error) {
	application, err := newServices(ctx, serviceConfig{
		Database: cfg.DatabaseURLConfig,
		TLS:      cfg.TLSConfig,
		Redis:    cfg.RedisConfig,
		Secrets:  cfg.SecretsConfig,
	}, nil)
	if err != nil {
		return nil, err
	}
	application.Schedule = cfg
	return application, nil
}

func newServices(ctx context.Context, cfg serviceConfig, apiCfg *config.APIConfig) (*App, error) {
	slog.Info("initializing postgres connection")
	db, err := database.OpenPostgres(ctx, cfg.Database.DatabaseURL)
	if err != nil {
		return nil, err
	}
	slog.Info("postgres connection ready")

	userService := users.NewService(db)
	tokenService := tokens.NewService(db, cfg.Secrets.JWTSecret)
	userService.SetTokenRevoker(tokenService)
	subscriptionService := subscriptions.NewService(db)
	firewallService := firewall.NewService(db)
	faqService := faq.NewService(db)
	groupService := groups.NewService(db)
	secretCipher, err := secrets.NewCipher(cfg.Secrets.SecretsKey)
	if err != nil {
		return nil, err
	}
	providerService := provider.NewService(db, secretCipher)
	pricingService := pricing.NewService(db, providerService)
	mcpService := mcp.NewService(db, secretCipher)
	monitoringService := monitoring.NewService(db)
	setupGuideService := setupguide.NewService(db)
	caHTTPClient, err := platformhttp.NewWithCAFile(cfg.TLS.CAFile, 0)
	if err != nil {
		return nil, fmt.Errorf("initialize CA HTTP client: %w", err)
	}
	if caHTTPClient != nil {
		slog.Info("loaded CA file", "path", cfg.TLS.CAFile)
		monitoringService.SetHTTPClient(&http.Client{
			Transport: caHTTPClient.Transport,
			Timeout:   monitoring.DefaultCheckTimeout,
		})
	}
	proxyService := proxy.NewService(db, proxy.WithUsageCost(proxy.UsageCostConfig{
		Enabled: cfg.Usage.Enabled,
		Rates: proxy.CostRates{
			InputUSDPer1MTokens:     cfg.Usage.Input,
			OutputUSDPer1MTokens:    cfg.Usage.Output,
			EmbeddingUSDPer1MTokens: cfg.Usage.Embedding,
		},
	}), proxy.WithPriceResolver(pricingService))
	slog.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.Redis.RedisURL, cfg.Redis.RedisCacheTTL, slog.Default())
	if err != nil {
		return nil, fmt.Errorf("initialize redis: %w", err)
	}
	var validator *auth.Validator
	initialized := false
	defer func() {
		if initialized {
			return
		}
		if validator != nil {
			validator.Close()
		}
		_ = redisStore.Close()
	}()
	slog.Info("redis connection ready")
	userService.SetNotifier(redisStore)
	tokenService.SetNotifier(redisStore)
	firewallService.SetNotifier(redisStore)
	groupService.SetNotifier(redisStore)
	providerService.SetNotifier(redisStore)
	mcpService.SetNotifier(redisStore)
	subscriptionService.SetNotifier(redisStore)
	quotaRedis := subscriptions.NewRedisStore(redisStore, subscriptionService, cfg.Redis.RedisCacheTTL, slog.Default())
	quotaRedis.SyncVersion(ctx)
	if err := quotaRedis.WarmSnapshot(ctx); err != nil {
		return nil, fmt.Errorf("warm subscription snapshot: %w", err)
	}

	var sessionStore *auth.SessionStore
	var oidcService *auth.OIDCService
	if apiCfg != nil && (apiCfg.KeycloakIssuerURL != "" || apiCfg.KeycloakJWKSURL != "") {
		slog.Info("initializing token validator", "issuer", apiCfg.KeycloakIssuerURL)
		validator, err = auth.NewValidator(ctx, apiCfg.KeycloakIssuerURL, apiCfg.KeycloakJWKSURL, auth.WithValidatorHTTPClient(caHTTPClient))
		if err != nil {
			return nil, err
		}
		slog.Info("token validator ready")

		slog.Info("initializing session store")
		if redisStore.Enabled() {
			sessionStore = auth.NewRedisSessionStore(userService, apiCfg.SessionTTL, redisStore)
		} else {
			sessionStore = auth.NewSessionStore(userService, apiCfg.SessionTTL)
		}
		slog.Info("session store ready")

		slog.Info("initializing OIDC service")
		oidcService, err = auth.NewOIDCService(ctx, *apiCfg, validator, sessionStore, userService, caHTTPClient)
		if err != nil {
			return nil, err
		}
		slog.Info("OIDC service ready")
	} else {
		slog.Info("skipping OIDC initialization")
	}

	application := &App{
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
	}
	initialized = true
	return application, nil
}

// StartBackgroundJobs launches long-running goroutines tied to the context lifetime.
func (a *App) StartBackgroundJobs(ctx context.Context) {
	slog.Info("starting token cleanup goroutine", "interval", a.Schedule.TokenCleanupInterval)
	a.Tokens.StartCleanup(ctx, a.Schedule.TokenCleanupInterval)
	slog.Info("starting user access expiration goroutine", "interval", a.Schedule.UserAccessExpirationInterval)
	a.Users.StartAccessExpiration(ctx, a.Schedule.UserAccessExpirationInterval)
	slog.Info("starting monitoring checker goroutine", "tick", monitoring.DefaultSchedulerTick)
	a.Monitoring.StartScheduler(ctx, monitoring.DefaultSchedulerTick)
	slog.Info("starting raw usage cleanup goroutine", "retention", a.Schedule.UsageRawRetention, "interval", a.Schedule.UsageRawCleanupInterval)
	a.Proxy.StartRawUsageCleanup(ctx, a.Schedule.UsageRawRetention, a.Schedule.UsageRawCleanupInterval)
	if a.Subscriptions != nil && a.QuotaRedis != nil {
		slog.Info("starting subscription quota sync goroutine", "interval", a.Schedule.SubscriptionQuotaSyncInterval)
		a.Subscriptions.StartQuotaStateSync(ctx, a.QuotaRedis, a.Schedule.SubscriptionQuotaSyncInterval)
	}
}
