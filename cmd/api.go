package cli

import (
	"context"
	"errors"
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
func newAPICommand() *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "Run the HTTP API server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runAPI()
		},
	}
}

// runAPI loads API configuration, initializes dependencies, and serves HTTP requests until shutdown.
func runAPI() error {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.LoadApi()
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
	if a.Validator != nil {
		defer a.Validator.Close()
	}
	defer func() {
		if err := a.Redis.Close(); err != nil {
			logger.Warn("failed to close redis", "error", err)
		}
	}()

	server := &http.Server{
		Addr: cfg.ListenAddress(),
		Handler: httpapi.NewHandler(httpapi.Dependencies{
			Config:    a.Config,
			DB:        a.DB,
			Users:     a.Users,
			Tokens:    a.Tokens,
			Firewall:  a.Firewall,
			Groups:    a.Groups,
			Providers: a.Providers,
			MCP:       a.MCP,
			Proxy:     a.Proxy,
			OIDC:      a.OIDC,
			Sessions:  a.Sessions,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutdown signal received, stopping api server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("failed to shut down server cleanly", "error", err)
			return
		}

		logger.Info("api server stopped cleanly")
	}()

	logger.Info("api server listening", "address", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped unexpectedly", "error", err)
		return err
	}

	return nil
}
