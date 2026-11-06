package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"promptgate/backend/internal/domain/groups"
	"promptgate/backend/internal/domain/modellab"
	"promptgate/backend/internal/domain/policy"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/tokens"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/clientip"
	"promptgate/backend/internal/platform/redisstore"
)

const modelLabTestPayload = `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":128}`

func TestValidatedModelLabSkipsIPServiceAndCache(t *testing.T) {
	for _, status := range []int{404, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			fixture := newModelLabProxyFixture(t)
			ticket := fixture.issueTicket(t)
			cache := miniredis.RunT(t)
			store, err := redisstore.NewRequired(context.Background(), "redis://"+cache.Addr(), time.Minute, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			calls := 0
			client, err := policy.NewClient("https://validation.example/ips/{ip}", "secret", &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			evaluator := policy.NewEvaluator(client, store, time.Minute, nil)
			reached := 0
			next := tokens.MiddlewareWithOptions(tokens.MiddlewareOptions{})(clientip.Middleware(false)(policy.Middleware(evaluator, nil, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached++
				if !modellab.HasValidatedTicket(r.Context()) {
					t.Fatal("missing marker")
				}
				w.WriteHeader(204)
			}))))
			commands := cache.CommandCount()
			rec := fixture.serve(ticket, fixture.origin, modelLabTestPayload, next)
			if rec.Code != 204 || reached != 1 || calls != 0 || cache.CommandCount() != commands {
				t.Fatalf("bypass failed: status=%d reached=%d external=%d cache=%d/%d", rec.Code, reached, calls, cache.CommandCount(), commands)
			}
			replay := fixture.serve(ticket, fixture.origin, modelLabTestPayload, next)
			if replay.Code != 401 || reached != 1 || calls != 0 || cache.CommandCount() != commands {
				t.Fatal("replayed ticket bypassed authentication")
			}
		})
	}
}

func TestInvalidModelLabCannotReachIPBypass(t *testing.T) {
	for _, name := range []string{"expired", "forged", "wrong target", "wrong origin", "inactive account", "expired account", "concurrency"} {
		t.Run(name, func(t *testing.T) {
			fixture := newModelLabProxyFixture(t)
			ticket := fixture.issueTicket(t)
			origin := fixture.origin
			want := 401
			switch name {
			case "expired":
				*fixture.clock = fixture.clock.Add(2 * time.Minute)
			case "forged":
				ticket.Token += "forged"
			case "wrong target":
				ticket.Endpoint = strings.Replace(ticket.Endpoint, "openai-test", "other-provider", 1)
			case "wrong origin":
				origin = "https://evil.example"
			case "inactive account":
				want = 403
				if err := fixture.db.Model(&users.User{}).Where("id = ?", fixture.userID).Update("is_active", false).Error; err != nil {
					t.Fatal(err)
				}
			case "expired account":
				want = 403
				if err := fixture.db.Model(&users.User{}).Where("id = ?", fixture.userID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			case "concurrency":
				want = 429
				for range 3 {
					leaseTicket := fixture.issueTicket(t)
					claims, err := fixture.service.ParseTicket(leaseTicket.Token)
					if err != nil {
						t.Fatal(err)
					}
					if err = fixture.service.AcquireLease(context.Background(), claims); err != nil {
						t.Fatal(err)
					}
				}
			}
			called := false
			next := policy.Middleware(nil, nil, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
			rec := fixture.serve(ticket, origin, modelLabTestPayload, next)
			if rec.Code != want || called {
				t.Fatalf("invalid ticket reached bypass: status=%d expected=%d body=%s", rec.Code, want, rec.Body)
			}
		})
	}
}

func TestModelLabBypassPreservesGroupsAndQuotas(t *testing.T) {
	for _, name := range []string{"group denied", "quota exhausted", "allowed"} {
		t.Run(name, func(t *testing.T) {
			fixture := newModelLabProxyFixture(t)
			ctx := context.Background()
			service := subscriptions.NewService(fixture.db)
			if err := service.AutoMigrate(ctx); err != nil {
				t.Fatal(err)
			}
			quota := int64(10000)
			if _, err := service.CreatePlan(ctx, subscriptions.PlanInput{Name: "Default", IsDefault: true, Quota5HTokens: &quota, Quota7DTokens: &quota}); err != nil {
				t.Fatal(err)
			}
			quotaStore := subscriptions.NewRedisStore(fixture.store, service, time.Minute, nil)
			quotaStore.SyncVersion(ctx)
			if err := quotaStore.WarmSnapshot(ctx); err != nil {
				t.Fatal(err)
			}
			if name == "quota exhausted" {
				if err := quotaStore.IncrementUsage(ctx, fixture.userID, quota, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := groups.Snapshot{KnownProviders: []string{"openai-test"}, ProviderTypes: map[string]provider.ProviderType{"openai-test": provider.ProviderTypeOpenAI}, Users: map[string]groups.UserAccess{}}
			if name != "group denied" {
				snapshot.Users[fixture.userID] = groups.UserAccess{Rules: []groups.AccessRule{{Providers: []string{"openai-test"}, ModelPatterns: []string{"^gpt-test$"}}}}
			}
			access := groups.NewSnapshotStore(nil)
			if err := access.SetSnapshot(snapshot); err != nil {
				t.Fatal(err)
			}
			reached := false
			next := policy.Middleware(nil, nil, nil)(groups.Middleware(access, nil)(subscriptions.Middleware(quotaStore, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true; w.WriteHeader(204) }))))
			rec := fixture.serve(fixture.issueTicket(t), fixture.origin, modelLabTestPayload, next)
			want := map[string]int{"group denied": 403, "quota exhausted": 429, "allowed": 204}[name]
			if rec.Code != want || reached != (name == "allowed") {
				t.Fatalf("downstream checks bypassed: status=%d want=%d reached=%v body=%s", rec.Code, want, reached, rec.Body)
			}
		})
	}
}

func TestOrdinaryRequestWithModelLabHeadersStillRequiresIPValidation(t *testing.T) {
	fixture := newModelLabProxyFixture(t)
	user, err := fixture.users.UserByID(context.Background(), fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	// Use the real API-token path with a short-lived token whose display name
	// deliberately impersonates Model Lab.
	tokenService := tokens.NewService(fixture.db, "0123456789abcdef0123456789abcdef")
	if err = tokenService.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	days := 1
	issued, err := tokenService.CreateToken(context.Background(), user, "model-lab", "Model Lab", &days)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client, err := policy.NewClient("https://validation.example/ips/{ip}", "secret", &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	checked := policy.Middleware(policy.NewEvaluator(client, nil, time.Minute, nil), nil, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("ordinary token bypassed refusal") }))
	handler := modelLabTicketMiddleware(fixture.config, fixture.service, fixture.users)(tokens.MiddlewareWithOptions(tokens.MiddlewareOptions{TokenService: tokenService, UserResolver: fixture.users})(clientip.Middleware(false)(checked)))
	req := httptest.NewRequest("POST", "/openai-test/v1/chat/completions", strings.NewReader(modelLabTestPayload))
	req.Header.Set("Authorization", "Bearer "+issued.Token)
	req.Header.Set("User-Agent", "PromptGate-ModelLab/1.0")
	req.Header.Set("X-Model-Lab", "true")
	req.Header.Set("Origin", fixture.origin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 403 || calls != 1 || strings.TrimSpace(rec.Body.String()) != `{"error":"ip_not_allowed"}` {
		t.Fatalf("ordinary token escaped validation: %d calls=%d body=%s", rec.Code, calls, rec.Body)
	}
}
