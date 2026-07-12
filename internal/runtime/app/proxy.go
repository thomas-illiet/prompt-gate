package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	cdrslog "cdr.dev/slog/v3"
	"go.opentelemetry.io/otel"

	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	localmcp "promptgate/backend/internal/domain/mcp"
	localprovider "promptgate/backend/internal/domain/provider"
	localproxy "promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/database"
	platformhttp "promptgate/backend/internal/platform/httpclient"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/secrets"
	proxyruntime "promptgate/backend/internal/runtime/proxy"
)

// ProxyRuntime owns the dependencies and HTTP handler of the proxy process.
// Signal handling and the HTTP server lifecycle remain the CLI's responsibility.
type ProxyRuntime struct {
	Handler           http.Handler
	manager           *proxyruntime.Manager
	subscriptionStore *subscriptions.RedisStore
	redis             *redisstore.Store
	logger            *slog.Logger
}

// NewProxy initializes a proxy runtime from typed configuration.
func NewProxy(ctx context.Context, cfg config.ProxyConfig, logger *slog.Logger, bridgeLogger cdrslog.Logger) (*ProxyRuntime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	db, err := database.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}

	secretCipher, err := secrets.NewCipher(cfg.SecretsKey)
	if err != nil {
		return nil, fmt.Errorf("initialize secret cipher: %w", err)
	}

	logger.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.RedisURL, cfg.RedisCacheTTL, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize redis: %w", err)
	}
	logger.Info("redis connection ready")
	success := false
	defer func() {
		if !success {
			_ = redisStore.Close()
		}
	}()

	userService := users.NewService(db)
	tokenService := tokens.NewService(db, cfg.JWTSecret)
	firewallService := firewall.NewService(db)
	groupService := groups.NewService(db)
	subscriptionService := subscriptions.NewService(db)
	providerService := localprovider.NewService(db, secretCipher)
	mcpService := localmcp.NewService(db, secretCipher)
	subscriptionStore := subscriptions.NewRedisStore(redisStore, subscriptionService, cfg.RedisCacheTTL, logger)
	subscriptionStore.SyncVersion(ctx)
	if err := subscriptionStore.WarmSnapshot(ctx); err != nil {
		return nil, fmt.Errorf("warm subscription snapshot: %w", err)
	}

	recorder := subscriptions.NewQuotaRecorder(localproxy.NewRedisRecorder(redisStore, logger), subscriptionStore, logger)
	proxyHTTPClient := &http.Client{Timeout: cfg.ProxyUpstreamTimeout}
	caHTTPClient, err := platformhttp.NewWithCAFile(cfg.CAFile, cfg.ProxyUpstreamTimeout)
	if err != nil {
		return nil, fmt.Errorf("initialize proxy CA HTTP client: %w", err)
	}
	if caHTTPClient != nil {
		logger.Info("loaded proxy CA file", "path", cfg.CAFile)
		proxyHTTPClient = caHTTPClient
	}

	authCache := tokens.NewRedisAuthCache(redisStore, cfg.RedisCacheTTL, logger)
	authCache.SyncVersion(ctx)
	firewallSnapshot := firewall.NewSnapshotStore(firewallService)
	accessSnapshot := groups.NewSnapshotStore(groupService)
	manager, err := proxyruntime.NewManager(ctx, proxyruntime.Options{
		Providers:                providerService,
		MCP:                      mcpService,
		Recorder:                 recorder,
		FirewallSnapshot:         firewallSnapshot,
		AccessSnapshot:           accessSnapshot,
		AuthCache:                authCache,
		Redis:                    redisStore,
		HTTPClient:               proxyHTTPClient,
		Logger:                   logger,
		BridgeLogger:             bridgeLogger,
		Tracer:                   otel.GetTracerProvider().Tracer("promptgate-proxy"),
		ReloadDebounce:           cfg.ProxyReloadDebounce,
		MaxBufferedRequestBytes:  cfg.ProxyMaxBufferedRequestBytes,
		MaxBufferedResponseBytes: cfg.ProxyMaxBufferedResponseBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize proxy manager: %w", err)
	}

	runtime := &ProxyRuntime{
		manager:           manager,
		subscriptionStore: subscriptionStore,
		redis:             redisStore,
		logger:            logger,
	}
	runtime.Handler = runtime.buildHandler(cfg, tokenService, userService, authCache, firewallSnapshot, accessSnapshot)
	success = true
	return runtime, nil
}

// Start launches runtime watchers tied to the supplied context.
func (p *ProxyRuntime) Start(ctx context.Context) {
	go p.manager.Watch(ctx)
	go p.subscriptionStore.Watch(ctx)
}

// Reload refreshes proxy-backed configuration without dropping the current runtime on failure.
func (p *ProxyRuntime) Reload(ctx context.Context) error {
	return p.manager.Reload(ctx)
}

// Close releases the proxy bridge and Redis connection.
func (p *ProxyRuntime) Close(ctx context.Context) error {
	return errors.Join(p.manager.Shutdown(ctx), p.redis.Close())
}
