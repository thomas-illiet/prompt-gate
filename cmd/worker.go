package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"promptgate/backend/internal/domain/notifications"
	"promptgate/backend/internal/domain/proxy"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/database"
	"promptgate/backend/internal/platform/redisstore"
)

// newWorkerCommand builds the CLI command that starts generic background workers.
func newWorkerCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Run generic background workers",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runWorker(*configPath)
		},
	}
}

// runWorker loads worker configuration and consumes Redis-backed background jobs.
func runWorker(configPath string) error {
	bootstrapLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.LoadWorker(configPath)
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

	db, err := database.OpenPostgres(ctx, cfg.DatabaseURL, database.PoolOptions{
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		MaxIdleConns:    cfg.MaxIdleConnections,
		MaxOpenConns:    cfg.MaxOpenConnections,
	})
	if err != nil {
		logger.Error("failed to initialize postgres connection", "error", err)
		return err
	}

	logger.Info("initializing redis connection")
	redisStore, err := redisstore.NewRequired(ctx, cfg.RedisURL, cfg.RedisCacheTTL, logger, redisstore.ClientOptions{
		PoolSize:     cfg.RedisPoolSize,
		MinIdleConns: cfg.RedisMinIdleConnections,
		PoolTimeout:  cfg.RedisPoolTimeout,
		ReadTimeout:  cfg.RedisReadTimeout,
		WriteTimeout: cfg.RedisWriteTimeout,
	})
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
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	errorsCh := make(chan error, 2)
	go func() { errorsCh <- worker.Run(runCtx) }()
	workers := 1
	if cfg.NotificationsEnabled {
		mailWorker := notifications.NewWorker(db, notifications.WorkerOptions{
			FromName: cfg.MailFromName, FromAddress: cfg.MailFromAddress, ReplyTo: cfg.MailReplyToAddress,
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
			TLSMode: cfg.SMTPTLSMode, Timeout: cfg.SMTPTimeout, PollInterval: cfg.MailPollInterval,
			BatchSize: cfg.MailBatchSize, LeaseDuration: cfg.MailLeaseDuration, MaxAttempts: cfg.MailMaxAttempts, RetryBase: cfg.MailRetryBase,
		}, logger)
		workers++
		go func() { errorsCh <- mailWorker.Run(runCtx) }()
	}
	var runErr error
	for range workers {
		if err := <-errorsCh; err != nil && runErr == nil {
			runErr = err
			cancelRun()
		}
	}
	if runErr != nil {
		err := runErr
		logger.Error("worker stopped with error", "error", err)
		return err
	}
	logger.Info("worker stopped")
	return nil
}
