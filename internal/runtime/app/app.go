package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"gorm.io/gorm"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/faq"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/modellab"
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
	FAQ           *faq.Service
	Groups        *groups.Service
	Providers     *provider.Service
	Monitoring    *monitoring.Service
	ModelLab      *modellab.Service
	Pricing       *pricing.Service
	Proxy         *proxy.Service
	Subscriptions *subscriptions.Service
	SetupGuides   *setupguide.Service
	OIDC          *auth.OIDCService
	Validator     *auth.Validator
	Sessions      *auth.SessionStore
	Redis         *redisstore.Store
	QuotaRedis    *subscriptions.RedisStore

	subscriptionWatcherOnce sync.Once
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
	db, err := database.OpenPostgres(ctx, cfg.Database.DatabaseURL, database.PoolOptions{
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		MaxIdleConns:    cfg.Database.MaxIdleConnections,
		MaxOpenConns:    cfg.Database.MaxOpenConnections,
	})
	if err != nil {
		return nil, err
	}
	slog.Info("postgres connection ready")

	userService := users.NewService(db)
	tokenService := tokens.NewService(db, cfg.Secrets.JWTSecret)
	userService.SetTokenRevoker(tokenService)
	subscriptionService := subscriptions.NewService(db)
	faqService := faq.NewService(db)
	groupService := groups.NewService(db)
	secretCipher, err := secrets.NewCipher(cfg.Secrets.SecretsKey)
	if err != nil {
		return nil, err
	}
	providerService := provider.NewService(db, secretCipher)
	pricingService := pricing.NewService(db, providerService)
	monitoringService := monitoring.NewService(db)
	setupGuideService := setupguide.NewService(db)
	caHTTPClient, err := platformhttp.NewWithCAFile(cfg.TLS.CAFile, 0)
	if err != nil {
		return nil, fmt.Errorf("initialize CA HTTP client: %w", err)
	}
	providerModelHTTPClient := &http.Client{Timeout: 3 * time.Second}
	if caHTTPClient != nil {
		slog.Info("loaded CA file", "path", cfg.TLS.CAFile)
		providerModelHTTPClient.Transport = caHTTPClient.Transport
		monitoringService.SetHTTPClient(&http.Client{
			Transport: caHTTPClient.Transport,
			Timeout:   monitoring.DefaultCheckTimeout,
		})
	}
	providerService.SetModelHTTPClient(providerModelHTTPClient)
	proxyService := proxy.NewService(db, proxy.WithUsageCost(proxy.UsageCostConfig{
		Rates: proxy.CostRates{
			InputUSDPer1MTokens:     cfg.Usage.Input,
			OutputUSDPer1MTokens:    cfg.Usage.Output,
			EmbeddingUSDPer1MTokens: cfg.Usage.Embedding,
		},
	}), proxy.WithPriceResolver(pricingService))
	slog.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.Redis.RedisURL, cfg.Redis.RedisCacheTTL, slog.Default(), redisstore.ClientOptions{
		PoolSize:     cfg.Redis.RedisPoolSize,
		MinIdleConns: cfg.Redis.RedisMinIdleConnections,
		PoolTimeout:  cfg.Redis.RedisPoolTimeout,
		ReadTimeout:  cfg.Redis.RedisReadTimeout,
		WriteTimeout: cfg.Redis.RedisWriteTimeout,
	})
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
	groupService.SetNotifier(redisStore)
	providerService.SetNotifier(redisStore)
	subscriptionService.SetNotifier(redisStore)
	quotaRedis := subscriptions.NewRedisStore(redisStore, subscriptionService, cfg.Redis.RedisCacheTTL, slog.Default())
	quotaRedis.SyncVersion(ctx)
	if err := quotaRedis.WarmSnapshot(ctx); err != nil {
		return nil, fmt.Errorf("warm subscription snapshot: %w", err)
	}

	var modelLabService *modellab.Service
	if apiCfg != nil {
		catalogHTTPClient := &http.Client{Timeout: 3 * time.Second}
		if caHTTPClient != nil {
			catalogHTTPClient.Transport = caHTTPClient.Transport
		}
		modelLabService, err = modellab.NewService(providerService, pricingService, redisStore, modellab.Options{
			TicketSecret:       apiCfg.SecretsKey,
			ProxyBaseURL:       apiCfg.ProxyBaseURL,
			TicketTTL:          apiCfg.ModelLab.TicketTTL,
			CatalogFreshFor:    apiCfg.ModelLab.CatalogFreshTTL,
			CatalogLastGoodFor: apiCfg.ModelLab.CatalogStaleTTL,
			MaxTargets:         apiCfg.ModelLab.MaxTargets,
			MaxConcurrent:      apiCfg.ModelLab.MaxConcurrent,
			MaxPromptBytes:     apiCfg.ModelLab.MaxPromptBytes,
			MaxOutputTokens:    apiCfg.ModelLab.MaxOutputTokens,
			HTTPClient:         catalogHTTPClient,
		})
		if err != nil {
			return nil, fmt.Errorf("initialize model lab: %w", err)
		}
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
		FAQ:           faqService,
		Groups:        groupService,
		Providers:     providerService,
		Monitoring:    monitoringService,
		ModelLab:      modelLabService,
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

// StartSubscriptionWatcher keeps the process-local subscription assignment
// version and plan snapshot synchronized with configuration changes.
func (a *App) StartSubscriptionWatcher(ctx context.Context) {
	if a == nil || a.QuotaRedis == nil {
		return
	}
	a.subscriptionWatcherOnce.Do(func() {
		slog.Info("starting subscription config watcher")
		go a.QuotaRedis.Watch(ctx)
	})
}

// StartQuotaResetDistribution grants the current calendar month's reset credit
// immediately, then periodically reconciles newly eligible user accounts.
func (a *App) StartQuotaResetDistribution(ctx context.Context, interval time.Duration) {
	if a == nil || a.Subscriptions == nil || interval <= 0 {
		return
	}

	run := func() {
		result, err := a.Subscriptions.DistributeMonthlyQuotaResetCredits(ctx, time.Now().UTC())
		if err != nil {
			slog.Error("failed to distribute monthly quota reset credits", "error", err)
			return
		}
		slog.Info(
			"monthly quota reset credit distribution completed",
			"eligible", result.Eligible,
			"granted", result.Granted,
			"already_processed", result.AlreadyProcessed,
			"skipped_at_maximum", result.SkippedAtMaximum,
		)
	}

	go func() {
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// StartQuotaStateMaintenance applies durable pending reset events to Redis and
// then refreshes the PostgreSQL quota projection. It runs once immediately and
// repeats both operations in that order on the same ticker.
func (a *App) StartQuotaStateMaintenance(ctx context.Context, interval time.Duration) {
	if a == nil || a.Subscriptions == nil || a.QuotaRedis == nil || interval <= 0 {
		return
	}

	run := func() {
		count, err := a.Subscriptions.ReconcilePendingQuotaResets(ctx, a.QuotaRedis, time.Now().UTC())
		if err != nil {
			slog.Error("failed to reconcile pending quota resets", "error", err)
		}
		if count > 0 {
			slog.Info("reconciled pending quota resets", "resets", count)
		}

		synced, err := a.QuotaRedis.SyncQuotaStates(ctx, a.Subscriptions)
		if err != nil {
			slog.Error("failed to sync subscription quota states", "error", err)
			return
		}
		if synced > 0 {
			slog.Info("synced subscription quota states", "users", synced)
		}
	}

	go func() {
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
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
		a.StartSubscriptionWatcher(ctx)
		slog.Info("starting quota reset distribution goroutine", "interval", a.Schedule.QuotaResetDistributionInterval)
		a.StartQuotaResetDistribution(ctx, a.Schedule.QuotaResetDistributionInterval)
		slog.Info("starting subscription quota maintenance goroutine", "interval", a.Schedule.SubscriptionQuotaSyncInterval)
		a.StartQuotaStateMaintenance(ctx, a.Schedule.SubscriptionQuotaSyncInterval)
	}
}
