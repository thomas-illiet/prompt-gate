package cli

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// serveHTTP does not return until Shutdown has drained requests. Dependencies
// owned by the caller therefore stay open for the entire drain period.
func serveHTTP(ctx context.Context, server *http.Server, grace time.Duration, serve func() error) error {
	finished := make(chan error, 1)
	go func() { finished <- serve() }()
	select {
	case err := <-finished:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			_ = server.Close()
		}
		serveErr := <-finished
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(err, serveErr)
	}
}
