package tokens

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrTokenNotFound    = errors.New("token not found")
	ErrTokenRevoked     = errors.New("token revoked")
	ErrTokenExpired     = errors.New("token expired")
	ErrInvalidToken     = errors.New("invalid token")
	ErrAccountInactive  = errors.New("account inactive")
	ErrInsufficientRole = errors.New("insufficient role")
	ErrInvalidName      = errors.New("name must be lowercase alphanumeric with dashes or underscores, max 64 chars")
	ErrInvalidTTL       = errors.New("expires_in_days is outside the allowed range")
	ErrInvalidSort      = errors.New("invalid_sort")
)

var nameRegexp = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

const (
	maxUserTokenTTLInDays           = 30
	maxServiceAccountTokenTTLInDays = 365
)
const tokenSearchCondition = "LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(CAST(id AS TEXT)) LIKE ?"

type Service struct {
	db        *gorm.DB
	jwtSecret []byte
	notifier  configevents.Notifier
}

type UserResolver interface {
	UserByID(ctx context.Context, id string) (auth.UserProfile, error)
}

type ListParams struct {
	Page           int
	PageSize       int
	Search         string
	Status         string
	SortBy         string
	SortDir        string
	IncludeRevoked bool
}

type ListResult struct {
	Items    []TokenResponse `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Total    int64           `json:"total"`
}

// NewService creates a token service using the configured JWT secret.
func NewService(db *gorm.DB, secret string) *Service {
	return &Service{db: db, jwtSecret: []byte(secret), notifier: configevents.NoopNotifier{}}
}

// SetNotifier configures config event publication after token mutations.
func (s *Service) SetNotifier(notifier configevents.Notifier) {
	if notifier == nil {
		notifier = configevents.NoopNotifier{}
	}
	s.notifier = notifier
}

// AutoMigrate migrates token tables.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&Token{})
}

type tokenClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
	Name string `json:"name"`
}

// ttlForRole returns the default token lifetime for an app role.
func ttlForRole(role auth.AppRole) time.Duration {
	if role == auth.RoleManager || role == auth.RoleAdmin {
		return 30 * 24 * time.Hour
	}
	return 7 * 24 * time.Hour
}

// maxTTLInDaysForUser returns the maximum requested token lifetime for an account type.
func maxTTLInDaysForUser(user auth.UserProfile) int {
	if user.Type == auth.UserTypeService {
		return maxServiceAccountTokenTTLInDays
	}
	return maxUserTokenTTLInDays
}

// ttlFromRequest returns the requested token lifetime or the role default.
func ttlFromRequest(user auth.UserProfile, expiresInDays *int) (time.Duration, error) {
	if expiresInDays == nil {
		return ttlForRole(user.Role), nil
	}
	if *expiresInDays < 1 || *expiresInDays > maxTTLInDaysForUser(user) {
		return 0, ErrInvalidTTL
	}

	return time.Duration(*expiresInDays) * 24 * time.Hour, nil
}

// sha256hex returns a SHA-256 hex digest.
func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// CreateToken generates a signed JWT, hashes it, and stores the record.
func (s *Service) CreateToken(ctx context.Context, user auth.UserProfile, name, description string, expiresInDays *int) (CreateTokenResponse, error) {
	if !nameRegexp.MatchString(name) {
		return CreateTokenResponse{}, ErrInvalidName
	}
	ttl, err := ttlFromRequest(user, expiresInDays)
	if err != nil {
		return CreateTokenResponse{}, err
	}

	jti := uuid.NewString()
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Role: string(user.Role),
		Name: name,
	}

	rawToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return CreateTokenResponse{}, err
	}

	record := Token{
		UserID:      user.ID,
		Name:        name,
		Description: description,
		TokenHash:   sha256hex(rawToken),
		ExpiresAt:   expiresAt,
	}

	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return CreateTokenResponse{}, err
	}

	return CreateTokenResponse{
		Token:     rawToken,
		TokenInfo: record.toResponse(),
	}, nil
}

// ValidateToken verifies a PromptGate API token and returns its active user.
func (s *Service) ValidateToken(ctx context.Context, rawToken string, users UserResolver) (auth.UserProfile, error) {
	user, _, err := s.ValidateTokenWithExpiry(ctx, rawToken, users)
	return user, err
}

// ValidateTokenWithExpiry validates a JWT and returns its user profile and expiration.
func (s *Service) ValidateTokenWithExpiry(ctx context.Context, rawToken string, users UserResolver) (auth.UserProfile, time.Time, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return auth.UserProfile{}, time.Time{}, ErrInvalidToken
	}

	claims := &tokenClaims{}
	parsed, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return auth.UserProfile{}, time.Time{}, ErrInvalidToken
	}
	if strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.ID) == "" {
		return auth.UserProfile{}, time.Time{}, ErrInvalidToken
	}

	var record Token
	if err := s.db.WithContext(ctx).
		Where("token_hash = ?", sha256hex(rawToken)).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return auth.UserProfile{}, time.Time{}, ErrInvalidToken
		}
		return auth.UserProfile{}, time.Time{}, fmt.Errorf("load token: %w", err)
	}
	if record.UserID != claims.Subject {
		return auth.UserProfile{}, time.Time{}, ErrInvalidToken
	}
	if record.RevokedAt != nil {
		return auth.UserProfile{}, time.Time{}, ErrTokenRevoked
	}
	now := time.Now().UTC()
	if record.ExpiredAt != nil || record.ExpiresAt.Before(now) {
		return auth.UserProfile{}, time.Time{}, ErrTokenExpired
	}

	user, err := users.UserByID(ctx, record.UserID)
	if err != nil {
		return auth.UserProfile{}, time.Time{}, err
	}
	if !user.IsActive {
		return auth.UserProfile{}, time.Time{}, ErrAccountInactive
	}
	switch user.Role {
	case auth.RoleUser, auth.RoleManager, auth.RoleAdmin:
		return user, record.ExpiresAt, nil
	default:
		return auth.UserProfile{}, time.Time{}, ErrInsufficientRole
	}
}

// ListTokens returns all tokens owned by the given user.
func (s *Service) ListTokens(ctx context.Context, userID string) ([]TokenResponse, error) {
	return s.ListTokensFiltered(ctx, userID, true)
}

// ListTokensFiltered returns tokens owned by the given user, optionally including revoked tokens.
func (s *Service) ListTokensFiltered(ctx context.Context, userID string, includeRevoked bool) ([]TokenResponse, error) {
	result, err := s.ListTokensPaged(ctx, userID, ListParams{
		Page:           1,
		PageSize:       100,
		SortBy:         "createdAt",
		SortDir:        "desc",
		IncludeRevoked: includeRevoked,
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListTokensPaged returns tokens owned by the given user with pagination and sorting.
func (s *Service) ListTokensPaged(ctx context.Context, userID string, params ListParams) (ListResult, error) {
	normalizeListParams(&params)

	query := s.db.WithContext(ctx).Model(&Token{}).Where("user_id = ?", userID)
	if !params.IncludeRevoked {
		query = query.Where("revoked_at IS NULL")
	}
	if search := strings.TrimSpace(strings.ToLower(params.Search)); search != "" {
		like := "%" + search + "%"
		query = query.Where(tokenSearchCondition, like, like, like)
	}
	switch params.Status {
	case "", "all":
	case "active":
		query = query.Where("revoked_at IS NULL AND expired_at IS NULL AND expires_at > CURRENT_TIMESTAMP")
	case "expired":
		query = query.Where("revoked_at IS NULL AND (expired_at IS NOT NULL OR expires_at <= CURRENT_TIMESTAMP)")
	case "revoked":
		query = query.Where("revoked_at IS NOT NULL")
	default:
		return ListResult{}, ErrInvalidSort
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, err
	}

	var tokens []Token
	var err error
	query, err = applyTokenSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ListResult{}, err
	}
	if err := query.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&tokens).Error; err != nil {
		return ListResult{}, err
	}

	responses := make([]TokenResponse, len(tokens))
	for i, t := range tokens {
		responses[i] = t.toResponse()
	}
	return ListResult{
		Items:    responses,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// RevokeToken marks a token as revoked. Only the owning user can revoke their own token.
func (s *Service) RevokeToken(ctx context.Context, userID, tokenID string) error {
	var record Token
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", tokenID, userID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTokenNotFound
		}
		return err
	}

	if record.RevokedAt != nil {
		return nil
	}

	now := time.Now().UTC()
	record.RevokedAt = &now
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return err
	}
	s.notifier.Notify(ctx, configevents.DomainAuth)
	return nil
}

// AdminListTokens returns all tokens for any user (admin use).
func (s *Service) AdminListTokens(ctx context.Context, userID string) ([]TokenResponse, error) {
	result, err := s.ListTokensPaged(ctx, userID, ListParams{
		Page:           1,
		PageSize:       100,
		SortBy:         "createdAt",
		SortDir:        "desc",
		IncludeRevoked: true,
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// AdminRevokeToken revokes any token by ID regardless of owner.
func (s *Service) AdminRevokeToken(ctx context.Context, tokenID string) error {
	var record Token
	if err := s.db.WithContext(ctx).
		Where("id = ?", tokenID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTokenNotFound
		}
		return err
	}

	if record.RevokedAt != nil {
		return nil
	}

	now := time.Now().UTC()
	record.RevokedAt = &now
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return err
	}
	s.notifier.Notify(ctx, configevents.DomainAuth)
	return nil
}

// normalizeListParams applies default token pagination and sorting values.
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
		params.SortBy = "createdAt"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}
}

// applyTokenSort applies a validated token order to the query.
func applyTokenSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"name":        "name",
		"description": "description",
		"createdAt":   "created_at",
		"expiresAt":   "expires_at",
		"revokedAt":   "revoked_at",
		"expiredAt":   "expired_at",
		"status":      "CASE WHEN revoked_at IS NOT NULL THEN 3 WHEN expired_at IS NOT NULL OR expires_at <= CURRENT_TIMESTAMP THEN 2 ELSE 1 END",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	return query.Order(column + " " + dir).Order("id ASC"), nil
}

// normalizeSortDir converts a token sort direction into SQL syntax.
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
