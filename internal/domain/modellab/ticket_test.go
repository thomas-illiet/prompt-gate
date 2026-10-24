package modellab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/domain/provider"

	"github.com/google/uuid"
)

func newTicketTestService(t *testing.T, now *time.Time) (*Service, func()) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "model discovery unavailable", http.StatusBadGateway)
	}))
	providers := &fakeProviderRepository{records: []provider.Provider{
		{
			ID:        uuid.New(),
			Name:      "openai-main",
			Type:      provider.ProviderTypeOpenAI,
			BaseURL:   upstream.URL + "/v1",
			UpdatedAt: *now,
			Config:    provider.ProviderConfig{"modelLabModels": []any{"gpt-5", "gpt-5-mini"}},
		},
	}}
	service := newModelLabTestService(t, providers, nil, newModelLabTestStore(t), func() time.Time { return *now }, upstream.Client())
	return service, upstream.Close
}

func TestIssueAndConsumeTicketIsRequestBoundAndSingleUse(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	service, closeUpstream := newTicketTestService(t, &now)
	t.Cleanup(closeUpstream)

	issued, err := service.IssueTickets(
		context.Background(),
		"user-1",
		"user",
		"https://app.example.test",
		IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-5"}}},
		AccessFilter{},
	)
	if err != nil {
		t.Fatalf("issue tickets: %v", err)
	}
	if len(issued.Tickets) != 1 {
		t.Fatalf("expected one ticket, got %#v", issued)
	}
	ticket := issued.Tickets[0]
	if ticket.Endpoint != "https://proxy.example.test/openai-main/v1/chat/completions" || ticket.Protocol != ProtocolOpenAIChat {
		t.Fatalf("unexpected ticket endpoint: %#v", ticket)
	}
	if len(ticket.Token) <= len(TicketPrefix) || ticket.Token[:len(TicketPrefix)] != TicketPrefix {
		t.Fatalf("unexpected token prefix: %q", ticket.Token)
	}

	if _, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
		Token: ticket.Token, Method: http.MethodPost, Path: "/wrong", Origin: "https://app.example.test",
	}); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected wrong path to fail, got %v", err)
	}
	if _, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
		Token: ticket.Token, Method: "post", Path: "/openai-main/v1/chat/completions", Origin: "https://app.example.test",
	}); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected non-canonical method to fail, got %v", err)
	}
	claims, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
		Token: ticket.Token, Method: http.MethodPost, Path: "/openai-main/v1/chat/completions", Origin: "https://app.example.test",
	})
	if err != nil {
		t.Fatalf("consume valid ticket: %v", err)
	}
	if claims.Subject != "user-1" || claims.Model != "gpt-5" || claims.ComparisonID != issued.ComparisonID {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if _, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
		Token: ticket.Token, Method: http.MethodPost, Path: claims.Path, Origin: claims.Origin,
	}); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected replay to fail, got %v", err)
	}
	if _, err := service.ParseTicket(ticket.Token + "x"); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected tampered token to fail, got %v", err)
	}
}

func TestTicketIssuanceBudgetIsAtomicAndReleasedOnConsume(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	service, closeUpstream := newTicketTestService(t, &now)
	t.Cleanup(closeUpstream)

	const attempts = 40
	var successes atomic.Int64
	var limited atomic.Int64
	var firstTicket IssuedTicket
	var firstTicketMu sync.Mutex
	errorsSeen := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			issued, err := service.IssueTickets(
				context.Background(), "burst-user", "user", "",
				IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-5"}}},
				AccessFilter{},
			)
			switch {
			case err == nil:
				successes.Add(1)
				firstTicketMu.Lock()
				if firstTicket.Token == "" {
					firstTicket = issued.Tickets[0]
				}
				firstTicketMu.Unlock()
			case errors.Is(err, ErrTicketIssueLimit):
				limited.Add(1)
			default:
				errorsSeen <- err
			}
		}()
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("unexpected burst error: %v", err)
	}
	if successes.Load() != int64(service.maxOutstanding) || successes.Load()+limited.Load() != attempts {
		t.Fatalf("unexpected issuance budget result: successes=%d limited=%d max=%d", successes.Load(), limited.Load(), service.maxOutstanding)
	}

	firstTicketMu.Lock()
	ticket := firstTicket
	firstTicketMu.Unlock()
	if _, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
		Token: ticket.Token, Method: ticket.Method, Path: ticketPath(ticket),
	}); err != nil {
		t.Fatalf("consume reserved ticket: %v", err)
	}
	if _, err := service.IssueTickets(
		context.Background(), "burst-user", "user", "",
		IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-5"}}},
		AccessFilter{},
	); err != nil {
		t.Fatalf("expected issuance slot after consume, got %v", err)
	}
}

func TestTicketExpiresAndTargetsAreValidated(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	service, closeUpstream := newTicketTestService(t, &now)
	t.Cleanup(closeUpstream)

	duplicates := IssueTicketsInput{Targets: []TicketTarget{
		{ProviderName: "openai-main", Model: "gpt-5"},
		{ProviderName: "openai-main", Model: "gpt-5"},
	}}
	if _, err := service.IssueTickets(context.Background(), "user-1", "user", "", duplicates, AccessFilter{}); !errors.Is(err, ErrInvalidTargets) {
		t.Fatalf("expected duplicate targets to fail, got %v", err)
	}
	if _, err := service.IssueTickets(
		context.Background(), "user-1", "user", "",
		IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-4"}}},
		AccessFilter{},
	); !errors.Is(err, ErrTargetNotAllowed) {
		t.Fatalf("expected unknown target to fail, got %v", err)
	}

	issued, err := service.IssueTickets(
		context.Background(), "user-1", "user", "",
		IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-5"}}},
		AccessFilter{},
	)
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := service.ParseTicket(issued.Tickets[0].Token); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected expired ticket to fail, got %v", err)
	}
}

func TestTicketConsumptionIsAtomicUnderConcurrency(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	service, closeUpstream := newTicketTestService(t, &now)
	t.Cleanup(closeUpstream)
	issued, err := service.IssueTickets(
		context.Background(), "user-1", "user", "",
		IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-5"}}},
		AccessFilter{},
	)
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	ticket := issued.Tickets[0]

	var successes atomic.Int64
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
				Token: ticket.Token, Method: ticket.Method, Path: "/openai-main/v1/chat/completions",
			}); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected exactly one successful consumption, got %d", successes.Load())
	}
}

func TestLeaseEnforcesThreeParallelStreams(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	service, closeUpstream := newTicketTestService(t, &now)
	t.Cleanup(closeUpstream)

	claims := make([]TicketClaims, 0, 4)
	for range 4 {
		issued, err := service.IssueTickets(
			context.Background(), "user-1", "user", "",
			IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-5"}}},
			AccessFilter{},
		)
		if err != nil {
			t.Fatalf("issue ticket: %v", err)
		}
		ticket := issued.Tickets[0]
		claim, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
			Token: ticket.Token, Method: ticket.Method, Path: "/openai-main/v1/chat/completions",
		})
		if err != nil {
			t.Fatalf("consume ticket: %v", err)
		}
		claims = append(claims, claim)
	}
	for i := range 3 {
		if err := service.AcquireLease(context.Background(), claims[i]); err != nil {
			t.Fatalf("acquire lease %d: %v", i, err)
		}
	}
	if err := service.AcquireLease(context.Background(), claims[3]); !errors.Is(err, ErrParallelLimit) {
		t.Fatalf("expected fourth lease to fail, got %v", err)
	}
	if err := service.ReleaseLease(context.Background(), claims[0]); err != nil {
		t.Fatalf("release lease: %v", err)
	}
	if err := service.AcquireLease(context.Background(), claims[3]); err != nil {
		t.Fatalf("expected lease after release, got %v", err)
	}
}

func TestNewServiceRejectsUnsafeConfiguration(t *testing.T) {
	providers := &fakeProviderRepository{}
	if _, err := NewService(providers, nil, nil, Options{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("expected invalid configuration, got %v", err)
	}
	store := newModelLabTestStore(t)
	for _, options := range []Options{
		{TicketSecret: "0123456789abcdef0123456789abcdef", ProxyBaseURL: "https://proxy.example.test", MaxTargets: MaxTargets + 1},
		{TicketSecret: "0123456789abcdef0123456789abcdef", ProxyBaseURL: "https://proxy.example.test", MaxConcurrent: MaxTargets + 1},
		{TicketSecret: "0123456789abcdef0123456789abcdef", ProxyBaseURL: "https://user:secret@proxy.example.test"},
	} {
		if _, err := NewService(providers, nil, store, options); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("expected configured hard-cap rejection, got %v", err)
		}
	}
}

func TestIssueTicketSeparatesPublicPrefixFromCanonicalProxyPath(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	base, closeUpstream := newTicketTestService(t, &now)
	t.Cleanup(closeUpstream)
	service, err := NewService(base.providers, base.pricing, newModelLabTestStore(t), Options{
		TicketSecret: "0123456789abcdef0123456789abcdef",
		ProxyBaseURL: "https://proxy.example.test/remote",
		HTTPClient:   base.httpClient,
		Now:          base.now,
	})
	if err != nil {
		t.Fatalf("create prefixed service: %v", err)
	}
	issued, err := service.IssueTickets(
		context.Background(), "user-1", "user", "",
		IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-main", Model: "gpt-5"}}},
		AccessFilter{},
	)
	if err != nil {
		t.Fatalf("issue prefixed ticket: %v", err)
	}
	ticket := issued.Tickets[0]
	if ticket.Endpoint != "https://proxy.example.test/remote/openai-main/v1/chat/completions" {
		t.Fatalf("unexpected public endpoint: %q", ticket.Endpoint)
	}
	claims, err := service.ParseTicket(ticket.Token)
	if err != nil {
		t.Fatalf("parse prefixed ticket: %v", err)
	}
	if claims.Path != "/openai-main/v1/chat/completions" {
		t.Fatalf("ticket path must match the post-rewrite proxy path, got %q", claims.Path)
	}
	if _, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
		Token: ticket.Token, Method: ticket.Method, Path: "/remote" + claims.Path,
	}); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("public prefix must not be accepted by the proxy binding, got %v", err)
	}
	if _, err := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
		Token: ticket.Token, Method: ticket.Method, Path: claims.Path,
	}); err != nil {
		t.Fatalf("consume canonical post-rewrite path: %v", err)
	}
}

func TestNewServicePreservesEqualCatalogFreshAndStaleTTLs(t *testing.T) {
	service, err := NewService(&fakeProviderRepository{}, nil, newModelLabTestStore(t), Options{
		TicketSecret:       "0123456789abcdef0123456789abcdef",
		ProxyBaseURL:       "https://proxy.example.test",
		CatalogFreshFor:    time.Hour,
		CatalogLastGoodFor: time.Hour,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if service.catalogFreshFor != time.Hour || service.catalogLastGoodFor != time.Hour {
		t.Fatalf("equal catalog TTLs were rewritten: fresh=%s stale=%s", service.catalogFreshFor, service.catalogLastGoodFor)
	}
}

func TestServiceAppliesConfiguredModelLabLimits(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "discovery disabled", http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)
	providers := &fakeProviderRepository{records: []provider.Provider{{
		ID:        uuid.New(),
		Name:      "openai-limited",
		Type:      provider.ProviderTypeOpenAI,
		BaseURL:   upstream.URL + "/v1",
		UpdatedAt: now,
		Config: provider.ProviderConfig{
			"modelLabModels": []any{"gpt-a", "gpt-b"},
		},
	}}}
	service, err := NewService(providers, nil, newModelLabTestStore(t), Options{
		TicketSecret:    "0123456789abcdef0123456789abcdef",
		ProxyBaseURL:    "https://proxy.example.test",
		TicketTTL:       time.Minute,
		LeaseTTL:        time.Minute,
		MaxTargets:      1,
		MaxConcurrent:   1,
		MaxPromptBytes:  2048,
		MaxOutputTokens: 128,
		HTTPClient:      upstream.Client(),
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("create limited service: %v", err)
	}
	if service.maxOutstanding != 1 {
		t.Fatalf("expected configured outstanding-ticket budget 1, got %d", service.maxOutstanding)
	}
	catalog, err := service.Catalog(context.Background(), AccessFilter{})
	if err != nil {
		t.Fatalf("load configured catalog limits: %v", err)
	}
	if catalog.Limits != (ModelLabLimits{MaxTargets: 1, MaxPromptBytes: 2048, MaxOutputTokens: 128}) {
		t.Fatalf("unexpected catalog limits: %#v", catalog.Limits)
	}

	if _, err := service.IssueTickets(
		context.Background(), "user-1", "user", "",
		IssueTicketsInput{Targets: []TicketTarget{
			{ProviderName: "openai-limited", Model: "gpt-a"},
			{ProviderName: "openai-limited", Model: "gpt-b"},
		}}, AccessFilter{},
	); !errors.Is(err, ErrInvalidTargets) {
		t.Fatalf("expected configured target limit, got %v", err)
	}

	claims := make([]TicketClaims, 0, 2)
	for _, model := range []string{"gpt-a", "gpt-b"} {
		issued, issueErr := service.IssueTickets(
			context.Background(), "user-1", "user", "",
			IssueTicketsInput{Targets: []TicketTarget{{ProviderName: "openai-limited", Model: model}}},
			AccessFilter{},
		)
		if issueErr != nil {
			t.Fatalf("issue %s ticket: %v", model, issueErr)
		}
		ticket := issued.Tickets[0]
		claim, consumeErr := service.ConsumeTicket(context.Background(), ConsumeTicketInput{
			Token: ticket.Token, Method: ticket.Method, Path: ticketPath(ticket),
		})
		if consumeErr != nil {
			t.Fatalf("consume %s ticket: %v", model, consumeErr)
		}
		claims = append(claims, claim)
	}
	if err := service.AcquireLease(context.Background(), claims[0]); err != nil {
		t.Fatalf("acquire first configured lease: %v", err)
	}
	if err := service.AcquireLease(context.Background(), claims[1]); !errors.Is(err, ErrParallelLimit) {
		t.Fatalf("expected configured parallel limit, got %v", err)
	}

	payload := []byte(`{"model":"gpt-a","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":129}`)
	if _, err := service.ValidateAndNormalizePayload(ProtocolOpenAIChat, "gpt-a", payload, DefaultMaxRequestBytes); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected configured output-token limit, got %v", err)
	}
}

func ticketPath(ticket IssuedTicket) string {
	prefix := "https://proxy.example.test"
	return strings.TrimPrefix(ticket.Endpoint, prefix)
}
