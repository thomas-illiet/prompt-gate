package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/database"
	"promptgate/backend/internal/platform/redisstore"
)

// newWorkerCommand builds the CLI command that starts generic background workers.
func newWorkerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Run generic background workers",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runWorker()
		},
	}
}

// runWorker loads worker configuration and consumes Redis-backed background jobs.
func runWorker() error {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.LoadWorker()
	if err != nil {
		bootstrapLogger.Error("failed to load worker configuration", "error", err)
		return err
	}

	logLevel := cfg.SlogLevel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info(
		"worker configuration loaded",
		"log_level",
		logLevel.String(),
		"database",
		cfg.DatabaseLogValue(),
		"worker_batch_size",
		cfg.WorkerBatchSize,
		"worker_block_timeout",
		cfg.WorkerBlockTimeout,
		"worker_pending_idle_timeout",
		cfg.WorkerPendingIdleTimeout,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize postgres connection", "error", err)
		return err
	}

	logger.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.RedisURL, cfg.RedisCacheTTL, logger)
	if err != nil {
		logger.Error("failed to initialize redis connection", "error", err)
		return err
	}
	defer func() {
		if err := redisStore.Close(); err != nil {
			logger.Warn("failed to close redis", "error", err)
		}
	}()
	logger.Info("redis connection ready")

	worker := proxy.NewWorker(db, redisStore, proxy.WorkerOptions{
		ConsumerName:       cfg.WorkerConsumerName,
		BatchSize:          cfg.WorkerBatchSize,
		BlockTimeout:       cfg.WorkerBlockTimeout,
		PendingIdleTimeout: cfg.WorkerPendingIdleTimeout,
	}, logger)
	if err := worker.Run(ctx); err != nil {
		logger.Error("worker stopped with error", "error", err)
		return err
	}
	logger.Info("worker stopped")
	return nil
}
