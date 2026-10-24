package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/redisstore"

	"github.com/alicebob/miniredis/v2"
)

type httpModelLabFixture struct {
	server         server
	profile        auth.UserProfile
	allowedCalls   *atomic.Int64
	deniedCalls    *atomic.Int64
	frontendOrigin string
}

func newHTTPModelLabFixture(t *testing.T) httpModelLabFixture {
	t.Helper()

	ctx := context.Background()
	providerService, groupService, db := newHTTPSetupServices(t)
	profile := testUserProfile()
	createHTTPUser(t, db, profile)

	allowedCalls := &atomic.Int64{}
	deniedCalls := &atomic.Int64{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/allowed/v1/models":
			allowedCalls.Add(1)
			_, _ = io.WriteString(w, `{"data":[{"id":"gpt-5"},{"id":"gpt-4.1"}]}`)
		case "/denied/v1/models":
			deniedCalls.Add(1)
			_, _ = io.WriteString(w, `{"data":[{"id":"secret-model"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	allowed, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:        "openai-allowed",
		DisplayName: "OpenAI Allowed",
		Type:        provider.ProviderTypeOpenAI,
		BaseURL:     upstream.URL + "/allowed/v1",
		APIKey:      "allowed-provider-secret",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("create allowed provider: %v", err)
	}
	if _, err := providerService.CreateProvider(ctx, provider.CreateProviderInput{
		Name:        "openai-denied",
		DisplayName: "OpenAI Denied",
		Type:        provider.ProviderTypeOpenAI,
		BaseURL:     upstream.URL + "/denied/v1",
		APIKey:      "denied-provider-secret",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("create denied provider: %v", err)
	}

	group, err := groupService.CreateGroup(ctx, groups.CreateGroupInput{
		Name:          "model-lab-users",
		DisplayName:   "Model Lab users",
		ProviderIDs:   []string{allowed.ID.String()},
		ModelPatterns: []string{`^gpt-5$`},
	})
	if err != nil {
		t.Fatalf("create Model Lab access group: %v", err)
	}
	if err := groupService.AddMember(ctx, group.ID.String(), profile.ID); err != nil {
		t.Fatalf("add Model Lab group member: %v", err)
	}

	redisServer := miniredis.RunT(t)
	store, err := redisstore.NewRequired(ctx, "redis://"+redisServer.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("create Redis store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	modelLabService, err := modellab.NewService(providerService, nil, store, modellab.Options{
		TicketSecret: "0123456789abcdef0123456789abcdef",
		ProxyBaseURL: "https://proxy.example.test",
		TicketTTL:    time.Minute,
		HTTPClient:   upstream.Client(),
	})
	if err != nil {
		t.Fatalf("create Model Lab service: %v", err)
	}

	const frontendOrigin = "https://app.example.test"
	return httpModelLabFixture{
		server: server{
			config: config.APIConfig{
				ServerConfig:    config.ServerConfig{CORSAllowedOrigins: []string{frontendOrigin}},
				PublicURLConfig: config.PublicURLConfig{FrontendBaseURL: frontendOrigin, ProxyBaseURL: "https://proxy.example.test"},
				ModelLab:        config.ModelLabConfig{},
			},
			groups:   groupService,
			modelLab: modelLabService,
		},
		profile:        profile,
		allowedCalls:   allowedCalls,
		deniedCalls:    deniedCalls,
		frontendOrigin: frontendOrigin,
	}
}

func (f httpModelLabFixture) request(method, target, body, origin string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	return request.WithContext(auth.ContextWithUser(context.Background(), f.profile))
}

func TestModelLabCatalogIsGroupFilteredAndNotStored(t *testing.T) {
	fixture := newHTTPModelLabFixture(t)
	recorder := httptest.NewRecorder()

	fixture.server.handleModelLabCatalog(recorder, fixture.request(http.MethodGet, "/api/v1/me/models", "", ""))

	if recorder.Code != http.StatusOK {
		t.Fatalf("catalog status: got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("catalog Cache-Control: got %q", got)
	}
	if fixture.allowedCalls.Load() != 1 || fixture.deniedCalls.Load() != 0 {
		t.Fatalf("unexpected provider discovery calls: allowed=%d denied=%d", fixture.allowedCalls.Load(), fixture.deniedCalls.Load())
	}

	var response modellab.CatalogResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(response.Providers) != 1 || response.Providers[0].Name != "openai-allowed" {
		t.Fatalf("unexpected provider catalog: %#v", response.Providers)
	}
	models := response.Providers[0].Models
	if len(models) != 1 || models[0].ID != "gpt-5" {
		t.Fatalf("unexpected group-filtered models: %#v", models)
	}
	for _, secret := range []string{"allowed-provider-secret", "denied-provider-secret"} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("catalog leaked secret %q: %s", secret, recorder.Body.String())
		}
	}
}

func TestModelLabTicketOriginNoStoreAndDisabled(t *testing.T) {
	fixture := newHTTPModelLabFixture(t)
	payload := `{"targets":[{"providerName":"openai-allowed","model":"gpt-5"}]}`

	recorder := httptest.NewRecorder()
	fixture.server.handleIssueModelLabTickets(recorder, fixture.request(http.MethodPost, "/api/v1/me/model-lab/tickets", payload, fixture.frontendOrigin))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("ticket status: got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("ticket Cache-Control: got %q", got)
	}
	var issued modellab.IssueTicketsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&issued); err != nil {
		t.Fatalf("decode issued ticket: %v", err)
	}
	if len(issued.Tickets) != 1 || !strings.HasPrefix(issued.Tickets[0].Token, modellab.TicketPrefix) {
		t.Fatalf("unexpected issued tickets: %#v", issued)
	}
	if issued.Tickets[0].Endpoint != "https://proxy.example.test/openai-allowed/v1/chat/completions" {
		t.Fatalf("unexpected ticket endpoint: %q", issued.Tickets[0].Endpoint)
	}

	wrongOrigin := httptest.NewRecorder()
	fixture.server.handleIssueModelLabTickets(wrongOrigin, fixture.request(http.MethodPost, "/api/v1/me/model-lab/tickets", payload, "https://evil.example.test"))
	if wrongOrigin.Code != http.StatusForbidden || !strings.Contains(wrongOrigin.Body.String(), "model_lab_origin_not_allowed") {
		t.Fatalf("wrong origin was not rejected: status=%d body=%s", wrongOrigin.Code, wrongOrigin.Body.String())
	}
	if got := wrongOrigin.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("wrong-origin Cache-Control: got %q", got)
	}

	unavailableServer := fixture.server
	unavailableServer.modelLab = nil
	unavailable := httptest.NewRecorder()
	unavailableServer.handleModelLabCatalog(unavailable, fixture.request(http.MethodGet, "/api/v1/me/models", "", ""))
	if unavailable.Code != http.StatusServiceUnavailable || !strings.Contains(unavailable.Body.String(), "model_lab_unavailable") {
		t.Fatalf("missing Model Lab service was not reported unavailable: status=%d body=%s", unavailable.Code, unavailable.Body.String())
	}
	if got := unavailable.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("unavailable Cache-Control: got %q", got)
	}
}

func TestModelLabTicketIssueLimitReturnsTooManyRequests(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeModelLabIssueError(recorder, modellab.ErrTicketIssueLimit)
	if recorder.Code != http.StatusTooManyRequests || !strings.Contains(recorder.Body.String(), "model_lab_ticket_issue_limit") {
		t.Fatalf("unexpected issue-limit response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
