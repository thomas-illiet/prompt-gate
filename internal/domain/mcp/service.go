package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"promptgate/backend/internal/platform/configevents"
	"promptgate/backend/internal/platform/secrets"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrServerNotFound = errors.New("mcp server not found")
	ErrInvalidName    = errors.New("invalid_name")
	ErrInvalidURL     = errors.New("invalid_url")
	ErrInvalidHeader  = errors.New("invalid_header")
	ErrInvalidRegex   = errors.New("invalid_regex")
	ErrNameConflict   = errors.New("name_conflict")
	ErrInvalidSort    = errors.New("invalid_sort")
)

var serverNameRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Service struct {
	db       *gorm.DB
	cipher   *secrets.Cipher
	notifier configevents.Notifier
}

// NewService creates an MCP service backed by GORM and the secrets cipher.
func NewService(db *gorm.DB, cipher *secrets.Cipher) *Service {
	return &Service{db: db, cipher: cipher, notifier: configevents.NoopNotifier{}}
}

// SetNotifier configures config event publication after MCP mutations.
func (s *Service) SetNotifier(notifier configevents.Notifier) {
	if notifier == nil {
		notifier = configevents.NoopNotifier{}
	}
	s.notifier = notifier
}

// AutoMigrate migrates MCP tables.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&MCPServer{})
}

type HeaderValue struct {
	Set   bool
	Value *string
}

// UnmarshalJSON tracks whether a header value was present in the request.
func (v *HeaderValue) UnmarshalJSON(data []byte) error {
	v.Set = true
	if bytes.Equal(data, []byte("null")) {
		v.Value = nil
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

type HeaderInput struct {
	Name      string      `json:"name"`
	Value     HeaderValue `json:"value"`
	Sensitive bool        `json:"sensitive"`
}

type HeaderResponse struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	Sensitive bool   `json:"sensitive"`
	HasValue  bool   `json:"hasValue"`
}

type CreateServerInput struct {
	Name         string        `json:"name"`
	DisplayName  string        `json:"displayName"`
	URL          string        `json:"url"`
	Headers      []HeaderInput `json:"headers"`
	AllowPattern string        `json:"allowPattern"`
	DenyPattern  string        `json:"denyPattern"`
	Enabled      bool          `json:"enabled"`
}

type UpdateServerInput struct {
	Name         *string        `json:"name,omitempty"`
	DisplayName  *string        `json:"displayName,omitempty"`
	URL          *string        `json:"url,omitempty"`
	Headers      *[]HeaderInput `json:"headers,omitempty"`
	AllowPattern *string        `json:"allowPattern,omitempty"`
	DenyPattern  *string        `json:"denyPattern,omitempty"`
	Enabled      *bool          `json:"enabled,omitempty"`
}

type ServerResponse struct {
	ID           uuid.UUID        `json:"id"`
	Name         string           `json:"name"`
	DisplayName  string           `json:"displayName"`
	URL          string           `json:"url"`
	Headers      []HeaderResponse `json:"headers"`
	AllowPattern string           `json:"allowPattern"`
	DenyPattern  string           `json:"denyPattern"`
	Enabled      bool             `json:"enabled"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type ListParams struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

type ListResult struct {
	Items    []ServerResponse `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int64            `json:"total"`
}

// ListServers returns all MCP servers in admin response form.
func (s *Service) ListServers(ctx context.Context) ([]ServerResponse, error) {
	result, err := s.ListServersPaged(ctx, ListParams{
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

// ListServersPaged returns MCP servers with pagination and sorting.
func (s *Service) ListServersPaged(ctx context.Context, params ListParams) (ListResult, error) {
	normalizeListParams(&params)

	query := s.db.WithContext(ctx).Model(&MCPServer{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("count mcp servers: %w", err)
	}

	var records []MCPServer
	if mcpSortInMemory(params.SortBy) {
		if _, err := normalizeSortDir(params.SortDir); err != nil {
			return ListResult{}, err
		}
		if err := query.Find(&records).Error; err != nil {
			return ListResult{}, fmt.Errorf("list mcp servers: %w", err)
		}
		sortMCPRecords(records, params.SortBy, params.SortDir)
		records = pageMCPRecords(records, params.Page, params.PageSize)
	} else {
		var err error
		query, err = applyMCPSort(query, params.SortBy, params.SortDir)
		if err != nil {
			return ListResult{}, err
		}
		if err := query.
			Offset((params.Page - 1) * params.PageSize).
			Limit(params.PageSize).
			Find(&records).Error; err != nil {
			return ListResult{}, fmt.Errorf("list mcp servers: %w", err)
		}
	}
	out := make([]ServerResponse, len(records))
	for i, record := range records {
		out[i] = s.toResponse(record)
	}
	return ListResult{
		Items:    out,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// ListEnabled returns enabled MCP servers for proxy runtime builds.
func (s *Service) ListEnabled(ctx context.Context) ([]MCPServer, error) {
	var records []MCPServer
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list enabled mcp servers: %w", err)
	}
	return records, nil
}

// normalizeListParams applies default MCP pagination and sorting values.
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

// mcpSortInMemory reports whether sorting depends on computed MCP fields.
func mcpSortInMemory(sortBy string) bool {
	return sortBy == "headers" || sortBy == "filters"
}

// applyMCPSort applies a validated database-backed MCP order.
func applyMCPSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"name":        "name",
		"displayName": "display_name",
		"url":         "url",
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

// sortMCPRecords orders MCP records by computed response fields.
func sortMCPRecords(records []MCPServer, sortBy, sortDir string) {
	desc := strings.EqualFold(sortDir, "desc")
	sort.SliceStable(records, func(i, j int) bool {
		left := mcpComputedSortValue(records[i], sortBy)
		right := mcpComputedSortValue(records[j], sortBy)
		if left == right {
			return records[i].ID.String() < records[j].ID.String()
		}
		if desc {
			return left > right
		}
		return left < right
	})
}

// mcpComputedSortValue returns the comparable value for in-memory MCP sorting.
func mcpComputedSortValue(record MCPServer, sortBy string) string {
	switch sortBy {
	case "headers":
		return fmt.Sprintf("%08d", len(record.Headers))
	case "filters":
		return strings.TrimSpace(record.AllowPattern + " " + record.DenyPattern)
	default:
		return ""
	}
}

// pageMCPRecords slices sorted MCP records for the requested page.
func pageMCPRecords(records []MCPServer, page, pageSize int) []MCPServer {
	start := (page - 1) * pageSize
	if start >= len(records) {
		return []MCPServer{}
	}
	end := start + pageSize
	if end > len(records) {
		end = len(records)
	}
	return records[start:end]
}

// normalizeSortDir converts an MCP sort direction into SQL syntax.
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

// GetServer returns one MCP server in admin response form.
func (s *Service) GetServer(ctx context.Context, id string) (ServerResponse, error) {
	record, err := s.getServer(ctx, s.db, id)
	if err != nil {
		return ServerResponse{}, err
	}
	return s.toResponse(record), nil
}

// CreateServer validates, encrypts sensitive headers, and stores a server.
func (s *Service) CreateServer(ctx context.Context, input CreateServerInput) (ServerResponse, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return ServerResponse{}, err
	}
	serverURL, err := validateURL(input.URL)
	if err != nil {
		return ServerResponse{}, err
	}
	if err := validateRegex(input.AllowPattern); err != nil {
		return ServerResponse{}, err
	}
	if err := validateRegex(input.DenyPattern); err != nil {
		return ServerResponse{}, err
	}
	headers, err := s.buildHeaders(input.Headers, nil)
	if err != nil {
		return ServerResponse{}, err
	}

	record := MCPServer{
		Name:         name,
		DisplayName:  strings.TrimSpace(input.DisplayName),
		URL:          serverURL,
		Headers:      headers,
		AllowPattern: strings.TrimSpace(input.AllowPattern),
		DenyPattern:  strings.TrimSpace(input.DenyPattern),
		Enabled:      input.Enabled,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ServerResponse{}, ErrNameConflict
		}
		return ServerResponse{}, fmt.Errorf("create mcp server: %w", err)
	}
	s.notifier.Notify(ctx, configevents.DomainMCP)
	return s.toResponse(record), nil
}

// UpdateServer patches a server while preserving omitted header values.
func (s *Service) UpdateServer(ctx context.Context, id string, input UpdateServerInput) (ServerResponse, error) {
	var name string
	if input.Name != nil {
		parsed, err := validateName(*input.Name)
		if err != nil {
			return ServerResponse{}, err
		}
		name = parsed
	}
	var serverURL string
	if input.URL != nil {
		parsed, err := validateURL(*input.URL)
		if err != nil {
			return ServerResponse{}, err
		}
		serverURL = parsed
	}
	if input.AllowPattern != nil {
		if err := validateRegex(*input.AllowPattern); err != nil {
			return ServerResponse{}, err
		}
	}
	if input.DenyPattern != nil {
		if err := validateRegex(*input.DenyPattern); err != nil {
			return ServerResponse{}, err
		}
	}

	record, err := s.getServer(ctx, s.db, id)
	if err != nil {
		return ServerResponse{}, err
	}
	if input.Name != nil {
		record.Name = name
	}
	if input.DisplayName != nil {
		record.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.URL != nil {
		record.URL = serverURL
	}
	if input.Headers != nil {
		headers, err := s.buildHeaders(*input.Headers, record.Headers)
		if err != nil {
			return ServerResponse{}, err
		}
		record.Headers = headers
	}
	if input.AllowPattern != nil {
		record.AllowPattern = strings.TrimSpace(*input.AllowPattern)
	}
	if input.DenyPattern != nil {
		record.DenyPattern = strings.TrimSpace(*input.DenyPattern)
	}
	if input.Enabled != nil {
		record.Enabled = *input.Enabled
	}

	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return ServerResponse{}, ErrNameConflict
		}
		return ServerResponse{}, fmt.Errorf("update mcp server: %w", err)
	}
	s.notifier.Notify(ctx, configevents.DomainMCP)
	return s.toResponse(record), nil
}

// DeleteServer deletes an MCP server by id.
func (s *Service) DeleteServer(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Delete(&MCPServer{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete mcp server: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrServerNotFound
	}
	s.notifier.Notify(ctx, configevents.DomainMCP)
	return nil
}

// HeadersForProxy decrypts and returns headers needed by the proxy.
func (s *Service) HeadersForProxy(server MCPServer) (map[string]string, error) {
	headers := make(map[string]string, len(server.Headers))
	for _, header := range server.Headers {
		value := header.Value
		if header.Sensitive && header.ValueCiphertext != "" {
			plain, err := s.cipher.Decrypt(header.ValueCiphertext)
			if err != nil {
				return nil, fmt.Errorf("decrypt mcp header %q: %w", header.Name, err)
			}
			value = plain
		}
		if value != "" {
			headers[header.Name] = value
		}
	}
	return headers, nil
}

// getServer fetches an MCP server or returns ErrNotFound.
func (s *Service) getServer(ctx context.Context, db *gorm.DB, id string) (MCPServer, error) {
	var record MCPServer
	if err := db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MCPServer{}, ErrServerNotFound
		}
		return MCPServer{}, fmt.Errorf("get mcp server: %w", err)
	}
	return record, nil
}

// buildHeaders validates and materializes structured MCP headers.
func (s *Service) buildHeaders(inputs []HeaderInput, existing MCPHeaders) (MCPHeaders, error) {
	existingByName := make(map[string]MCPHeader, len(existing))
	for _, header := range existing {
		existingByName[strings.ToLower(header.Name)] = header
	}

	out := make(MCPHeaders, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		name := httpHeaderName(input.Name)
		if name == "" {
			return nil, ErrInvalidHeader
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, ErrInvalidHeader
		}
		seen[key] = struct{}{}

		value, err := s.headerValue(input, existingByName[key])
		if err != nil {
			return nil, err
		}
		header := MCPHeader{Name: name, Sensitive: input.Sensitive}
		if input.Sensitive {
			if value != "" {
				ciphertext, err := s.cipher.Encrypt(value)
				if err != nil {
					return nil, err
				}
				header.ValueCiphertext = ciphertext
			}
		} else {
			header.Value = value
		}
		out = append(out, header)
	}
	return out, nil
}

// headerValue resolves clear or encrypted header storage for an input header.
func (s *Service) headerValue(input HeaderInput, existing MCPHeader) (string, error) {
	if input.Value.Set {
		if input.Value.Value == nil {
			return "", nil
		}
		return strings.TrimSpace(*input.Value.Value), nil
	}
	if existing.Name == "" {
		return "", nil
	}
	if existing.Sensitive {
		if existing.ValueCiphertext == "" {
			return "", nil
		}
		return s.cipher.Decrypt(existing.ValueCiphertext)
	}
	return existing.Value, nil
}

// toResponse redacts sensitive MCP header values for admin API output.
func (s *Service) toResponse(record MCPServer) ServerResponse {
	headers := make([]HeaderResponse, 0, len(record.Headers))
	for _, header := range record.Headers {
		response := HeaderResponse{
			Name:      header.Name,
			Sensitive: header.Sensitive,
			HasValue:  header.Value != "" || header.ValueCiphertext != "",
		}
		if !header.Sensitive {
			response.Value = header.Value
		}
		headers = append(headers, response)
	}
	return ServerResponse{
		ID:           record.ID,
		Name:         record.Name,
		DisplayName:  record.DisplayName,
		URL:          record.URL,
		Headers:      headers,
		AllowPattern: record.AllowPattern,
		DenyPattern:  record.DenyPattern,
		Enabled:      record.Enabled,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
}

// validateName normalizes and validates an MCP server name.
func validateName(raw string) (string, error) {
	name := strings.TrimSpace(strings.ToLower(raw))
	if !serverNameRegexp.MatchString(name) {
		return "", ErrInvalidName
	}
	return name, nil
}

// validateURL validates an HTTP or HTTPS MCP server URL.
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

// validateRegex validates an optional MCP allow or deny pattern.
func validateRegex(pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return nil
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return ErrInvalidRegex
	}
	return nil
}

// httpHeaderName canonicalizes an HTTP header name.
func httpHeaderName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" || strings.ContainsAny(name, " \t\r\n:") {
		return ""
	}
	return name
}

// isUniqueConstraintError detects database uniqueness violations.
func isUniqueConstraintError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505")
}
