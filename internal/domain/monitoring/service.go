package monitoring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultIntervalSeconds = 60
	MinIntervalSeconds     = 30
	MaxIntervalSeconds     = 86400
	DefaultCheckTimeout    = 5 * time.Second
	DefaultSchedulerTick   = 15 * time.Second
	maxStoredErrorLength   = 512
)

var (
	ErrServiceNotFound = errors.New("monitoring service not found")
	ErrInvalidName     = errors.New("invalid_name")
	ErrInvalidURL      = errors.New("invalid_url")
	ErrInvalidStatus   = errors.New("invalid_status_code")
	ErrInvalidInterval = errors.New("invalid_interval")
	ErrNameConflict    = errors.New("name_conflict")
	ErrInvalidSort     = errors.New("invalid_sort")
)

var serviceNameRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Service struct {
	db         *gorm.DB
	httpClient *http.Client
}

// NewService creates a monitoring service backed by GORM.
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:         db,
		httpClient: &http.Client{Timeout: DefaultCheckTimeout},
	}
}

// SetHTTPClient configures the HTTP client used for service checks.
func (s *Service) SetHTTPClient(client *http.Client) {
	if client == nil {
		client = &http.Client{Timeout: DefaultCheckTimeout}
	}
	s.httpClient = client
}

// AutoMigrate migrates monitoring tables.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&MonitoringService{})
}

type CreateServiceInput struct {
	Name               string `json:"name"`
	DisplayName        string `json:"displayName"`
	URL                string `json:"url"`
	ExpectedStatusCode int    `json:"expectedStatusCode"`
	IntervalSeconds    int    `json:"intervalSeconds"`
	Enabled            bool   `json:"enabled"`
}

type UpdateServiceInput struct {
	Name               *string `json:"name,omitempty"`
	DisplayName        *string `json:"displayName,omitempty"`
	URL                *string `json:"url,omitempty"`
	ExpectedStatusCode *int    `json:"expectedStatusCode,omitempty"`
	IntervalSeconds    *int    `json:"intervalSeconds,omitempty"`
	Enabled            *bool   `json:"enabled,omitempty"`
}

type ServiceResponse struct {
	ID                  uuid.UUID  `json:"id"`
	Name                string     `json:"name"`
	DisplayName         string     `json:"displayName"`
	URL                 string     `json:"url"`
	ExpectedStatusCode  int        `json:"expectedStatusCode"`
	IntervalSeconds     int        `json:"intervalSeconds"`
	Enabled             bool       `json:"enabled"`
	Status              Status     `json:"status"`
	LastCheckedAt       *time.Time `json:"lastCheckedAt"`
	LastStatusCode      *int       `json:"lastStatusCode"`
	LastError           string     `json:"lastError"`
	LastLatencyMS       int64      `json:"lastLatencyMs"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type StatusServiceResponse struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	Status         Status     `json:"status"`
	LastCheckedAt  *time.Time `json:"lastCheckedAt"`
	LastStatusCode *int       `json:"lastStatusCode"`
	LastError      string     `json:"lastError"`
	LastLatencyMS  int64      `json:"lastLatencyMs"`
}

type StatusResponse struct {
	Status   Status                  `json:"status"`
	Services []StatusServiceResponse `json:"services"`
}

type ListParams struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

type ListResult struct {
	Items    []ServiceResponse `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int64             `json:"total"`
}

// ListServicesPaged returns monitoring services with pagination and sorting.
func (s *Service) ListServicesPaged(ctx context.Context, params ListParams) (ListResult, error) {
	normalizeListParams(&params)

	query := s.db.WithContext(ctx).Model(&MonitoringService{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("count monitoring services: %w", err)
	}

	var records []MonitoringService
	var err error
	query, err = applySort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ListResult{}, err
	}
	if err := query.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return ListResult{}, fmt.Errorf("list monitoring services: %w", err)
	}

	out := make([]ServiceResponse, len(records))
	for i, record := range records {
		out[i] = record.toResponse()
	}
	return ListResult{
		Items:    out,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// GetService returns one monitoring service in admin response form.
func (s *Service) GetService(ctx context.Context, id string) (ServiceResponse, error) {
	record, err := s.getService(ctx, s.db, id)
	if err != nil {
		return ServiceResponse{}, err
	}
	return record.toResponse(), nil
}

// CreateService validates and stores a monitoring service.
func (s *Service) CreateService(ctx context.Context, input CreateServiceInput) (ServiceResponse, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return ServiceResponse{}, err
	}
	serviceURL, err := validateURL(input.URL)
	if err != nil {
		return ServiceResponse{}, err
	}
	expectedStatusCode, err := validateExpectedStatusCode(input.ExpectedStatusCode)
	if err != nil {
		return ServiceResponse{}, err
	}
	intervalSeconds, err := normalizeCreateInterval(input.IntervalSeconds)
	if err != nil {
		return ServiceResponse{}, err
	}

	record := MonitoringService{
		Name:               name,
		DisplayName:        strings.TrimSpace(input.DisplayName),
		URL:                serviceURL,
		ExpectedStatusCode: expectedStatusCode,
		IntervalSeconds:    intervalSeconds,
		Enabled:            input.Enabled,
		Status:             StatusOK,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ServiceResponse{}, ErrNameConflict
		}
		return ServiceResponse{}, fmt.Errorf("create monitoring service: %w", err)
	}
	return record.toResponse(), nil
}

// UpdateService patches a monitoring service.
func (s *Service) UpdateService(ctx context.Context, id string, input UpdateServiceInput) (ServiceResponse, error) {
	var name string
	if input.Name != nil {
		parsed, err := validateName(*input.Name)
		if err != nil {
			return ServiceResponse{}, err
		}
		name = parsed
	}
	var serviceURL string
	if input.URL != nil {
		parsed, err := validateURL(*input.URL)
		if err != nil {
			return ServiceResponse{}, err
		}
		serviceURL = parsed
	}
	var expectedStatusCode int
	if input.ExpectedStatusCode != nil {
		parsed, err := validateExpectedStatusCode(*input.ExpectedStatusCode)
		if err != nil {
			return ServiceResponse{}, err
		}
		expectedStatusCode = parsed
	}
	var intervalSeconds int
	if input.IntervalSeconds != nil {
		parsed, err := validateInterval(*input.IntervalSeconds)
		if err != nil {
			return ServiceResponse{}, err
		}
		intervalSeconds = parsed
	}

	record, err := s.getService(ctx, s.db, id)
	if err != nil {
		return ServiceResponse{}, err
	}
	if input.Name != nil {
		record.Name = name
	}
	if input.DisplayName != nil {
		record.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.URL != nil {
		record.URL = serviceURL
	}
	if input.ExpectedStatusCode != nil {
		record.ExpectedStatusCode = expectedStatusCode
	}
	if input.IntervalSeconds != nil {
		record.IntervalSeconds = intervalSeconds
	}
	if input.Enabled != nil {
		record.Enabled = *input.Enabled
	}

	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ServiceResponse{}, ErrNameConflict
		}
		return ServiceResponse{}, fmt.Errorf("update monitoring service: %w", err)
	}
	return record.toResponse(), nil
}

// DeleteService deletes a monitoring service by id.
func (s *Service) DeleteService(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Delete(&MonitoringService{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete monitoring service: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrServiceNotFound
	}
	return nil
}

// CheckService runs one immediate HTTP check and persists the result.
func (s *Service) CheckService(ctx context.Context, id string) (ServiceResponse, error) {
	record, err := s.getService(ctx, s.db, id)
	if err != nil {
		return ServiceResponse{}, err
	}
	checked, err := s.checkRecord(ctx, record)
	if err != nil {
		return ServiceResponse{}, err
	}
	return checked.toResponse(), nil
}

// RunDueChecks checks every enabled service whose per-service interval has elapsed.
func (s *Service) RunDueChecks(ctx context.Context, now time.Time) (int, error) {
	records, err := s.ListEnabledDue(ctx, now)
	if err != nil {
		return 0, err
	}
	for _, record := range records {
		if _, err := s.checkRecord(ctx, record); err != nil {
			return 0, err
		}
	}
	return len(records), nil
}

// ListEnabledDue returns enabled services whose interval has elapsed.
func (s *Service) ListEnabledDue(ctx context.Context, now time.Time) ([]MonitoringService, error) {
	now = now.UTC()
	var records []MonitoringService
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("name ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list due monitoring services: %w", err)
	}

	out := make([]MonitoringService, 0, len(records))
	for _, record := range records {
		interval := time.Duration(record.intervalSeconds()) * time.Second
		if record.LastCheckedAt == nil || !record.LastCheckedAt.Add(interval).After(now) {
			out = append(out, record)
		}
	}
	return out, nil
}

// CurrentStatus returns currently degraded enabled monitoring services for app users.
func (s *Service) CurrentStatus(ctx context.Context) (StatusResponse, error) {
	var records []MonitoringService
	if err := s.db.WithContext(ctx).
		Where("enabled = ? AND status = ?", true, StatusDegraded).
		Order("name ASC").
		Find(&records).Error; err != nil {
		return StatusResponse{}, fmt.Errorf("load monitoring status: %w", err)
	}

	out := StatusResponse{
		Status:   StatusOK,
		Services: []StatusServiceResponse{},
	}
	if len(records) > 0 {
		out.Status = StatusDegraded
	}
	for _, record := range records {
		out.Services = append(out.Services, StatusServiceResponse{
			ID:             record.ID,
			Name:           record.Name,
			DisplayName:    record.DisplayName,
			Status:         record.Status,
			LastCheckedAt:  record.LastCheckedAt,
			LastStatusCode: record.LastStatusCode,
			LastError:      record.LastError,
			LastLatencyMS:  record.LastLatencyMS,
		})
	}
	return out, nil
}

// StartScheduler starts a background monitoring checker tied to the context lifetime.
func (s *Service) StartScheduler(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = DefaultSchedulerTick
	}

	go func() {
		s.runDueChecksLog(ctx)
		ticker := time.NewTicker(tick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runDueChecksLog(ctx)
			}
		}
	}()
}

// checkRecord performs a GET request and persists the resulting status.
func (s *Service) checkRecord(ctx context.Context, record MonitoringService) (MonitoringService, error) {
	startedAt := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, record.URL, nil)
	if err != nil {
		return MonitoringService{}, fmt.Errorf("build monitoring request: %w", err)
	}

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: DefaultCheckTimeout}
	}
	resp, err := client.Do(req)
	latencyMS := time.Since(startedAt).Milliseconds()
	checkedAt := time.Now().UTC()

	record.LastCheckedAt = &checkedAt
	record.LastLatencyMS = latencyMS
	record.LastStatusCode = nil
	record.LastError = ""

	if err != nil {
		record.Status = StatusDegraded
		record.LastError = truncateError(err.Error())
		record.ConsecutiveFailures++
		return record, s.saveCheckResult(ctx, record)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	statusCode := resp.StatusCode
	record.LastStatusCode = &statusCode
	if statusCode == record.ExpectedStatusCode {
		record.Status = StatusOK
		record.LastError = ""
		record.ConsecutiveFailures = 0
	} else {
		record.Status = StatusDegraded
		record.LastError = fmt.Sprintf("expected HTTP %d, got %d", record.ExpectedStatusCode, statusCode)
		record.ConsecutiveFailures++
	}
	return record, s.saveCheckResult(ctx, record)
}

// saveCheckResult persists every status field, including nil status codes.
func (s *Service) saveCheckResult(ctx context.Context, record MonitoringService) error {
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return fmt.Errorf("save monitoring check result: %w", err)
	}
	return nil
}

// runDueChecksLog runs monitoring checks and logs operational failures.
func (s *Service) runDueChecksLog(ctx context.Context) {
	count, err := s.RunDueChecks(ctx, time.Now().UTC())
	if err != nil {
		slog.Error("failed to run monitoring checks", "error", err)
		return
	}
	if count > 0 {
		slog.Info("monitoring checks completed", "services", count)
	}
}

// getService fetches a monitoring service or returns ErrServiceNotFound.
func (s *Service) getService(ctx context.Context, db *gorm.DB, id string) (MonitoringService, error) {
	var record MonitoringService
	if err := db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MonitoringService{}, ErrServiceNotFound
		}
		return MonitoringService{}, fmt.Errorf("get monitoring service: %w", err)
	}
	return record, nil
}

// normalizeListParams applies default monitoring pagination and sorting values.
func normalizeListParams(params *ListParams) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	if params.SortBy == "" {
		params.SortBy = "name"
	}
	if params.SortDir == "" {
		params.SortDir = "asc"
	}
}

// applySort applies a validated monitoring order to the query.
func applySort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"name":                "name",
		"displayName":         "display_name",
		"url":                 "url",
		"expectedStatusCode":  "expected_status_code",
		"intervalSeconds":     "interval_seconds",
		"enabled":             "enabled",
		"status":              "status",
		"lastCheckedAt":       "last_checked_at",
		"lastStatusCode":      "last_status_code",
		"lastLatencyMs":       "last_latency_ms",
		"consecutiveFailures": "consecutive_failures",
		"createdAt":           "created_at",
		"updatedAt":           "updated_at",
	}
	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	return query.Order(column + " " + dir).Order("id ASC"), nil
}

// normalizeSortDir converts a monitoring sort direction into SQL syntax.
func normalizeSortDir(sortDir string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sortDir)) {
	case "asc":
		return "ASC", nil
	case "desc":
		return "DESC", nil
	default:
		return "", ErrInvalidSort
	}
}

// validateName normalizes and validates a monitoring service name.
func validateName(raw string) (string, error) {
	name := normalizeName(raw)
	if !serviceNameRegexp.MatchString(name) {
		return "", ErrInvalidName
	}
	return name, nil
}

// validateURL validates an HTTP or HTTPS monitoring URL.
func validateURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ErrInvalidURL
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrInvalidURL
	}
	switch parsed.Scheme {
	case "http", "https":
		return value, nil
	default:
		return "", ErrInvalidURL
	}
}

// validateExpectedStatusCode checks whether an expected HTTP status code is valid.
func validateExpectedStatusCode(value int) (int, error) {
	if value == 0 {
		value = http.StatusOK
	}
	if value < 100 || value > 599 {
		return 0, ErrInvalidStatus
	}
	return value, nil
}

// normalizeCreateInterval applies the default interval for create requests.
func normalizeCreateInterval(value int) (int, error) {
	if value == 0 {
		return DefaultIntervalSeconds, nil
	}
	return validateInterval(value)
}

// validateInterval checks the per-service check interval.
func validateInterval(value int) (int, error) {
	if value < MinIntervalSeconds || value > MaxIntervalSeconds {
		return 0, ErrInvalidInterval
	}
	return value, nil
}

// intervalSeconds returns a safe interval value for legacy or malformed records.
func (s MonitoringService) intervalSeconds() int {
	if s.IntervalSeconds < MinIntervalSeconds || s.IntervalSeconds > MaxIntervalSeconds {
		return DefaultIntervalSeconds
	}
	return s.IntervalSeconds
}

// toResponse maps a database record to the admin response shape.
func (s MonitoringService) toResponse() ServiceResponse {
	return ServiceResponse{
		ID:                  s.ID,
		Name:                s.Name,
		DisplayName:         s.DisplayName,
		URL:                 s.URL,
		ExpectedStatusCode:  s.ExpectedStatusCode,
		IntervalSeconds:     s.IntervalSeconds,
		Enabled:             s.Enabled,
		Status:              s.Status,
		LastCheckedAt:       s.LastCheckedAt,
		LastStatusCode:      s.LastStatusCode,
		LastError:           s.LastError,
		LastLatencyMS:       s.LastLatencyMS,
		ConsecutiveFailures: s.ConsecutiveFailures,
		CreatedAt:           s.CreatedAt,
		UpdatedAt:           s.UpdatedAt,
	}
}

// truncateError limits stored operational errors to a compact value.
func truncateError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxStoredErrorLength {
		return value
	}
	return value[:maxStoredErrorLength]
}

// isUniqueConstraintError detects database uniqueness violations.
func isUniqueConstraintError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505")
}
