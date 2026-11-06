package policy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/redisstore"
	"promptgate/backend/internal/platform/requestprivacy"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
func response(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("not JSON; ignored")), Header: make(http.Header)}
}
func testClient(t *testing.T, transport roundTripperFunc) *Client {
	t.Helper()
	client, err := NewClient("https://validation.example/ips/{ip}?scope=proxy", "service-token", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
func testRedis(t *testing.T) (*miniredis.Miniredis, *redisstore.Store) {
	t.Helper()
	server := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+server.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return server, store
}

func TestClientContractAndNormalization(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"192.0.2.10", "192.0.2.10"}, {"::ffff:192.0.2.10", "192.0.2.10"}, {"2001:0db8:0000::1", "2001:db8::1"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			client := testClient(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/ips/"+tc.want || req.URL.RawQuery != "scope=proxy" || req.Body != nil {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				if req.Header.Get("Authorization") != "Bearer service-token" || len(req.Header) != 1 {
					t.Fatalf("unexpected headers: %#v", req.Header)
				}
				return response(200), nil
			})
			decision, err := client.Decide(context.Background(), tc.input)
			if err != nil || !decision.Allow {
				t.Fatalf("decision=%+v error=%v", decision, err)
			}
		})
	}
}

func TestClientStatusContract(t *testing.T) {
	for _, status := range []int{200, 404, 201, 204, 301, 302, 307, 308, 400, 401, 403, 429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client := testClient(t, func(*http.Request) (*http.Response, error) { return response(status), nil })
			decision, err := client.Decide(context.Background(), "192.0.2.10")
			switch status {
			case 200:
				if err != nil || !decision.Allow {
					t.Fatalf("%+v %v", decision, err)
				}
			case 404:
				if err != nil || decision.Allow || decision.Reason != DeniedReason {
					t.Fatalf("%+v %v", decision, err)
				}
			default:
				if !errors.Is(err, ErrUnavailable) || !errors.Is(err, ErrInvalidResponse) {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

type unreadBody struct{ closed bool }

func (*unreadBody) Read([]byte) (int, error) { panic("response body must not be read") }
func (b *unreadBody) Close() error           { b.closed = true; return nil }
func TestClientDoesNotReadResponseBody(t *testing.T) {
	body := &unreadBody{}
	client := testClient(t, func(*http.Request) (*http.Response, error) { return &http.Response{StatusCode: 200, Body: body}, nil })
	decision, err := client.Decide(context.Background(), "192.0.2.10")
	if err != nil || !decision.Allow || !body.closed {
		t.Fatalf("%+v %v closed=%v", decision, err, body.closed)
	}
}

func TestClientRejectsInvalidIPBeforeNetwork(t *testing.T) {
	client := testClient(t, func(*http.Request) (*http.Response, error) { t.Fatal("invalid IP reached service"); return nil, nil })
	for _, ip := range []string{"", "hostname", "192.0.2.1:8080", "192.0.2.1/24", "../admin", "fe80::1%eth0"} {
		if _, err := client.Decide(context.Background(), ip); !errors.Is(err, ErrInvalidIP) {
			t.Fatalf("IP %q: %v", ip, err)
		}
	}
}

func TestClientNeverFollowsRedirects(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/secret-target", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	original := server.Client()
	client, err := NewClient(server.URL+"/ips/{ip}", "secret", original)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Decide(context.Background(), "192.0.2.1")
	if !errors.Is(err, ErrInvalidResponse) || calls.Load() != 1 || original.CheckRedirect != nil {
		t.Fatalf("redirect protection failed: calls=%d err=%v", calls.Load(), err)
	}
}

func TestClientSanitizesTransportErrorsAndHonorsCancellation(t *testing.T) {
	for _, cause := range []error{errors.New("secret-token https://secret-host/192.0.2.1"), context.DeadlineExceeded, context.Canceled} {
		client := testClient(t, func(*http.Request) (*http.Response, error) { return nil, cause })
		_, err := client.Decide(context.Background(), "192.0.2.1")
		if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "192.0.2.1") {
			t.Fatalf("unsafe error: %v", err)
		}
		if cause == context.DeadlineExceeded || cause == context.Canceled {
			if !errors.Is(err, cause) {
				t.Fatalf("lost cancellation category: %v", err)
			}
		}
	}
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.Decide(ctx, "192.0.2.1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestClientEnforcesConfiguredHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Keep the connection open without returning headers. Only the client's
		// configured timeout should interrupt this otherwise healthy connection.
		<-r.Context().Done()
	}))
	defer server.Close()
	httpClient := server.Client()
	httpClient.Timeout = 50 * time.Millisecond
	client, err := NewClient(server.URL+"/ips/{ip}", "service-token", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	decision, err := client.Decide(ctx, "192.0.2.1")
	if decision.Allow || !errors.Is(err, ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("slow service must fail closed: %+v %v", decision, err)
	}
	if ctx.Err() != nil {
		t.Fatal("request reached the caller deadline instead of the shorter HTTP timeout")
	}
}

func TestEvaluatorCachesOnlyAllowsWithFixedExpiry(t *testing.T) {
	for _, status := range []int{200, 404, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server, store := testRedis(t)
			calls := 0
			client := testClient(t, func(*http.Request) (*http.Response, error) { calls++; return response(status), nil })
			evaluator := NewEvaluator(client, store, 10*time.Minute, nil)
			_, _ = evaluator.Evaluate(context.Background(), "::ffff:192.0.2.1")
			server.FastForward(9 * time.Minute)
			_, _ = evaluator.Evaluate(context.Background(), "192.0.2.1")
			want := 2
			if status == 200 {
				want = 1
			}
			if calls != want {
				t.Fatalf("calls=%d want=%d", calls, want)
			}
			server.FastForward(time.Minute)
			_, _ = evaluator.Evaluate(context.Background(), "192.0.2.1")
			if calls != want+1 {
				t.Fatal("hit prolonged the cache TTL")
			}
			if status != 200 && len(server.Keys()) != 0 {
				t.Fatal("non-allow cached")
			}
		})
	}
}

func TestCachedAllowSurvivesOutageOnlyUntilExpiry(t *testing.T) {
	server, store := testRedis(t)
	status := 200
	calls := 0
	client := testClient(t, func(*http.Request) (*http.Response, error) { calls++; return response(status), nil })
	evaluator := NewEvaluator(client, store, 10*time.Minute, nil)
	_, _ = evaluator.Evaluate(context.Background(), "192.0.2.1")
	status = 503
	if decision, err := evaluator.Evaluate(context.Background(), "192.0.2.1"); err != nil || !decision.Allow || calls != 1 {
		t.Fatalf("valid allow lost: %+v %v", decision, err)
	}
	server.FastForward(10 * time.Minute)
	if _, err := evaluator.Evaluate(context.Background(), "192.0.2.1"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("expired allow survived outage")
	}
	status = 404
	if decision, err := evaluator.Evaluate(context.Background(), "192.0.2.1"); err != nil || decision.Allow {
		t.Fatal("revoked IP allowed")
	}
	status = 200
	if decision, err := evaluator.Evaluate(context.Background(), "192.0.2.1"); err != nil || !decision.Allow {
		t.Fatal("refusal was cached")
	}
}

func TestCacheIsScopedByIPAndServiceConfiguration(t *testing.T) {
	server, store := testRedis(t)
	calls := 0
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) { calls++; return response(200), nil })
	for _, tc := range []struct {
		url, token, ip string
		ttl            time.Duration
	}{
		{"https://service/ips/{ip}", "token-a", "192.0.2.1", time.Minute},
		{"https://service/ips/{ip}", "token-a", "192.0.2.2", time.Minute},
		{"https://other/ips/{ip}", "token-a", "192.0.2.1", time.Minute},
		{"https://service/ips/{ip}", "token-b", "192.0.2.1", time.Minute},
		{"https://service/ips/{ip}", "token-a", "192.0.2.1", 2 * time.Minute},
	} {
		client, err := NewClient(tc.url, tc.token, &http.Client{Transport: transport})
		if err != nil {
			t.Fatal(err)
		}
		evaluator := NewEvaluator(client, store, tc.ttl, nil)
		if _, err = evaluator.Evaluate(context.Background(), tc.ip); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 5 || len(server.Keys()) != 5 {
		t.Fatalf("shared incompatible entries: %d %v", calls, server.Keys())
	}
	for _, key := range server.Keys() {
		if strings.Contains(key, "token-") || strings.Contains(key, "192.0.2") || !strings.HasPrefix(key, "promptgate:proxy:ip-validation:v1:") {
			t.Fatalf("unsafe cache key: %s", key)
		}
	}
}

func TestEvaluatorIgnoresOldNegativeAndMalformedCache(t *testing.T) {
	server, store := testRedis(t)
	calls := 0
	client := testClient(t, func(*http.Request) (*http.Response, error) { calls++; return response(404), nil })
	evaluator := NewEvaluator(client, store, time.Minute, nil)
	if err := server.Set("promptgate:proxy:policy:old", `{"allow":true}`); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{`{"allow":false}`, `{broken`, `null`} {
		if err := server.Set(cacheKey(evaluator.cacheScope, "192.0.2.1"), value); err != nil {
			t.Fatal(err)
		}
		decision, err := evaluator.Evaluate(context.Background(), "192.0.2.1")
		if err != nil || decision.Allow {
			t.Fatalf("invalid cached decision trusted: %+v %v", decision, err)
		}
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestEvaluatorFallsBackOnRedisFailure(t *testing.T) {
	_, store := testRedis(t)
	_ = store.Close()
	calls := 0
	client := testClient(t, func(*http.Request) (*http.Response, error) { calls++; return response(200), nil })
	evaluator := NewEvaluator(client, store, time.Minute, nil)
	for range 2 {
		if decision, err := evaluator.Evaluate(context.Background(), "192.0.2.1"); err != nil || !decision.Allow {
			t.Fatalf("fallback: %+v %v", decision, err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

type recordedDenial struct{ accountID, ip, reason string }
type denialRecorder struct {
	records []recordedDenial
	err     error
}

func (r *denialRecorder) RecordPolicyDenial(_ context.Context, accountID, ip, reason string, _ time.Time) error {
	r.records = append(r.records, recordedDenial{accountID, ip, reason})
	return r.err
}
func principalContext() context.Context {
	return auth.ContextWithPrincipal(clientip.ContextWithClientIP(context.Background(), "192.0.2.1"), auth.Principal{User: auth.UserProfile{ID: "user"}, CredentialID: "model-lab", CredentialName: "Model Lab"})
}
func TestMiddlewareUsesOnlyValidatedTicketMarkerForBypass(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want int
	}{
		{"ordinary token named Model Lab", principalContext(), 403},
		{"privacy marker alone", requestprivacy.MarkMetadataOnly(principalContext()), 403},
		{"validated ticket", modellab.ContextWithValidatedTicket(principalContext()), 204},
		{"unauthenticated", context.Background(), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := testClient(t, func(*http.Request) (*http.Response, error) { calls++; return response(404), nil })
			recorder := &denialRecorder{}
			handler := Middleware(NewEvaluator(client, nil, time.Minute, nil), recorder, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
			req := httptest.NewRequest(http.MethodPost, "/provider/v1/chat", nil).WithContext(tc.ctx)
			req.Header.Set("Authorization", "Bearer pgml_forged")
			req.Header.Set("User-Agent", "PromptGate-ModelLab/1.0")
			req.Header.Set("X-Model-Lab", "true")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
			}
			wantCalls := 0
			if tc.want == 403 {
				wantCalls = 1
			}
			if calls != wantCalls || len(recorder.records) != wantCalls {
				t.Fatalf("calls=%d denials=%+v", calls, recorder.records)
			}
		})
	}
	// A bypass must not even dereference an evaluator or its Redis dependency.
	handler := Middleware(nil, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil).WithContext(modellab.ContextWithValidatedTicket(principalContext())))
	if rec.Code != 204 {
		t.Fatal(rec.Code)
	}
}

func TestMiddlewareStatusRecordingAndSafeLogs(t *testing.T) {
	for _, status := range []int{200, 404, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			client := testClient(t, func(*http.Request) (*http.Response, error) { return response(status), nil })
			recorder := &denialRecorder{err: errors.New("recording unavailable")}
			handler := Middleware(NewEvaluator(client, nil, time.Minute, logger), recorder, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
			for range 2 {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil).WithContext(principalContext()))
				want := map[int]int{200: 204, 404: 403, 503: 503}[status]
				if rec.Code != want {
					t.Fatalf("status=%d", rec.Code)
				}
				if status == 404 && strings.TrimSpace(rec.Body.String()) != `{"error":"ip_not_allowed"}` {
					t.Fatal(rec.Body.String())
				}
				if status == 503 && !strings.Contains(rec.Body.String(), "policy_unavailable") {
					t.Fatal(rec.Body.String())
				}
			}
			if status == 404 && (len(recorder.records) != 2 || recorder.records[0] != (recordedDenial{"user", "192.0.2.1", DeniedReason})) {
				t.Fatalf("denials=%+v", recorder.records)
			}
			if strings.Contains(logs.String(), "service-token") {
				t.Fatal("Bearer leaked")
			}
		})
	}
}
