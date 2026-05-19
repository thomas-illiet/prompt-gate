package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/database"
	"promptgate/backend/internal/platform/migrations"
)

func newMigrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMigrate()
		},
	}
}

func runMigrate() error {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.LoadMigration()
	if err != nil {
		bootstrapLogger.Error("failed to load migration configuration", "error", err)
		return err
	}

	logLevel := cfg.SlogLevel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info(
		"migration configuration loaded",
		"log_level",
		logLevel.String(),
		"database",
		cfg.DatabaseLogValue(),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("initializing postgres connection")
	db, err := database.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize postgres connection", "error", err)
		return err
	}
	logger.Info("postgres connection ready")

	if err := migrations.Run(ctx, db); err != nil {
		logger.Error("failed to migrate database", "error", err)
		return err
	}

	return nil
}
