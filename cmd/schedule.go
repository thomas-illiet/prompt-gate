package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/runtime/app"
)

// newScheduleCommand builds the CLI command that starts scheduled background jobs.
func newScheduleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "schedule",
		Short: "Run scheduled background jobs",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSchedule()
		},
	}
}

// runSchedule loads scheduler configuration and runs background workers until shutdown.
func runSchedule() error {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.LoadSchedule()
	if err != nil {
		bootstrapLogger.Error("failed to load schedule configuration", "error", err)
		return err
	}

	logLevel := cfg.SlogLevel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info(
		"schedule configuration loaded",
		"log_level",
		logLevel.String(),
		"database",
		cfg.DatabaseLogValue(),
		"token_cleanup_interval",
		cfg.TokenCleanupInterval,
		"user_access_expiration_interval",
		cfg.UserAccessExpirationInterval,
		"usage_raw_retention",
		cfg.UsageRawRetention,
		"usage_raw_cleanup_interval",
		cfg.UsageRawCleanupInterval,
		"subscription_quota_sync_interval",
		cfg.SubscriptionQuotaSyncInterval,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		logger.Error("failed to initialize schedule application", "error", err)
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

	a.StartBackgroundJobs(ctx)
	logger.Info("schedule worker started")

	<-ctx.Done()
	logger.Info("schedule worker stopped")

	return nil
}
