package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/secrets"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrInvalidName      = errors.New("invalid_name")
	ErrInvalidType      = errors.New("invalid_type")
	ErrInvalidURL       = errors.New("invalid_url")
	ErrNameConflict     = errors.New("name_conflict")
	ErrInvalidSort      = errors.New("invalid_sort")
)

var providerNameRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Service struct {
	db              *gorm.DB
	cipher          *secrets.Cipher
	notifier        configevents.Notifier
	modelHTTPClient *http.Client
}

// NewService creates a provider service backed by GORM and the secrets cipher.
func NewService(db *gorm.DB, cipher *secrets.Cipher) *Service {
	return &Service{
		db:              db,
		cipher:          cipher,
		notifier:        configevents.NoopNotifier{},
		modelHTTPClient: &http.Client{Timeout: 3 * time.Second},
	}
}

// SetNotifier configures config event publication after provider mutations.
func (s *Service) SetNotifier(notifier configevents.Notifier) {
	if notifier == nil {
		notifier = configevents.NoopNotifier{}
	}
	s.notifier = notifier
}

// SetModelHTTPClient configures the HTTP client used for upstream model discovery.
func (s *Service) SetModelHTTPClient(client *http.Client) {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	s.modelHTTPClient = client
}

// AutoMigrate migrates provider tables.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&Provider{})
}

type CreateProviderInput struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName"`
	Type        ProviderType   `json:"type"`
	BaseURL     string         `json:"baseUrl"`
	APIKey      string         `json:"apiKey"`
	Config      ProviderConfig `json:"config"`
	Enabled     bool           `json:"enabled"`
}

type UpdateProviderInput struct {
	Name        *string         `json:"name,omitempty"`
	DisplayName *string         `json:"displayName,omitempty"`
	Type        *ProviderType   `json:"type,omitempty"`
	BaseURL     *string         `json:"baseUrl,omitempty"`
	APIKey      *string         `json:"apiKey,omitempty"`
	Config      *ProviderConfig `json:"config,omitempty"`
	Enabled     *bool           `json:"enabled,omitempty"`
}

type ProviderResponse struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName"`
	Type        ProviderType   `json:"type"`
	BaseURL     string         `json:"baseUrl"`
	HasAPIKey   bool           `json:"hasApiKey"`
	Config      ProviderConfig `json:"config"`
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type ListParams struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

type ListResult struct {
	Items    []ProviderResponse `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Total    int64              `json:"total"`
}

type HelpSetupResponse struct {
	ProxyBaseURL string              `json:"proxyBaseUrl"`
	Providers    []HelpSetupProvider `json:"providers"`
}

type HelpSetupProvider struct {
	Name             string       `json:"name"`
	DisplayName      string       `json:"displayName"`
	Type             ProviderType `json:"type"`
	RoutePrefix      string       `json:"routePrefix"`
	OpenAIBaseURL    string       `json:"openaiBaseUrl,omitempty"`
	AnthropicBaseURL string       `json:"anthropicBaseUrl,omitempty"`
	Models           []string     `json:"models"`
	ModelsError      string       `json:"modelsError,omitempty"`
}

// ListProviders returns all providers in admin response form.
func (s *Service) ListProviders(ctx context.Context) ([]ProviderResponse, error) {
	result, err := s.ListProvidersPaged(ctx, ListParams{
		Page:     1,
		PageSize: 100,
		SortBy:   "name",
		SortDir:  "asc",
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListProvidersPaged returns providers with pagination and sorting.
func (s *Service) ListProvidersPaged(ctx context.Context, params ListParams) (ListResult, error) {
	normalizeListParams(&params)

	query := s.db.WithContext(ctx).Model(&Provider{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("count providers: %w", err)
	}

	var records []Provider
	var err error
	query, err = applyProviderSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ListResult{}, err
	}
	if err := query.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return ListResult{}, fmt.Errorf("list providers: %w", err)
	}
	out := make([]ProviderResponse, len(records))
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

// ListEnabled returns enabled providers for proxy runtime builds.
func (s *Service) ListEnabled(ctx context.Context) ([]Provider, error) {
	var records []Provider
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list enabled providers: %w", err)
	}
	return records, nil
}

// HelpSetup returns enabled provider metadata and best-effort upstream model lists.
func (s *Service) HelpSetup(ctx context.Context, proxyBaseURL string) (HelpSetupResponse, error) {
	records, err := s.ListEnabled(ctx)
	if err != nil {
		return HelpSetupResponse{}, err
	}

	proxyBaseURL = strings.TrimRight(strings.TrimSpace(proxyBaseURL), "/")
	out := HelpSetupResponse{
		ProxyBaseURL: proxyBaseURL,
		Providers:    make([]HelpSetupProvider, 0, len(records)),
	}

	for _, record := range records {
		item := HelpSetupProvider{
			Name:        record.Name,
			DisplayName: record.DisplayName,
			Type:        record.Type,
			RoutePrefix: routePrefix(record),
			Models:      []string{},
		}
		switch record.Type {
		case ProviderTypeOpenAI, ProviderTypeOllama:
			item.OpenAIBaseURL = proxyBaseURL + item.RoutePrefix
		case ProviderTypeAnthropic:
			item.AnthropicBaseURL = proxyBaseURL + item.RoutePrefix
		}

		models, err := s.fetchProviderModels(ctx, record)
		if err != nil {
			item.ModelsError = err.Error()
		} else {
			item.Models = models
		}
		out.Providers = append(out.Providers, item)
	}

	return out, nil
}

// normalizeListParams applies default provider pagination and sorting values.
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

// applyProviderSort applies a validated provider order to the query.
func applyProviderSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"name":        "name",
		"displayName": "display_name",
		"type":        "type",
		"hasApiKey":   "CASE WHEN api_key_ciphertext <> '' THEN 1 ELSE 0 END",
		"enabled":     "enabled",
		"createdAt":   "created_at",
		"updatedAt":   "updated_at",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	return query.Order(column + " " + dir).Order("id ASC"), nil
}

// normalizeSortDir converts a provider sort direction into SQL syntax.
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

// GetProvider returns one provider in admin response form.
func (s *Service) GetProvider(ctx context.Context, id string) (ProviderResponse, error) {
	record, err := s.getProvider(ctx, s.db, id)
	if err != nil {
		return ProviderResponse{}, err
	}
	return record.toResponse(), nil
}

// CreateProvider validates, encrypts secrets, and stores a provider.
func (s *Service) CreateProvider(ctx context.Context, input CreateProviderInput) (ProviderResponse, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return ProviderResponse{}, err
	}
	if err := validateType(input.Type); err != nil {
		return ProviderResponse{}, err
	}
	baseURL, err := validateURL(input.BaseURL)
	if err != nil {
		return ProviderResponse{}, err
	}
	config := input.Config
	if config == nil {
		config = ProviderConfig{}
	}
	apiKeyCiphertext, err := s.encryptOptional(input.APIKey)
	if err != nil {
		return ProviderResponse{}, err
	}

	record := Provider{
		Name:             name,
		DisplayName:      strings.TrimSpace(input.DisplayName),
		Type:             input.Type,
		BaseURL:          baseURL,
		APIKeyCiphertext: apiKeyCiphertext,
		Config:           config,
		Enabled:          input.Enabled,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ProviderResponse{}, ErrNameConflict
		}
		return ProviderResponse{}, fmt.Errorf("create provider: %w", err)
	}
	s.notifier.Notify(ctx, configevents.DomainProviders)
	return record.toResponse(), nil
}

// UpdateProvider patches a provider while preserving omitted secrets.
func (s *Service) UpdateProvider(ctx context.Context, id string, input UpdateProviderInput) (ProviderResponse, error) {
	var name string
	if input.Name != nil {
		parsed, err := validateName(*input.Name)
		if err != nil {
			return ProviderResponse{}, err
		}
		name = parsed
	}
	if input.Type != nil {
		if err := validateType(*input.Type); err != nil {
			return ProviderResponse{}, err
		}
	}
	var baseURL string
	if input.BaseURL != nil {
		parsed, err := validateURL(*input.BaseURL)
		if err != nil {
			return ProviderResponse{}, err
		}
		baseURL = parsed
	}
	var apiKeyCiphertext string
	var err error
	if input.APIKey != nil {
		apiKeyCiphertext, err = s.encryptOptional(*input.APIKey)
		if err != nil {
			return ProviderResponse{}, err
		}
	}

	record, err := s.getProvider(ctx, s.db, id)
	if err != nil {
		return ProviderResponse{}, err
	}
	if input.Name != nil {
		record.Name = name
	}
	if input.DisplayName != nil {
		record.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.Type != nil {
		record.Type = *input.Type
	}
	if input.BaseURL != nil {
		record.BaseURL = baseURL
	}
	if input.APIKey != nil {
		record.APIKeyCiphertext = apiKeyCiphertext
	}
	if input.Config != nil {
		if *input.Config == nil {
			record.Config = ProviderConfig{}
		} else {
			record.Config = *input.Config
		}
	}
	if input.Enabled != nil {
		record.Enabled = *input.Enabled
	}

	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ProviderResponse{}, ErrNameConflict
		}
		return ProviderResponse{}, fmt.Errorf("update provider: %w", err)
	}
	s.notifier.Notify(ctx, configevents.DomainProviders)
	return record.toResponse(), nil
}

// DeleteProvider deletes a provider by id.
func (s *Service) DeleteProvider(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Delete(&Provider{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete provider: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrProviderNotFound
	}
	s.notifier.Notify(ctx, configevents.DomainProviders)
	return nil
}

// DecryptAPIKey decrypts the provider API key for proxy use.
func (s *Service) DecryptAPIKey(record Provider) (string, error) {
	if strings.TrimSpace(record.APIKeyCiphertext) == "" {
		return "", nil
	}
	return s.cipher.Decrypt(record.APIKeyCiphertext)
}

// fetchProviderModels fetches available upstream models for setup help.
func (s *Service) fetchProviderModels(ctx context.Context, record Provider) ([]string, error) {
	endpoint, err := modelsEndpoint(record)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build model request: %w", err)
	}
	key, err := s.DecryptAPIKey(record)
	if err != nil {
		return nil, fmt.Errorf("decrypt provider key: %w", err)
	}
	switch record.Type {
	case ProviderTypeOpenAI, ProviderTypeOllama:
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	case ProviderTypeAnthropic:
		if key != "" {
			req.Header.Set("x-api-key", key)
		}
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	client := s.modelHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch models returned %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
		Models []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}

	models := make([]string, 0, len(payload.Data)+len(payload.Models))
	for _, model := range payload.Data {
		models = appendModelID(models, model.ID, model.Name, model.DisplayName)
	}
	for _, model := range payload.Models {
		models = appendModelID(models, model.ID, model.Name, model.DisplayName)
	}
	return models, nil
}

// appendModelID appends the first non-empty model identifier from the values.
func appendModelID(models []string, values ...string) []string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return append(models, value)
		}
	}
	return models
}

// routePrefix returns the proxy route prefix for a provider.
func routePrefix(record Provider) string {
	switch record.Type {
	case ProviderTypeOpenAI, ProviderTypeOllama:
		return "/" + record.Name + "/v1"
	default:
		return "/" + record.Name
	}
}

// modelsEndpoint returns the provider-specific upstream models endpoint.
func modelsEndpoint(record Provider) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(record.BaseURL), "/")
	if base == "" {
		return "", ErrInvalidURL
	}
	if record.Type == ProviderTypeAnthropic {
		if strings.HasSuffix(base, "/v1") {
			return base + "/models", nil
		}
		return base + "/v1/models", nil
	}
	return base + "/models", nil
}

// getProvider fetches a provider or returns ErrNotFound.
func (s *Service) getProvider(ctx context.Context, db *gorm.DB, id string) (Provider, error) {
	var record Provider
	if err := db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Provider{}, ErrProviderNotFound
		}
		return Provider{}, fmt.Errorf("get provider: %w", err)
	}
	return record, nil
}

// encryptOptional encrypts a non-empty secret value.
func (s *Service) encryptOptional(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return s.cipher.Encrypt(value)
}

// toResponse redacts provider secrets for admin API output.
func (p *Provider) toResponse() ProviderResponse {
	config := p.Config
	if config == nil {
		config = ProviderConfig{}
	}
	return ProviderResponse{
		ID:          p.ID,
		Name:        p.Name,
		DisplayName: p.DisplayName,
		Type:        p.Type,
		BaseURL:     p.BaseURL,
		HasAPIKey:   strings.TrimSpace(p.APIKeyCiphertext) != "",
		Config:      config,
		Enabled:     p.Enabled,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// validateName normalizes and validates a provider name.
func validateName(raw string) (string, error) {
	name := strings.TrimSpace(strings.ToLower(raw))
	if !providerNameRegexp.MatchString(name) {
		return "", ErrInvalidName
	}
	return name, nil
}

// validateType checks whether a provider type is accepted.
func validateType(t ProviderType) error {
	switch t {
	case ProviderTypeOpenAI, ProviderTypeAnthropic, ProviderTypeOllama:
		return nil
	default:
		return ErrInvalidType
	}
}

// validateURL validates an HTTP or HTTPS provider base URL.
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
		return strings.TrimRight(value, "/"), nil
	default:
		return "", ErrInvalidURL
	}
}

// isUniqueConstraintError detects database uniqueness violations.
func isUniqueConstraintError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505")
}
