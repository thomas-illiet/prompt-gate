package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeHTTPWaitsForRequestsBeforeReturning(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "drain", true: "timeout"}[force], func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
				}
				_, _ = w.Write([]byte("finished"))
			})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			grace := 2 * time.Second
			if force {
				grace = 30 * time.Millisecond
			}
			done := make(chan error, 1)
			go func() { done <- serveHTTP(ctx, server, grace, func() error { return server.Serve(listener) }) }()
			response := make(chan string, 1)
			go func() {
				r, err := http.Get("http://" + listener.Addr().String())
				if err != nil {
					response <- ""
					return
				}
				defer r.Body.Close()
				body, _ := io.ReadAll(r.Body)
				response <- string(body)
			}()
			<-entered
			cancel()
			if force {
				if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected drain timeout: %v", err)
				}
			} else {
				select {
				case err := <-done:
					t.Fatalf("returned before request completed: %v", err)
				case <-time.After(20 * time.Millisecond):
				}
				release <- struct{}{}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			if body := <-response; !force && body != "finished" {
				t.Fatalf("request interrupted: %q", body)
			}
		})
	}
}
