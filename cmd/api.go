package cli

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/runtime/app"
	"promptgate/backend/internal/transport/httpapi"
)

// newAPICommand builds the CLI command that starts the HTTP API server.
func newAPICommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "Run the HTTP API server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runAPI(*configPath)
		},
	}
}

// runAPI loads API configuration, initializes dependencies, and serves HTTP requests until shutdown.
func runAPI(configPath string) error {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.LoadApi(configPath)
	if err != nil {
		bootstrapLogger.Error("failed to load configuration", "error", err)
		return err
	}

	logLevel := cfg.SlogLevel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info(
		"configuration loaded",
		"log_level",
		logLevel.String(),
		"listen_address",
		cfg.ListenAddress(),
		"backend_base_url",
		cfg.BackendBaseURL,
		"frontend_base_url",
		cfg.FrontendBaseURL,
		"oidc_redirect_url",
		cfg.OIDCRedirectURL,
		"database",
		cfg.DatabaseLogValue(),
		"cors_allowed_origins",
		len(cfg.CORSAllowedOrigins),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		logger.Error("failed to initialize application", "error", err)
		return err
	}
	defer func() {
		if err := a.Close(); err != nil {
			logger.Warn("failed to close application", "error", err)
		}
	}()
	a.StartSubscriptionWatcher(ctx)

	server := &http.Server{
		Addr:              cfg.ListenAddress(),
		Handler:           newAPIHandler(a),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	logger.Info("api server listening", "address", server.Addr)
	return serveHTTP(ctx, server, 30*time.Second, server.ListenAndServe)
}

// newAPIHandler wires runtime app services into the public HTTP API handler.
func newAPIHandler(a *app.App) http.Handler {
	return httpapi.NewHandler(httpapi.Dependencies{
		Redis:         a.Redis,
		Config:        a.Config,
		DB:            a.DB,
		Users:         a.Users,
		Tokens:        a.Tokens,
		FAQ:           a.FAQ,
		Groups:        a.Groups,
		Providers:     a.Providers,
		Monitoring:    a.Monitoring,
		Notifications: a.Notifications,
		ModelLab:      a.ModelLab,
		Pricing:       a.Pricing,
		Proxy:         a.Proxy,
		Subscriptions: a.Subscriptions,
		SetupGuides:   a.SetupGuides,
		QuotaRedis:    a.QuotaRedis,
		OIDC:          a.OIDC,
		Sessions:      a.Sessions,
	})
}
