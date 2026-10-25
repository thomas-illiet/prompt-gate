package monitoring

import (
	"context"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"promptgate/backend/internal/platform/httpclient"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes the test transport function.
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

// TestCreateServiceValidatesAndDefaults verifies create service validates and defaults.
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

// TestCreateServiceRejectsInvalidInput verifies create service rejects invalid input.
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

// TestCheckServicePersistsSuccessAndUnexpectedStatus verifies check service persists success and unexpected status.
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

// TestCheckServicePersistsNetworkError verifies check service persists network error.
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

// TestCheckServiceUsesConfiguredHTTPClientCA verifies monitoring checks use the injected CA-aware client.
func TestCheckServiceUsesConfiguredHTTPClientCA(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	caCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: upstream.Certificate().Raw,
	})
	if err := os.WriteFile(caFile, caCert, 0o600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}

	client, err := httpclient.NewWithCAFile(caFile, DefaultCheckTimeout)
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	service.SetHTTPClient(client)

	created, err := service.CreateService(ctx, CreateServiceInput{
		Name:               "tls",
		URL:                upstream.URL,
		ExpectedStatusCode: http.StatusNoContent,
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
	if checked.Status != StatusOK || checked.LastStatusCode == nil || *checked.LastStatusCode != http.StatusNoContent {
		t.Fatalf("expected successful TLS check, got %#v", checked)
	}
}

// TestListEnabledDueFiltersByIntervalAndEnabledState verifies list enabled due filters by interval and enabled state.
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

// TestCurrentStatusOnlyReturnsEnabledDegradedServicesWithoutURLs verifies current status only returns enabled degraded services without URLs.
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

func TestServiceCRUDPaginationSortingAndConflicts(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	alpha, err := service.CreateService(ctx, CreateServiceInput{
		Name: "alpha", DisplayName: " Alpha ", URL: "https://alpha.example.test/health",
		ExpectedStatusCode: http.StatusOK, IntervalSeconds: 60, Enabled: true,
	})
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	bravo, err := service.CreateService(ctx, CreateServiceInput{
		Name: "bravo", DisplayName: "Bravo", URL: "https://bravo.example.test/health",
		ExpectedStatusCode: http.StatusNoContent, IntervalSeconds: 120, Enabled: false,
	})
	if err != nil {
		t.Fatalf("create bravo: %v", err)
	}
	if _, err := service.CreateService(ctx, CreateServiceInput{
		Name: "alpha", URL: "https://duplicate.example.test", ExpectedStatusCode: 200, IntervalSeconds: 60,
	}); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("duplicate create error = %v, want %v", err, ErrNameConflict)
	}

	got, err := service.GetService(ctx, alpha.ID.String())
	if err != nil || got.Name != "alpha" {
		t.Fatalf("get alpha = %#v, %v", got, err)
	}
	if _, err := service.GetService(ctx, "missing"); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("missing get error = %v", err)
	}

	listed, err := service.ListServicesPaged(ctx, ListParams{})
	if err != nil {
		t.Fatalf("list defaults: %v", err)
	}
	if listed.Page != 1 || listed.PageSize != 10 || listed.Total != 2 || len(listed.Items) != 2 || listed.Items[0].Name != "alpha" {
		t.Fatalf("unexpected default list: %#v", listed)
	}
	listed, err = service.ListServicesPaged(ctx, ListParams{Page: 2, PageSize: 1, SortBy: "displayName", SortDir: "desc"})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Name != "alpha" {
		t.Fatalf("unexpected sorted page: %#v", listed)
	}
	if _, err := service.ListServicesPaged(ctx, ListParams{SortBy: "secret", SortDir: "asc"}); !errors.Is(err, ErrInvalidSort) {
		t.Fatalf("invalid sort field error = %v", err)
	}
	if _, err := service.ListServicesPaged(ctx, ListParams{SortBy: "name", SortDir: "sideways"}); !errors.Is(err, ErrInvalidSort) {
		t.Fatalf("invalid sort direction error = %v", err)
	}

	name := " CHARLIE "
	displayName := " Charlie API "
	serviceURL := "https://charlie.example.test/ready"
	statusCode := http.StatusAccepted
	interval := 300
	enabled := true
	updated, err := service.UpdateService(ctx, bravo.ID.String(), UpdateServiceInput{
		Name: &name, DisplayName: &displayName, URL: &serviceURL,
		ExpectedStatusCode: &statusCode, IntervalSeconds: &interval, Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("update bravo: %v", err)
	}
	if updated.Name != "charlie" || updated.DisplayName != "Charlie API" || updated.URL != serviceURL ||
		updated.ExpectedStatusCode != statusCode || updated.IntervalSeconds != interval || !updated.Enabled {
		t.Fatalf("unexpected update: %#v", updated)
	}

	duplicateName := "alpha"
	if _, err := service.UpdateService(ctx, updated.ID.String(), UpdateServiceInput{Name: &duplicateName}); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("duplicate update error = %v, want %v", err, ErrNameConflict)
	}
	if _, err := service.UpdateService(ctx, "missing", UpdateServiceInput{}); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("missing update error = %v", err)
	}

	if err := service.DeleteService(ctx, alpha.ID.String()); err != nil {
		t.Fatalf("delete alpha: %v", err)
	}
	if err := service.DeleteService(ctx, alpha.ID.String()); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("second delete error = %v", err)
	}
}

func TestUpdateServiceRejectsEveryInvalidField(t *testing.T) {
	service, _ := newTestService(t)
	created, err := service.CreateService(context.Background(), CreateServiceInput{
		Name: "valid", URL: "https://valid.example.test", ExpectedStatusCode: 200, IntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	invalidName := "bad name"
	invalidURL := "ftp://example.test"
	invalidStatus := 600
	invalidInterval := MaxIntervalSeconds + 1
	for name, test := range map[string]struct {
		input UpdateServiceInput
		want  error
	}{
		"name":     {input: UpdateServiceInput{Name: &invalidName}, want: ErrInvalidName},
		"url":      {input: UpdateServiceInput{URL: &invalidURL}, want: ErrInvalidURL},
		"status":   {input: UpdateServiceInput{ExpectedStatusCode: &invalidStatus}, want: ErrInvalidStatus},
		"interval": {input: UpdateServiceInput{IntervalSeconds: &invalidInterval}, want: ErrInvalidInterval},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.UpdateService(context.Background(), created.ID.String(), test.input); !errors.Is(err, test.want) {
				t.Fatalf("update error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestValidationAndNormalizationBoundaries(t *testing.T) {
	for _, raw := range []string{"", "://broken", "mailto:test@example.com"} {
		if _, err := validateURL(raw); !errors.Is(err, ErrInvalidURL) {
			t.Fatalf("validateURL(%q) error = %v", raw, err)
		}
	}
	if got, err := validateURL(" http://example.test/health "); err != nil || got != "http://example.test/health" {
		t.Fatalf("valid URL = %q, %v", got, err)
	}
	if _, err := validateInterval(MaxIntervalSeconds + 1); !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf("high interval error = %v", err)
	}

	params := ListParams{Page: -1, PageSize: 101}
	normalizeListParams(&params)
	if params.Page != 1 || params.PageSize != 100 || params.SortBy != "name" || params.SortDir != "asc" {
		t.Fatalf("normalized params = %#v", params)
	}
	if got, err := normalizeSortDir(" DESC "); err != nil || got != "DESC" {
		t.Fatalf("normalized sort = %q, %v", got, err)
	}

	for _, interval := range []int{0, MinIntervalSeconds - 1, MaxIntervalSeconds + 1} {
		record := MonitoringService{IntervalSeconds: interval}
		if got := record.intervalSeconds(); got != DefaultIntervalSeconds {
			t.Fatalf("safe interval for %d = %d", interval, got)
		}
	}
	if got := (MonitoringService{IntervalSeconds: MinIntervalSeconds}).intervalSeconds(); got != MinIntervalSeconds {
		t.Fatalf("valid interval = %d", got)
	}

	longError := strings.Repeat("x", maxStoredErrorLength+20)
	if got := truncateError("  " + longError + "  "); len(got) != maxStoredErrorLength {
		t.Fatalf("truncated error length = %d", len(got))
	}
	var record MonitoringService
	if err := record.BeforeCreate(nil); err != nil {
		t.Fatalf("before create: %v", err)
	}
	if record.ID.String() == "00000000-0000-0000-0000-000000000000" || record.Status != StatusOK {
		t.Fatalf("before create defaults = %#v", record)
	}
}

func TestRunDueChecksAndScheduler(t *testing.T) {
	service, _ := newTestService(t)
	checked := make(chan string, 4)
	service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		checked <- request.URL.Host
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    request,
		}, nil
	})})
	ctx := context.Background()
	for _, name := range []string{"alpha", "bravo"} {
		if _, err := service.CreateService(ctx, CreateServiceInput{
			Name: name, URL: "https://" + name + ".example.test/health",
			ExpectedStatusCode: http.StatusNoContent, IntervalSeconds: 60, Enabled: true,
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	count, err := service.RunDueChecks(ctx, time.Now().UTC())
	if err != nil || count != 2 {
		t.Fatalf("run due checks = %d, %v", count, err)
	}
	for range count {
		<-checked
	}
	status, err := service.CurrentStatus(ctx)
	if err != nil || status.Status != StatusOK || len(status.Services) != 0 {
		t.Fatalf("current healthy status = %#v, %v", status, err)
	}

	var record MonitoringService
	if err := service.db.Order("name ASC").First(&record).Error; err != nil {
		t.Fatalf("load record: %v", err)
	}
	zero := time.Time{}
	record.LastCheckedAt = &zero
	if err := service.db.Save(&record).Error; err != nil {
		t.Fatalf("make record due: %v", err)
	}
	schedulerCtx, cancel := context.WithCancel(context.Background())
	service.StartScheduler(schedulerCtx, time.Hour)
	select {
	case <-checked:
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("scheduler did not run its initial due check")
	}

	cancelledCtx, cancelImmediately := context.WithCancel(context.Background())
	cancelImmediately()
	service.StartScheduler(cancelledCtx, 0)
	time.Sleep(10 * time.Millisecond)
}

func TestRunDueChecksStopsOnInvalidStoredURL(t *testing.T) {
	service, db := newTestService(t)
	record := MonitoringService{
		Name: "broken", URL: "://broken", ExpectedStatusCode: 200,
		IntervalSeconds: 60, Enabled: true, Status: StatusOK,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed invalid service: %v", err)
	}
	if count, err := service.RunDueChecks(context.Background(), time.Now().UTC()); err == nil || count != 0 {
		t.Fatalf("run invalid due check = %d, %v", count, err)
	}
	if _, err := service.CheckService(context.Background(), "missing"); !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("missing check error = %v", err)
	}
}

func TestDatabaseErrorsAreWrapped(t *testing.T) {
	tests := map[string]func(*Service) error{
		"list": func(service *Service) error {
			_, err := service.ListServicesPaged(context.Background(), ListParams{})
			return err
		},
		"get": func(service *Service) error {
			_, err := service.GetService(context.Background(), "missing")
			return err
		},
		"create": func(service *Service) error {
			_, err := service.CreateService(context.Background(), CreateServiceInput{
				Name: "closed", URL: "https://closed.example.test", ExpectedStatusCode: 200, IntervalSeconds: 60,
			})
			return err
		},
		"delete": func(service *Service) error {
			return service.DeleteService(context.Background(), "missing")
		},
		"due": func(service *Service) error {
			_, err := service.ListEnabledDue(context.Background(), time.Now())
			return err
		},
		"status": func(service *Service) error {
			_, err := service.CurrentStatus(context.Background())
			return err
		},
		"save check": func(service *Service) error {
			return service.saveCheckResult(context.Background(), MonitoringService{Name: "closed"})
		},
	}
	for name, operation := range tests {
		t.Run(name, func(t *testing.T) {
			service, db := newTestService(t)
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatalf("database handle: %v", err)
			}
			if err := sqlDB.Close(); err != nil {
				t.Fatalf("close database: %v", err)
			}
			if err := operation(service); err == nil {
				t.Fatal("expected closed database error")
			}
		})
	}
}
