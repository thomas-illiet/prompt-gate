package monitoring

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// newTestService creates an in-memory monitoring service.
func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return service, db
}

func TestCreateServiceValidatesAndDefaults(t *testing.T) {
	service, _ := newTestService(t)

	created, err := service.CreateService(context.Background(), CreateServiceInput{
		Name:               " api-health ",
		DisplayName:        " API health ",
		URL:                "https://example.com/health",
		ExpectedStatusCode: 0,
		IntervalSeconds:    0,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	if created.Name != "api-health" {
		t.Fatalf("expected normalized name, got %q", created.Name)
	}
	if created.DisplayName != "API health" {
		t.Fatalf("expected trimmed display name, got %q", created.DisplayName)
	}
	if created.ExpectedStatusCode != http.StatusOK {
		t.Fatalf("expected default status 200, got %d", created.ExpectedStatusCode)
	}
	if created.IntervalSeconds != DefaultIntervalSeconds {
		t.Fatalf("expected default interval, got %d", created.IntervalSeconds)
	}
	if created.Status != StatusOK {
		t.Fatalf("expected initial ok status, got %q", created.Status)
	}
}

func TestCreateServiceRejectsInvalidInput(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	for _, test := range []struct {
		name string
		in   CreateServiceInput
		want error
	}{
		{
			name: "invalid name",
			in:   CreateServiceInput{Name: "Bad Name", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 60},
			want: ErrInvalidName,
		},
		{
			name: "invalid url",
			in:   CreateServiceInput{Name: "bad-url", URL: "ftp://example.com", ExpectedStatusCode: 200, IntervalSeconds: 60},
			want: ErrInvalidURL,
		},
		{
			name: "invalid status",
			in:   CreateServiceInput{Name: "bad-status", URL: "https://example.com", ExpectedStatusCode: 99, IntervalSeconds: 60},
			want: ErrInvalidStatus,
		},
		{
			name: "invalid interval",
			in:   CreateServiceInput{Name: "bad-interval", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 10},
			want: ErrInvalidInterval,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.CreateService(ctx, test.in); !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestCheckServicePersistsSuccessAndUnexpectedStatus(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	upstreamStatus := http.StatusAccepted
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(upstreamStatus)
	}))
	t.Cleanup(upstream.Close)

	created, err := service.CreateService(ctx, CreateServiceInput{
		Name:               "worker",
		URL:                upstream.URL,
		ExpectedStatusCode: http.StatusAccepted,
		IntervalSeconds:    60,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	checked, err := service.CheckService(ctx, created.ID.String())
	if err != nil {
		t.Fatalf("check service: %v", err)
	}
	if checked.Status != StatusOK || checked.LastStatusCode == nil || *checked.LastStatusCode != http.StatusAccepted {
		t.Fatalf("expected successful check, got %#v", checked)
	}
	if checked.ConsecutiveFailures != 0 || checked.LastError != "" {
		t.Fatalf("expected cleared failure state, got %#v", checked)
	}

	upstreamStatus = http.StatusBadGateway
	checked, err = service.CheckService(ctx, created.ID.String())
	if err != nil {
		t.Fatalf("check service again: %v", err)
	}
	if checked.Status != StatusDegraded {
		t.Fatalf("expected degraded status, got %#v", checked)
	}
	if checked.LastStatusCode == nil || *checked.LastStatusCode != http.StatusBadGateway {
		t.Fatalf("expected last status 502, got %#v", checked.LastStatusCode)
	}
	if checked.ConsecutiveFailures != 1 || !strings.Contains(checked.LastError, "expected HTTP 202, got 502") {
		t.Fatalf("expected failure metadata, got %#v", checked)
	}
}

func TestCheckServicePersistsNetworkError(t *testing.T) {
	service, _ := newTestService(t)
	service.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		}),
	})
	ctx := context.Background()
	created, err := service.CreateService(ctx, CreateServiceInput{
		Name:               "network",
		URL:                "https://network.example.com/health",
		ExpectedStatusCode: http.StatusOK,
		IntervalSeconds:    60,
		Enabled:            true,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	checked, err := service.CheckService(ctx, created.ID.String())
	if err != nil {
		t.Fatalf("check service: %v", err)
	}
	if checked.Status != StatusDegraded || checked.LastStatusCode != nil {
		t.Fatalf("expected network degradation without status code, got %#v", checked)
	}
	if checked.ConsecutiveFailures != 1 || !strings.Contains(checked.LastError, "dial failed") {
		t.Fatalf("expected network failure metadata, got %#v", checked)
	}
}

func TestListEnabledDueFiltersByIntervalAndEnabledState(t *testing.T) {
	service, db := newTestService(t)
	ctx := context.Background()
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	oldCheck := now.Add(-time.Minute)
	recentCheck := now.Add(-10 * time.Second)

	records := []MonitoringService{
		{Name: "due", URL: "https://due.example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, Enabled: true, LastCheckedAt: &oldCheck, Status: StatusOK},
		{Name: "never-checked", URL: "https://never.example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, Enabled: true, Status: StatusOK},
		{Name: "recent", URL: "https://recent.example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, Enabled: true, LastCheckedAt: &recentCheck, Status: StatusOK},
		{Name: "disabled", URL: "https://disabled.example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, Enabled: false, LastCheckedAt: &oldCheck, Status: StatusOK},
	}
	for _, record := range records {
		if err := db.WithContext(ctx).Create(&record).Error; err != nil {
			t.Fatalf("seed record: %v", err)
		}
	}

	due, err := service.ListEnabledDue(ctx, now)
	if err != nil {
		t.Fatalf("list due services: %v", err)
	}

	got := make([]string, 0, len(due))
	for _, record := range due {
		got = append(got, record.Name)
	}
	if strings.Join(got, ",") != "due,never-checked" {
		t.Fatalf("unexpected due services: %#v", got)
	}
}

func TestCurrentStatusOnlyReturnsEnabledDegradedServicesWithoutURLs(t *testing.T) {
	service, db := newTestService(t)
	ctx := context.Background()
	for _, record := range []MonitoringService{
		{Name: "degraded", DisplayName: "Degraded", URL: "https://secret.example.com/health", ExpectedStatusCode: 200, IntervalSeconds: 60, Enabled: true, Status: StatusDegraded, LastError: "expected HTTP 200, got 500"},
		{Name: "ok", URL: "https://ok.example.com/health", ExpectedStatusCode: 200, IntervalSeconds: 60, Enabled: true, Status: StatusOK},
		{Name: "disabled", URL: "https://disabled.example.com/health", ExpectedStatusCode: 200, IntervalSeconds: 60, Enabled: false, Status: StatusDegraded},
	} {
		if err := db.WithContext(ctx).Create(&record).Error; err != nil {
			t.Fatalf("seed record: %v", err)
		}
	}

	status, err := service.CurrentStatus(ctx)
	if err != nil {
		t.Fatalf("current status: %v", err)
	}
	if status.Status != StatusDegraded || len(status.Services) != 1 {
		t.Fatalf("unexpected status response: %#v", status)
	}
	if status.Services[0].Name != "degraded" || status.Services[0].DisplayName != "Degraded" {
		t.Fatalf("unexpected degraded service: %#v", status.Services[0])
	}
}
