package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/config"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/requestprivacy"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type modelLabProxyProviderRepository struct {
	record provider.Provider
}

func (r modelLabProxyProviderRepository) ListEnabled(context.Context) ([]provider.Provider, error) {
	return []provider.Provider{r.record}, nil
}

func (modelLabProxyProviderRepository) DecryptAPIKey(provider.Provider) (string, error) {
	return "", nil
}

type modelLabProxyFixture struct {
	config  config.ModelLabConfig
	db      *gorm.DB
	origin  string
	service *modellab.Service
	userID  string
	users   *users.Service
}

func newModelLabProxyFixture(t *testing.T) modelLabProxyFixture {
	t.Helper()
	ctx := context.Background()
	catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"gpt-test"}]}`)
	}))
	t.Cleanup(catalogServer.Close)

	redisServer := miniredis.RunT(t)
	store, err := redisstore.NewRequired(ctx, "redis://"+redisServer.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("create Redis store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	userService := users.NewService(db)
	if err := userService.AutoMigrate(ctx); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	record := users.User{
		Email:             "model-lab@example.test",
		PreferredUsername: "model-lab-user",
		Name:              "Model Lab User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	service, err := modellab.NewService(
		modelLabProxyProviderRepository{record: provider.Provider{
			ID:        uuid.New(),
			Name:      "openai-test",
			Type:      provider.ProviderTypeOpenAI,
			BaseURL:   catalogServer.URL + "/v1",
			Enabled:   true,
			UpdatedAt: time.Now().UTC(),
		}},
		nil,
		store,
		modellab.Options{
			TicketSecret:    "0123456789abcdef0123456789abcdef",
			ProxyBaseURL:    "https://proxy.example.test",
			TicketTTL:       time.Minute,
			LeaseTTL:        time.Minute,
			MaxTargets:      3,
			MaxConcurrent:   3,
			MaxOutputTokens: 512,
			HTTPClient:      catalogServer.Client(),
		},
	)
	if err != nil {
		t.Fatalf("create Model Lab service: %v", err)
	}

	return modelLabProxyFixture{
		config: config.ModelLabConfig{
			MaxPromptBytes:  32 << 10,
			MaxOutputTokens: 512,
			MaxTargets:      3,
			MaxConcurrent:   3,
		},
		db:      db,
		origin:  "https://app.example.test",
		service: service,
		userID:  record.ID,
		users:   userService,
	}
}

func (f modelLabProxyFixture) issueTicket(t *testing.T) modellab.IssuedTicket {
	t.Helper()
	user, err := f.users.UserByID(context.Background(), f.userID)
	if err != nil {
		t.Fatalf("load fixture user: %v", err)
	}
	issued, err := f.service.IssueTickets(
		context.Background(),
		user.ID,
		string(user.Role),
		f.origin,
		modellab.IssueTicketsInput{Targets: []modellab.TicketTarget{{ProviderName: "openai-test", Model: "gpt-test"}}},
		modellab.AccessFilter{},
	)
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	return issued.Tickets[0]
}

func (f modelLabProxyFixture) serve(ticket modellab.IssuedTicket, origin, payload string, next http.Handler) *httptest.ResponseRecorder {
	return f.serveMethod(ticket, http.MethodPost, origin, payload, next)
}

func (f modelLabProxyFixture) serveMethod(ticket modellab.IssuedTicket, method, origin, payload string, next http.Handler) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, ticket.Endpoint, strings.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+ticket.Token)
	request.Header.Set("Origin", origin)
	request.Header.Set("X-Untrusted-Provider-Header", "remove-me")
	request.Header.Set("X-Forwarded-For", "198.51.100.42")
	request.Header.Set("X-Real-IP", "198.51.100.43")
	request.Header.Set("Traceparent", "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01")
	handler := requestprivacy.Middleware(
		modelLabTicketMiddleware(f.config, f.service, f.users)(stripModelLabClientHeaders(next)),
	)
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestModelLabTicketMiddlewareRejectsNonCanonicalMethodBeforeUpstream(t *testing.T) {
	fixture := newModelLabProxyFixture(t)
	ticket := fixture.issueTicket(t)
	var calls atomic.Int64
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	payload := `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true}`

	recorder := fixture.serveMethod(ticket, "post", fixture.origin, payload, next)
	if recorder.Code != http.StatusUnauthorized || calls.Load() != 0 {
		t.Fatalf("non-canonical method bypassed ticket binding: status=%d calls=%d body=%s", recorder.Code, calls.Load(), recorder.Body.String())
	}
}

func TestModelLabTicketMiddlewareAuthenticatesNormalizesAndRejectsReplay(t *testing.T) {
	fixture := newModelLabProxyFixture(t)
	ticket := fixture.issueTicket(t)
	var calls atomic.Int64
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !requestprivacy.IsMetadataOnly(r.Context()) {
			t.Fatal("expected metadata-only request context")
		}
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok || principal.CredentialID != "model-lab" || principal.CredentialName != "Model Lab" {
			t.Fatalf("unexpected principal: %#v, ok=%v", principal, ok)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Origin") != "" || r.Header.Get("X-Untrusted-Provider-Header") != "" || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" || r.Header.Get("Traceparent") != "" {
			t.Fatalf("unsafe request headers survived: %#v", r.Header)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read normalized payload: %v", err)
		}
		var normalized map[string]any
		if err := json.Unmarshal(body, &normalized); err != nil {
			t.Fatalf("decode normalized payload: %v", err)
		}
		if normalized["model"] != "gpt-test" || normalized["stream"] != true || normalized["max_tokens"] != float64(128) {
			t.Fatalf("unexpected normalized payload: %s", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[]}\n\n")
	})
	payload := `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":128}`

	first := fixture.serve(ticket, fixture.origin, payload, next)
	if first.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("first request failed: status=%d body=%s calls=%d", first.Code, first.Body.String(), calls.Load())
	}
	if first.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("missing no-store response policy: %#v", first.Header())
	}

	replayed := fixture.serve(ticket, fixture.origin, payload, next)
	if replayed.Code != http.StatusUnauthorized || !strings.Contains(replayed.Body.String(), "invalid_playground_ticket") || calls.Load() != 1 {
		t.Fatalf("replay was not rejected: status=%d body=%s calls=%d", replayed.Code, replayed.Body.String(), calls.Load())
	}
}

func TestModelLabTicketMiddlewareDoesNotConsumeInvalidPayloadOrOrigin(t *testing.T) {
	fixture := newModelLabProxyFixture(t)
	validPayload := `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":128}`
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	payloadTicket := fixture.issueTicket(t)
	invalid := fixture.serve(payloadTicket, fixture.origin, `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true,"tools":[]}`, next)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid payload rejection, got %d: %s", invalid.Code, invalid.Body.String())
	}
	validAfterPayloadFix := fixture.serve(payloadTicket, fixture.origin, validPayload, next)
	if validAfterPayloadFix.Code != http.StatusNoContent {
		t.Fatalf("invalid payload consumed ticket: %d: %s", validAfterPayloadFix.Code, validAfterPayloadFix.Body.String())
	}

	originTicket := fixture.issueTicket(t)
	wrongOrigin := fixture.serve(originTicket, "https://evil.example.test", validPayload, next)
	if wrongOrigin.Code != http.StatusUnauthorized {
		t.Fatalf("expected origin rejection, got %d: %s", wrongOrigin.Code, wrongOrigin.Body.String())
	}
	validAfterOriginFix := fixture.serve(originTicket, fixture.origin, validPayload, next)
	if validAfterOriginFix.Code != http.StatusNoContent {
		t.Fatalf("wrong origin consumed ticket: %d: %s", validAfterOriginFix.Code, validAfterOriginFix.Body.String())
	}
}

func TestModelLabResponseWriterSanitizesHTTPErrorBodyAndHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "application/problem+json")
	recorder.Header().Set("Set-Cookie", "provider-secret=canary")
	recorder.Header().Set("X-Provider-Error", "secret-canary")
	w := &modelLabResponseWriter{ResponseWriter: recorder}

	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(`{"error":{"message":"secret-canary"}}`))
	w.finish()

	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "model_lab_upstream_unavailable") {
		t.Fatalf("unexpected sanitized response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-canary") || recorder.Header().Get("Set-Cookie") != "" || recorder.Header().Get("X-Provider-Error") != "" {
		t.Fatalf("provider error leaked: headers=%#v body=%s", recorder.Header(), recorder.Body.String())
	}
}

func TestModelLabResponseWriterDoesNotForwardProviderRedirects(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Location", "https://internal-provider.example/secret")
	w := &modelLabResponseWriter{ResponseWriter: recorder}
	w.WriteHeader(http.StatusTemporaryRedirect)
	_, _ = w.Write([]byte("provider-secret-canary"))
	w.finish()

	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "model_lab_upstream_unavailable") {
		t.Fatalf("unexpected redirect sanitization: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Location") != "" || strings.Contains(recorder.Body.String(), "provider-secret-canary") {
		t.Fatalf("provider redirect leaked: headers=%#v body=%s", recorder.Header(), recorder.Body.String())
	}
}

func TestModelLabResponseWriterDoesNotForwardNonStreamingSuccessBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "application/json")
	w := &modelLabResponseWriter{ResponseWriter: recorder}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"error":{"message":"provider-secret-canary"}}`))
	w.finish()

	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "model_lab_upstream_unavailable") {
		t.Fatalf("unexpected non-stream sanitization: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "provider-secret-canary") {
		t.Fatalf("provider JSON body leaked: %s", recorder.Body.String())
	}
}

func TestModelLabTicketMiddlewareRejectsRoleChange(t *testing.T) {
	fixture := newModelLabProxyFixture(t)
	ticket := fixture.issueTicket(t)
	if err := fixture.db.Model(&users.User{}).Where("id = ?", fixture.userID).Update("role", auth.RoleAdmin).Error; err != nil {
		t.Fatalf("change fixture role: %v", err)
	}
	var calls atomic.Int64
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	payload := `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true}`

	recorder := fixture.serve(ticket, fixture.origin, payload, next)
	if recorder.Code != http.StatusForbidden || calls.Load() != 0 {
		t.Fatalf("role-changed ticket reached upstream: status=%d calls=%d body=%s", recorder.Code, calls.Load(), recorder.Body.String())
	}
}

func TestModelLabResponseWriterSanitizesSplitSSEErrorEvents(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	recorder.Header().Set("Set-Cookie", "provider-secret=canary")
	w := &modelLabResponseWriter{ResponseWriter: recorder}
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"safe\"}}]}\n\n"))
	_, _ = w.Write([]byte("data: {\"error\":{\"message\":\"secret-"))
	_, _ = w.Write([]byte("canary\"}}\n\n"))
	_, _ = w.Write([]byte("event: error\r\ndata: {\"type\":\"error\",\"error\":{\"message\":\"upstream-stack\"}}\r\n\r\n"))
	_, _ = w.Write([]byte("data: {\"error\":\ndata: {\"message\":\"multi-line-canary\"}}\n\n"))
	_, _ = w.Write([]byte("data: {\"choices\":[]}\n\r\ndata: {\"error\":{\"message\":\"mixed-newline-canary-a\"}}\r\n\ndata: {\"error\":{\"message\":\"mixed-newline-canary-b\"}}\n\n"))
	w.finish()

	body := recorder.Body.String()
	if !strings.Contains(body, `"content":"safe"`) {
		t.Fatalf("normal SSE event was not preserved: %s", body)
	}
	if strings.Contains(body, "secret-canary") || strings.Contains(body, "upstream-stack") || strings.Contains(body, "multi-line-canary") || strings.Contains(body, "mixed-newline-canary") || recorder.Header().Get("Set-Cookie") != "" {
		t.Fatalf("SSE provider error leaked: headers=%#v body=%s", recorder.Header(), body)
	}
	if strings.Count(body, "model_lab_upstream_error") != 5 {
		t.Fatalf("expected all SSE errors to be sanitized: %s", body)
	}
}

func TestModelLabResponseWriterBoundsCumulativeStreamSize(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream")
	w := &modelLabResponseWriter{ResponseWriter: recorder, maxStreamBytes: 80}
	w.WriteHeader(http.StatusOK)

	frame := []byte("data: {\"choices\":[]}\n\n")
	for range 3 {
		if _, err := w.Write(frame); err != nil {
			t.Fatalf("write bounded frame: %v", err)
		}
	}
	if _, err := w.Write([]byte("data: provider-secret-canary\n\n")); !errors.Is(err, errModelLabSSEStreamTooLarge) {
		t.Fatalf("expected cumulative stream limit, got %v", err)
	}
	w.finish()

	body := recorder.Body.String()
	if strings.Contains(body, "provider-secret-canary") || !strings.Contains(body, "model_lab_upstream_error") {
		t.Fatalf("cumulative overflow was not safely replaced: %s", body)
	}
}

func TestModelLabStreamByteLimitIsBounded(t *testing.T) {
	if got := modelLabStreamByteLimit(1); got != minModelLabStreamBytes {
		t.Fatalf("small token limit: got %d, want %d", got, minModelLabStreamBytes)
	}
	if got := modelLabStreamByteLimit(1 << 30); got != hardMaxModelLabStreamBytes {
		t.Fatalf("large token limit: got %d, want %d", got, hardMaxModelLabStreamBytes)
	}
}

func TestModelLabResponseWriterBoundsIncompleteSSEFrames(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(fmt.Sprintf("complete_%t", complete), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			recorder.Header().Set("Content-Type", "text/event-stream")
			w := &modelLabResponseWriter{ResponseWriter: recorder}
			w.WriteHeader(http.StatusOK)

			payload := append([]byte("data: "), bytes.Repeat([]byte("x"), maxModelLabSSEFrameBytes)...)
			payload = append(payload, []byte("provider-secret-canary")...)
			if complete {
				payload = append(payload, '\n', '\n')
			}
			if _, err := w.Write(payload); !errors.Is(err, errModelLabSSEFrameTooLarge) {
				t.Fatalf("expected oversized frame error, got %v", err)
			}
			w.finish()

			body := recorder.Body.String()
			if strings.Contains(body, "provider-secret-canary") || !strings.Contains(body, "model_lab_upstream_error") {
				t.Fatalf("oversized frame was not safely replaced: %s", body)
			}
		})
	}
}

func TestModelLabBearerTokenRequiresDedicatedPrefix(t *testing.T) {
	for _, header := range []string{"", "Bearer regular-token", "Basic pgml_token", "Bearer"} {
		if _, ok := modelLabBearerToken(header); ok {
			t.Fatalf("unexpected Model Lab token match for %q", header)
		}
	}
	token, ok := modelLabBearerToken("Bearer pgml_token")
	if !ok || token != "pgml_token" {
		t.Fatalf("expected Model Lab token match, got %q ok=%v", token, ok)
	}
}

func TestStripModelLabClientHeadersLeavesOrdinaryTrafficUntouched(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-For") != "198.51.100.42" || r.Header.Get("Traceparent") == "" {
			t.Fatalf("ordinary proxy headers were modified: %#v", r.Header)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.42")
	req.Header.Set("Traceparent", "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01")
	rec := httptest.NewRecorder()
	stripModelLabClientHeaders(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestModelLabIssuedEndpointHasNoQuery(t *testing.T) {
	fixture := newModelLabProxyFixture(t)
	ticket := fixture.issueTicket(t)
	parsed, err := url.Parse(ticket.Endpoint)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		t.Fatalf("unsafe issued endpoint: %q err=%v", ticket.Endpoint, err)
	}
}
