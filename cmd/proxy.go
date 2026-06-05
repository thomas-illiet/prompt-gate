package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cdrslog "cdr.dev/slog/v3"
	"cdr.dev/slog/v3/sloggers/sloghuman"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/firewall"
	"promptgate/backend/internal/domain/groups"
	localmcp "promptgate/backend/internal/domain/mcp"
	localprovider "promptgate/backend/internal/domain/provider"
	localproxy "promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/database"
	platformhttp "promptgate/backend/internal/platform/httpclient"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/secrets"
	proxyruntime "promptgate/backend/internal/runtime/proxy"
	httpmiddleware "promptgate/backend/internal/transport/httpmiddleware"
)

// newProxyCommand builds the CLI command that starts the LLM proxy server.
func newProxyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "proxy",
		Short: "Run the LLM proxy server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runProxy()
		},
	}
}

// runProxy loads proxy configuration, initializes runtime services, and serves proxy traffic until shutdown.
func runProxy() error {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.LoadProxy()
	if err != nil {
		bootstrapLogger.Error("failed to load proxy configuration", "error", err)
		return err
	}

	stdLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.SlogLevel()}))
	slog.SetDefault(stdLogger)
	bridgeLogger := cdrslog.Make(sloghuman.Sink(os.Stdout)).Leveled(cdrLevel(cfg.LogLevel))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	reloadSignals := make(chan os.Signal, 1)
	signal.Notify(reloadSignals, syscall.SIGHUP)
	defer signal.Stop(reloadSignals)

	db, err := database.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		stdLogger.Error("failed to initialize postgres connection", "error", err)
		return err
	}

	secretCipher, err := secrets.NewCipher(cfg.SecretsKey)
	if err != nil {
		stdLogger.Error("failed to initialize secret cipher", "error", err)
		return err
	}

	stdLogger.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.RedisURL, cfg.RedisCacheTTL, stdLogger)
	if err != nil {
		stdLogger.Error("failed to initialize redis connection", "error", err)
		return err
	}
	stdLogger.Info("redis connection ready")
	defer func(redisStore *redisstore.Store) {
		err := redisStore.Close()
		if err != nil {
			stdLogger.Error("failed to close redis store", "error", err)
		}
	}(redisStore)

	userService := users.NewService(db)
	tokenService := tokens.NewService(db, cfg.JWTSecret)
	firewallService := firewall.NewService(db)
	groupService := groups.NewService(db)
	subscriptionService := subscriptions.NewService(db)
	providerService := localprovider.NewService(db, secretCipher)
	mcpService := localmcp.NewService(db, secretCipher)
	subscriptionStore := subscriptions.NewRedisStore(redisStore, subscriptionService, cfg.RedisCacheTTL, stdLogger)
	subscriptionStore.SyncVersion(ctx)
	if err := subscriptionStore.WarmSnapshot(ctx); err != nil {
		stdLogger.Error("failed to warm subscription snapshot", "error", err)
		return err
	}
	recorder := subscriptions.NewQuotaRecorder(localproxy.NewRedisRecorder(redisStore, stdLogger), subscriptionStore, stdLogger)
	tracer := otel.GetTracerProvider().Tracer("promptgate-proxy")
	proxyHTTPClient := &http.Client{Timeout: cfg.ProxyUpstreamTimeout}
	caHTTPClient, err := platformhttp.NewWithCAFile(cfg.CAFile, cfg.ProxyUpstreamTimeout)
	if err != nil {
		stdLogger.Error("failed to initialize proxy CA HTTP client", "error", err)
		return err
	}
	if caHTTPClient != nil {
		stdLogger.Info("loaded proxy CA file", "path", cfg.CAFile)
		proxyHTTPClient = caHTTPClient
	}

	authCache := tokens.NewRedisAuthCache(redisStore, cfg.RedisCacheTTL, stdLogger)
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
		Logger:                   stdLogger,
		BridgeLogger:             bridgeLogger,
		Tracer:                   tracer,
		ReloadDebounce:           cfg.ProxyReloadDebounce,
		MaxBufferedRequestBytes:  cfg.ProxyMaxBufferedRequestBytes,
		MaxBufferedResponseBytes: cfg.ProxyMaxBufferedResponseBytes,
	})
	if err != nil {
		stdLogger.Error("failed to initialize proxy runtime", "error", err)
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := manager.Shutdown(shutdownCtx); err != nil {
			stdLogger.Error("failed to shut down proxy runtime", "error", err)
		}
	}()
	go manager.Watch(ctx)
	go subscriptionStore.Watch(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-reloadSignals:
				stdLogger.Info("reload signal received, reloading proxy runtime")
				reloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := manager.Reload(reloadCtx); err != nil {
					stdLogger.Error("proxy runtime reload failed; keeping previous runtime", "error", err)
				} else {
					stdLogger.Info("proxy runtime reloaded")
				}
				cancel()
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	clientIPOptions := clientip.Options{
		TrustForwardHeaders: cfg.ProxyTrustForwardHeaders,
		TrustedProxies:      cfg.ProxyTrustedProxies,
	}
	proxyHandler := tokens.MiddlewareWithOptions(tokens.MiddlewareOptions{
		TokenService: tokenService,
		UserResolver: userService,
		Cache:        authCache,
		Logger:       stdLogger,
	})(
		clientip.MiddlewareWithOptions(clientIPOptions)(
			firewall.MiddlewareWithOptions(firewallSnapshot, clientIPOptions, stdLogger)(
				groups.MiddlewareWithOptions(accessSnapshot, stdLogger, groups.MiddlewareOptions{
					MaxBufferedRequestBytes: cfg.ProxyMaxBufferedRequestBytes,
				})(
					subscriptions.Middleware(subscriptionStore, stdLogger)(
						auth.ActorMiddleware(manager),
					),
				),
			),
		),
	)
	if len(cfg.CORSAllowedOrigins) > 0 {
		proxyHandler = httpmiddleware.CORS(cfg.CORSAllowedOrigins)(proxyHandler)
	}
	mux.Handle("/", proxyHandler)

	server := &http.Server{
		Addr:              cfg.ListenAddress(),
		Handler:           httpmiddleware.SecurityHeaders()(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		stdLogger.Info("shutdown signal received, stopping proxy server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			stdLogger.Error("failed to shut down proxy server cleanly", "error", err)
		}
	}()

	stdLogger.Info("proxy server listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		stdLogger.Error("proxy server stopped unexpectedly", "error", err)
		return err
	}

	return nil
}

// cdrLevel maps configured log level text to the cdr/slog level type.
func cdrLevel(level string) cdrslog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return cdrslog.LevelDebug
	case "warn", "warning":
		return cdrslog.LevelWarn
	case "error":
		return cdrslog.LevelError
	default:
		return cdrslog.LevelInfo
	}
}
