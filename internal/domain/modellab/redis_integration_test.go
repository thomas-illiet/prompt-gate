package modellab

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/testconfig"

	"github.com/google/uuid"
)

func TestRedisTicketConsumptionIsAtomicAcrossConcurrentClients(t *testing.T) {
	rawRedisURL := testconfig.RedisURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := redisstore.NewRequired(ctx, rawRedisURL, time.Minute, nil)
	if err != nil {
		t.Skipf("integration Redis from %s is unavailable: %v", testconfig.Path(t), err)
	}
	t.Cleanup(func() { _ = store.Close() })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"gpt-atomic"}]}`)
	}))
	t.Cleanup(upstream.Close)

	providerName := "redis-atomic-" + uuid.NewString()
	providerRecord := provider.Provider{
		ID:        uuid.New(),
		Name:      providerName,
		Type:      provider.ProviderTypeOpenAI,
		BaseURL:   upstream.URL + "/v1",
		Enabled:   true,
		UpdatedAt: time.Now().UTC(),
	}
	repository := &fakeProviderRepository{records: []provider.Provider{providerRecord}}
	service, err := NewService(repository, nil, store, Options{
		TicketSecret: "0123456789abcdef0123456789abcdef",
		ProxyBaseURL: "https://proxy.example.test",
		TicketTTL:    time.Minute,
		HTTPClient:   upstream.Client(),
	})
	if err != nil {
		t.Fatalf("create Model Lab service: %v", err)
	}

	subject := "redis-atomic-user-" + uuid.NewString()
	issued, err := service.IssueTickets(
		ctx,
		subject,
		"user",
		"https://app.example.test",
		IssueTicketsInput{Targets: []TicketTarget{{ProviderName: providerName, Model: "gpt-atomic"}}},
		AccessFilter{},
	)
	if err != nil {
		t.Fatalf("issue integration ticket: %v", err)
	}
	ticket := issued.Tickets[0]
	claims, err := service.ParseTicket(ticket.Token)
	if err != nil {
		t.Fatalf("parse integration ticket: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cleanupCancel()
		_ = store.Del(cleanupCtx, ticketStoreKey(claims.ID), catalogCacheKey(providerRecord), leaseStoreKey(subject))
	})

	start := make(chan struct{})
	var successes atomic.Int64
	var invalidTickets atomic.Int64
	var unexpected atomic.Int64
	var waitGroup sync.WaitGroup
	for range 100 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, consumeErr := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
				Token:  ticket.Token,
				Method: ticket.Method,
				Path:   claims.Path,
				Origin: claims.Origin,
			})
			switch {
			case consumeErr == nil:
				successes.Add(1)
			case errors.Is(consumeErr, ErrInvalidTicket):
				invalidTickets.Add(1)
			default:
				unexpected.Add(1)
			}
		}()
	}
	close(start)
	waitGroup.Wait()

	if successes.Load() != 1 || invalidTickets.Load() != 99 || unexpected.Load() != 0 {
		t.Fatalf(
			"unexpected concurrent consumption results: success=%d invalid=%d unexpected=%d",
			successes.Load(), invalidTickets.Load(), unexpected.Load(),
		)
	}
}
