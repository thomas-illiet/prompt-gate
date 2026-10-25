package cli

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cdrslog "cdr.dev/slog/v3"
	"cdr.dev/slog/v3/sloggers/sloghuman"
	"github.com/spf13/cobra"

	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/requestprivacy"
	"promptgate/backend/internal/runtime/app"
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
	bridgeLogger := cdrslog.Make(
		requestprivacy.NewMetadataOnlySink(sloghuman.Sink(os.Stdout)),
	).Leveled(cdrLevel(cfg.LogLevel))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	reloadSignals := make(chan os.Signal, 1)
	signal.Notify(reloadSignals, syscall.SIGHUP)
	defer signal.Stop(reloadSignals)

	proxyRuntime, err := app.NewProxy(ctx, cfg, stdLogger, bridgeLogger)
	if err != nil {
		stdLogger.Error("failed to initialize proxy runtime", "error", err)
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := proxyRuntime.Close(shutdownCtx); err != nil {
			stdLogger.Error("failed to shut down proxy runtime", "error", err)
		}
	}()
	proxyRuntime.Start(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-reloadSignals:
				stdLogger.Info("reload signal received, reloading proxy runtime")
				reloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := proxyRuntime.Reload(reloadCtx); err != nil {
					stdLogger.Error("proxy runtime reload failed; keeping previous runtime", "error", err)
				} else {
					stdLogger.Info("proxy runtime reloaded")
				}
				cancel()
			}
		}
	}()

	server := newProxyHTTPServer(cfg.ListenAddress(), proxyRuntime.Handler, cfg.ProxyUpstreamTimeout)

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

// newProxyHTTPServer bounds header and request-body reads at the connection
// layer. The server-wide WriteTimeout remains zero so a healthy SSE stream can
// live for the whole upstream request. A sliding per-write deadline instead
// bounds only a client that stops reading while one Write/Flush is in progress.
func newProxyHTTPServer(addr string, handler http.Handler, requestReadTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           writeDeadlineMiddleware(requestReadTimeout)(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       requestReadTimeout,
		IdleTimeout:       requestReadTimeout,
	}
}

func writeDeadlineMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(&writeDeadlineResponseWriter{ResponseWriter: w, timeout: timeout}, r)
		})
	}
}

type writeDeadlineResponseWriter struct {
	http.ResponseWriter
	timeout time.Duration
}

func (w *writeDeadlineResponseWriter) refresh() error {
	if w.timeout <= 0 {
		return nil
	}
	return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(time.Now().Add(w.timeout))
}

func (w *writeDeadlineResponseWriter) WriteHeader(statusCode int) {
	_ = w.refresh()
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *writeDeadlineResponseWriter) Write(raw []byte) (int, error) {
	if err := w.refresh(); err != nil {
		return 0, err
	}
	return w.ResponseWriter.Write(raw)
}

func (w *writeDeadlineResponseWriter) Flush() {
	if err := w.refresh(); err != nil {
		return
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *writeDeadlineResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *writeDeadlineResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if err := w.refresh(); err != nil {
		return 0, err
	}
	return io.Copy(w.ResponseWriter, reader)
}

func (w *writeDeadlineResponseWriter) Push(target string, options *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

func (w *writeDeadlineResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

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
