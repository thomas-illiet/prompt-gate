package policy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/redisstore"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

type recordedDenial struct {
	accountID string
	clientIP  string
	reason    string
	deniedAt  time.Time
}

type denialRecorder struct {
	records []recordedDenial
}

func (r *denialRecorder) RecordPolicyDenial(_ context.Context, accountID, clientIP, reason string, deniedAt time.Time) error {
	r.records = append(r.records, recordedDenial{accountID: accountID, clientIP: clientIP, reason: reason, deniedAt: deniedAt})
	return nil
}

func response(status int, body string, req *http.Request) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req}
}

func TestClientDecisionContract(t *testing.T) {
	input := Input{
		Identity: IdentityInput{ID: "user-id", Type: "user", Role: "admin", Groups: []string{"developers", "platform"}, CredentialID: "key-id", CredentialName: "local"},
		Request:  RequestInput{ClientIP: "192.0.2.10", Method: http.MethodPost, Path: "/v1/responses", Host: "proxy.example.com"},
	}
	client, err := NewClient("http://opa:8181", "promptgate/proxy/decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/data/promptgate/proxy/decision" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected OPA request headers: %#v", req.Header)
		}
		payload, readErr := io.ReadAll(req.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(payload), "prompt") || !strings.Contains(string(payload), `"groups":["developers","platform"]`) || !strings.Contains(string(payload), `"credential_id":"key-id"`) {
			t.Fatalf("unexpected OPA payload: %s", payload)
		}
		return response(http.StatusOK, `{"result":{"allow":true,"reason":"administrator"}}`, req), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := client.Decide(context.Background(), input)
	if err != nil || !decision.Allow || decision.Reason != "administrator" {
		t.Fatalf("unexpected decision: %#v, %v", decision, err)
	}
}

func TestClientReportsTimeout(t *testing.T) {
	client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})})
	_, err := client.Decide(context.Background(), Input{})
	if !errors.Is(err, ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected unavailable timeout, got %v", err)
	}
}

func TestClientRejectsUnavailableAndInvalidResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"non-2xx", http.StatusInternalServerError, `{}`},
		{"missing result", http.StatusOK, `{}`},
		{"missing allow", http.StatusOK, `{"result":{"reason":"incomplete"}}`},
		{"missing reason", http.StatusOK, `{"result":{"allow":false}}`},
		{"invalid json", http.StatusOK, `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return response(tc.status, tc.body, req), nil
			})})
			_, err := client.Decide(context.Background(), Input{})
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("expected unavailable error, got %v", err)
			}
		})
	}
}

func TestClientHonorsCancellation(t *testing.T) {
	client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Decide(ctx, Input{})
	if !errors.Is(err, ErrUnavailable) || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected unavailable cancellation, got %v", err)
	}
}

func TestEvaluatorCachesAllowAndDenyUntilTTL(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		t.Run(map[bool]string{true: "allow", false: "deny"}[allowed], func(t *testing.T) {
			server := miniredis.RunT(t)
			store, err := redisstore.NewRequired(context.Background(), "redis://"+server.Addr()+"/0", time.Minute, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			var calls atomic.Int32
			body := `{"result":{"allow":false,"reason":"default_deny"}}`
			if allowed {
				body = `{"result":{"allow":true,"reason":"administrator"}}`
			}
			client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls.Add(1)
				return response(http.StatusOK, body, req), nil
			})})
			evaluator := NewEvaluator(client, store, "decision", 10*time.Minute, nil)
			input := Input{Identity: IdentityInput{ID: "identity"}, Request: RequestInput{Path: "/v1/responses"}}
			for range 2 {
				decision, evalErr := evaluator.Evaluate(context.Background(), input)
				if evalErr != nil || decision.Allow != allowed {
					t.Fatalf("unexpected decision %#v: %v", decision, evalErr)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("expected one OPA call for cache hit, got %d", calls.Load())
			}
			server.FastForward(10 * time.Minute)
			if _, evalErr := evaluator.Evaluate(context.Background(), input); evalErr != nil {
				t.Fatal(evalErr)
			}
			if calls.Load() != 2 {
				t.Fatalf("expected cache expiry to call OPA again, got %d", calls.Load())
			}
		})
	}
}

func TestEvaluatorFallsBackToOPAWhenRedisFails(t *testing.T) {
	server := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+server.Addr()+"/0", time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()
	client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"result":{"allow":true,"reason":"fallback"}}`, req), nil
	})})
	decision, err := NewEvaluator(client, store, "decision", time.Minute, nil).Evaluate(context.Background(), Input{})
	if err != nil || !decision.Allow {
		t.Fatalf("expected direct OPA fallback, got %#v: %v", decision, err)
	}
}

func TestCacheKeySeparatesInputsWithoutPlainIdentity(t *testing.T) {
	input := Input{Identity: IdentityInput{ID: "sensitive-user-id"}, Request: RequestInput{ClientIP: "10.0.0.1", Method: "POST", Path: "/v1/responses", Host: "proxy"}}
	first, err := CacheKey("a/b", input)
	if err != nil {
		t.Fatal(err)
	}
	input.Request.Path = "/v1/chat/completions"
	second, _ := CacheKey("a/b", input)
	if first == second || strings.Contains(first, "sensitive-user-id") {
		t.Fatalf("unexpected cache keys %q %q", first, second)
	}
}

func TestCacheKeySeparatesGroupMemberships(t *testing.T) {
	input := Input{Identity: IdentityInput{ID: "user-id", Groups: []string{"developers"}}}
	first, err := CacheKey("a/b", input)
	if err != nil {
		t.Fatal(err)
	}
	input.Identity.Groups = []string{"developers", "platform"}
	second, err := CacheKey("a/b", input)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("expected group membership change to produce a new policy cache key")
	}
}

func TestMiddlewareForwardsTechnicalGroupNames(t *testing.T) {
	store := groups.NewSnapshotStore(nil)
	if err := store.SetSnapshot(groups.Snapshot{
		KnownProviders: []string{}, ProviderTypes: map[string]provider.ProviderType{}, Users: map[string]groups.UserAccess{},
		UserGroups: map[string][]string{
			"user-id":    {"platform", "developers", "platform"},
			"service-id": {"automation"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		user auth.UserProfile
		want []string
	}{
		{name: "user", user: auth.UserProfile{ID: "user-id", Type: auth.UserTypeUser, Role: auth.RoleUser}, want: []string{"developers", "platform"}},
		{name: "service", user: auth.UserProfile{ID: "service-id", Type: auth.UserTypeService, Role: auth.RoleUser}, want: []string{"automation"}},
		{name: "admin without group", user: auth.UserProfile{ID: "admin-id", Type: auth.UserTypeUser, Role: auth.RoleAdmin}, want: []string{}},
		{name: "unknown identity", user: auth.UserProfile{ID: "unknown-id", Type: auth.UserTypeUser, Role: auth.RoleUser}, want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				var payload struct {
					Input Input `json:"input"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(payload.Input.Identity.Groups, tc.want) {
					t.Fatalf("unexpected groups: got %#v want %#v", payload.Input.Identity.Groups, tc.want)
				}
				return response(http.StatusOK, `{"result":{"allow":true,"reason":"test"}}`, req), nil
			})})
			handler := Middleware(NewEvaluator(client, nil, "decision", 0, nil), store, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			ctx := auth.ContextWithPrincipal(req.Context(), auth.Principal{User: tc.user})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req.WithContext(ctx))
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("unexpected status: %d", recorder.Code)
			}
		})
	}
}

func TestMiddlewareMapsDenyAndUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status, want     int
	}{
		{"deny", `{"result":{"allow":false,"reason":"default_deny"}}`, "policy_denied", http.StatusOK, http.StatusForbidden},
		{"allow", `{"result":{"allow":true,"reason":"allowed"}}`, "", http.StatusOK, http.StatusNoContent},
		{"unavailable", `{}`, "policy_unavailable", http.StatusOK, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return response(tc.status, tc.body, req), nil
			})})
			evaluator := NewEvaluator(client, nil, "decision", 0, nil)
			denials := &denialRecorder{}
			handler := Middleware(evaluator, nil, denials, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			ctx := auth.ContextWithPrincipal(req.Context(), auth.Principal{User: auth.UserProfile{ID: "user", Type: auth.UserTypeUser, Role: auth.RoleUser}, CredentialID: "key"})
			ctx = clientip.ContextWithClientIP(ctx, "10.0.0.1")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req.WithContext(ctx))
			if recorder.Code != tc.want || (tc.code != "" && !strings.Contains(recorder.Body.String(), tc.code)) {
				t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "default_deny") || strings.Contains(recorder.Body.String(), "key") {
				t.Fatalf("policy response leaked internal details: %s", recorder.Body.String())
			}
			if tc.name == "deny" {
				if len(denials.records) != 1 || denials.records[0].accountID != "user" || denials.records[0].clientIP != "10.0.0.1" || denials.records[0].reason != "default_deny" || denials.records[0].deniedAt.IsZero() {
					t.Fatalf("unexpected denial record: %#v", denials.records)
				}
			} else if len(denials.records) != 0 {
				t.Fatalf("unexpected denial record for %s policy: %#v", tc.name, denials.records)
			}
		})
	}
}

func TestMiddlewareRecordsCachedDenials(t *testing.T) {
	srv := miniredis.RunT(t)
	store, err := redisstore.NewRequired(context.Background(), "redis://"+srv.Addr(), time.Minute, nil)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	var calls atomic.Int32
	client, _ := NewClient("http://opa:8181", "decision", &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(http.StatusOK, `{"result":{"allow":false,"reason":"cached_deny"}}`, req), nil
	})})
	denials := &denialRecorder{}
	handler := Middleware(NewEvaluator(client, store, "decision", time.Minute, nil), nil, denials, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		ctx := auth.ContextWithPrincipal(req.Context(), auth.Principal{User: auth.UserProfile{ID: "user", Type: auth.UserTypeUser, Role: auth.RoleUser}})
		ctx = clientip.ContextWithClientIP(ctx, "10.0.0.1")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req.WithContext(ctx))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("unexpected status: %d", recorder.Code)
		}
	}
	if calls.Load() != 1 || len(denials.records) != 2 {
		t.Fatalf("expected one OPA call and two denial records, calls=%d records=%#v", calls.Load(), denials.records)
	}
}
