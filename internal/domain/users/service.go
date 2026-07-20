package users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/platform/configevents"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound                    = errors.New("user not found")
	ErrInvalidExpiration               = errors.New("expires_at must be in the future")
	ErrInvalidNote                     = errors.New("note must be 2000 characters or fewer")
	ErrInvalidServiceAccountName       = errors.New("service account name is required")
	ErrInvalidServiceAccountIdentifier = errors.New("service account identifier must be lowercase alphanumeric with dashes or underscores, max 64 chars")
	ErrServiceAccountConflict          = errors.New("service account identifier already exists")
	ErrPreferredUsernameRequired       = errors.New("preferred_username is required")
	ErrInvalidSort                     = errors.New("invalid_sort")
)

const maxAccountNoteLength = 2000

var serviceAccountIdentifierRegexp = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

type User struct {
	ID                      string        `gorm:"type:uuid;primaryKey"`
	ExternalSub             string        `gorm:"column:external_sub;uniqueIndex;not null"`
	Email                   string        `gorm:"not null;index"`
	PreferredUsername       string        `gorm:"column:preferred_username;not null;index"`
	Name                    string        `gorm:"not null"`
	Type                    auth.UserType `gorm:"type:varchar(16);not null;default:'user';index"`
	Role                    auth.AppRole  `gorm:"type:varchar(16);not null;index"`
	SubscriptionPlanID      *string       `gorm:"column:subscription_plan_id;type:uuid;index"`
	Note                    string        `gorm:"type:text;not null;default:''"`
	IsActive                bool          `gorm:"not null;default:true;index"`
	FirewallOverrideEnabled bool          `gorm:"column:firewall_override_enabled;not null;default:false;index"`
	ExpiresAt               *time.Time    `gorm:"column:expires_at;index"`
	LastLoginAt             time.Time     `gorm:"column:last_login_at;not null;index"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type AdminUser struct {
	ID                        string                   `json:"id"`
	Sub                       string                   `json:"sub"`
	PreferredUsername         string                   `json:"preferredUsername"`
	Email                     string                   `json:"email"`
	Name                      string                   `json:"name"`
	Type                      auth.UserType            `json:"type"`
	Role                      auth.AppRole             `json:"role"`
	SubscriptionPlanID        *string                  `json:"subscriptionPlanId"`
	SubscriptionPlan          *AccountSubscriptionPlan `json:"subscriptionPlan,omitempty"`
	EffectiveSubscriptionPlan *AccountSubscriptionPlan `json:"effectiveSubscriptionPlan,omitempty"`
	QuotaState                *AccountQuotaState       `json:"quotaState,omitempty"`
	Note                      string                   `json:"note"`
	IsActive                  bool                     `json:"isActive"`
	FirewallOverrideEnabled   bool                     `json:"firewallOverrideEnabled"`
	InputTokens               int64                    `json:"inputTokens"`
	OutputTokens              int64                    `json:"outputTokens"`
	ExpiresAt                 *time.Time               `json:"expiresAt"`
	LastLoginAt               time.Time                `json:"lastLoginAt"`
	CreatedAt                 time.Time                `json:"createdAt"`
	UpdatedAt                 time.Time                `json:"updatedAt"`
}

type ListParams struct {
	Page     int
	PageSize int
	Search   string
	SortBy   string
	SortDir  string
	Type     auth.UserType
	Role     auth.AppRole
	Status   string
}

type ListResult struct {
	Items    []AdminUser `json:"items"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Total    int64       `json:"total"`
}

type ServiceAccount struct {
	ID                        string                   `json:"id"`
	Identifier                string                   `json:"identifier"`
	Name                      string                   `json:"name"`
	Role                      auth.AppRole             `json:"role"`
	SubscriptionPlanID        *string                  `json:"subscriptionPlanId"`
	SubscriptionPlan          *AccountSubscriptionPlan `json:"subscriptionPlan,omitempty"`
	EffectiveSubscriptionPlan *AccountSubscriptionPlan `json:"effectiveSubscriptionPlan,omitempty"`
	QuotaState                *AccountQuotaState       `json:"quotaState,omitempty"`
	Note                      string                   `json:"note"`
	IsActive                  bool                     `json:"isActive"`
	FirewallOverrideEnabled   bool                     `json:"firewallOverrideEnabled"`
	InputTokens               int64                    `json:"inputTokens"`
	OutputTokens              int64                    `json:"outputTokens"`
	CreatedAt                 time.Time                `json:"createdAt"`
	UpdatedAt                 time.Time                `json:"updatedAt"`
}

type AccountSubscriptionPlan struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Quota5HTokens *int64 `json:"quota5hTokens"`
	Quota7DTokens *int64 `json:"quota7dTokens"`
	IsDefault     bool   `json:"isDefault"`
}

type AccountQuotaState struct {
	HasSubscription bool       `json:"hasSubscription"`
	PlanID          *string    `json:"planId"`
	PlanName        string     `json:"planName"`
	Used5HTokens    int64      `json:"used5hTokens"`
	Quota5HTokens   *int64     `json:"quota5hTokens"`
	Reset5HAt       *time.Time `json:"reset5hAt"`
	Used7DTokens    int64      `json:"used7dTokens"`
	Quota7DTokens   *int64     `json:"quota7dTokens"`
	Reset7DAt       *time.Time `json:"reset7dAt"`
	SyncedAt        *time.Time `json:"syncedAt"`
}

type ServiceAccountListParams struct {
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}

type ServiceAccountListResult struct {
	Items    []ServiceAccount `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int64            `json:"total"`
}

type ServiceAccountInput struct {
	Identifier              string `json:"identifier"`
	Name                    string `json:"name"`
	IsActive                bool   `json:"isActive"`
	FirewallOverrideEnabled *bool  `json:"firewallOverrideEnabled,omitempty"`
}

type UpdateUserInput struct {
	Role                    auth.AppRole `json:"role"`
	IsActive                bool         `json:"isActive"`
	FirewallOverrideEnabled *bool        `json:"firewallOverrideEnabled,omitempty"`
	ExpiresAt               *time.Time   `json:"expiresAt"`
}

type UpdateAccountNoteInput struct {
	Note string `json:"note"`
}

type Service struct {
	db           *gorm.DB
	notifier     configevents.Notifier
	tokenRevoker tokenRevoker
}

type tokenConsumption struct {
	UserID       string `gorm:"column:user_id"`
	InputTokens  int64  `gorm:"column:input_tokens"`
	OutputTokens int64  `gorm:"column:output_tokens"`
}

type tokenRevoker interface {
	RevokeUserTokensTx(ctx context.Context, tx *gorm.DB, userIDs []string, revokedAt time.Time) (int64, error)
}

// NewService creates a new user service backed by the given database.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db, notifier: configevents.NoopNotifier{}}
}

// SetNotifier configures config event publication after user mutations.
func (s *Service) SetNotifier(notifier configevents.Notifier) {
	if notifier == nil {
		notifier = configevents.NoopNotifier{}
	}
	s.notifier = notifier
}

// SetTokenRevoker configures bulk token revocation when user access is removed.
func (s *Service) SetTokenRevoker(revoker tokenRevoker) {
	s.tokenRevoker = revoker
}

// BeforeCreate generates a UUID for the user if one is not already set.
func (u *User) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(u.ID) == "" {
		u.ID = uuid.NewString()
	}

	return nil
}

// AutoMigrate runs database schema migrations for the User model.
func (s *Service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&User{})
}

// SyncUser upserts an OIDC user by preferred username, assigning admin role to the first user.
func (s *Service) SyncUser(ctx context.Context, identity auth.Identity) (auth.UserProfile, error) {
	if strings.TrimSpace(identity.PreferredUsername) == "" {
		return auth.UserProfile{}, ErrPreferredUsernameRequired
	}

	var record User
	now := time.Now().UTC()

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where(
			"type = ? AND preferred_username = ?",
			auth.UserTypeUser,
			identity.PreferredUsername,
		).Take(&record).Error
		if err == nil {
			record.ExternalSub = identity.Sub
			record.Email = identity.Email
			record.PreferredUsername = identity.PreferredUsername
			record.Name = identity.Name
			record.LastLoginAt = now

			return tx.Save(&record).Error
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		role := auth.RoleNone
		var total int64
		if err := tx.Model(&User{}).Count(&total).Error; err != nil {
			return err
		}
		if total == 0 {
			role = auth.RoleAdmin
		}

		record = User{
			ExternalSub:       identity.Sub,
			Email:             identity.Email,
			PreferredUsername: identity.PreferredUsername,
			Name:              identity.Name,
			Type:              auth.UserTypeUser,
			Role:              role,
			IsActive:          true,
			LastLoginAt:       now,
		}

		return tx.Create(&record).Error
	})
	if err != nil {
		return auth.UserProfile{}, fmt.Errorf("sync user: %w", err)
	}

	return record.profile(), nil
}

// UserByID returns a UserProfile for the given user ID, or ErrUserNotFound if absent.
func (s *Service) UserByID(ctx context.Context, id string) (auth.UserProfile, error) {
	var record User
	if err := s.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return auth.UserProfile{}, ErrUserNotFound
		}

		return auth.UserProfile{}, fmt.Errorf("load user by id: %w", err)
	}

	return record.profile(), nil
}

// GetUser returns the admin view of a user by ID, or ErrUserNotFound if absent.
func (s *Service) GetUser(ctx context.Context, id string) (AdminUser, error) {
	var record User
	if err := s.db.WithContext(ctx).
		Where("id = ? AND type = ?", id, auth.UserTypeUser).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUser{}, ErrUserNotFound
		}

		return AdminUser{}, fmt.Errorf("get user: %w", err)
	}

	items := []AdminUser{record.adminUser()}
	if err := s.attachAdminUserTokenConsumption(ctx, items); err != nil {
		return AdminUser{}, err
	}

	return items[0], nil
}

// ListUsers returns a paginated, optionally filtered list of users.
func (s *Service) ListUsers(ctx context.Context, params ListParams) (ListResult, error) {
	normalizeUserListParams(&params)
	query := s.db.WithContext(ctx).Model(&User{})
	if userSortNeedsConsumption(params.SortBy) {
		query = query.Joins(userTokenConsumptionJoin())
	}

	if search := strings.TrimSpace(strings.ToLower(params.Search)); search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(preferred_username) LIKE ?",
			like,
			like,
			like,
		)
	}

	if params.Type != "" {
		query = query.Where("type = ?", params.Type)
	}

	if params.Role != "" {
		query = query.Where("role = ?", params.Role)
	}

	switch params.Status {
	case "active":
		query = query.Where("is_active = ?", true)
	case "inactive":
		query = query.Where("is_active = ?", false)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListResult{}, fmt.Errorf("count users: %w", err)
	}

	var records []User
	offset := (params.Page - 1) * params.PageSize
	query, err := applyUserSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ListResult{}, err
	}
	if err := query.
		Offset(offset).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return ListResult{}, fmt.Errorf("list users: %w", err)
	}

	items := make([]AdminUser, 0, len(records))
	for _, record := range records {
		items = append(items, record.adminUser())
	}
	if err := s.attachAdminUserTokenConsumption(ctx, items); err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// UpdateUser changes a user's role and active status, returning the updated admin view.
func (s *Service) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (AdminUser, error) {
	if !input.Role.IsValid() {
		return AdminUser{}, fmt.Errorf("invalid role %q", input.Role)
	}
	now := time.Now().UTC()
	if input.Role == auth.RoleNone {
		input.ExpiresAt = nil
	} else if input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
		return AdminUser{}, ErrInvalidExpiration
	}

	var record User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("id = ? AND type = ?", id, auth.UserTypeUser).
			First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}

			return fmt.Errorf("find user for update: %w", err)
		}

		revokesTokens := record.Role != auth.RoleNone && input.Role == auth.RoleNone
		record.Role = input.Role
		record.IsActive = input.IsActive
		if input.FirewallOverrideEnabled != nil {
			record.FirewallOverrideEnabled = *input.FirewallOverrideEnabled
		}
		record.ExpiresAt = input.ExpiresAt

		if err := tx.Save(&record).Error; err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		if revokesTokens && s.tokenRevoker != nil {
			if _, err := s.tokenRevoker.RevokeUserTokensTx(ctx, tx, []string{record.ID}, now); err != nil {
				return fmt.Errorf("revoke user tokens: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return AdminUser{}, ErrUserNotFound
		}
		return AdminUser{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainAuth)
	items := []AdminUser{record.adminUser()}
	if err := s.attachAdminUserTokenConsumption(ctx, items); err != nil {
		return AdminUser{}, err
	}

	return items[0], nil
}

// UpdateUserNote changes the admin note for a human user only.
func (s *Service) UpdateUserNote(ctx context.Context, id string, input UpdateAccountNoteInput) (AdminUser, error) {
	note, err := normalizeAccountNote(input.Note)
	if err != nil {
		return AdminUser{}, err
	}

	var record User
	if err := s.db.WithContext(ctx).
		Where("id = ? AND type = ?", id, auth.UserTypeUser).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminUser{}, ErrUserNotFound
		}

		return AdminUser{}, fmt.Errorf("find user for note update: %w", err)
	}

	record.Note = note
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return AdminUser{}, fmt.Errorf("update user note: %w", err)
	}

	items := []AdminUser{record.adminUser()}
	if err := s.attachAdminUserTokenConsumption(ctx, items); err != nil {
		return AdminUser{}, err
	}

	return items[0], nil
}

// UpdateServiceAccountNote changes the admin note for a service account only.
func (s *Service) UpdateServiceAccountNote(ctx context.Context, id string, input UpdateAccountNoteInput) (ServiceAccount, error) {
	note, err := normalizeAccountNote(input.Note)
	if err != nil {
		return ServiceAccount{}, err
	}

	record, err := s.findServiceAccount(ctx, s.db, id)
	if err != nil {
		return ServiceAccount{}, err
	}

	record.Note = note
	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return ServiceAccount{}, fmt.Errorf("update service account note: %w", err)
	}

	items := []ServiceAccount{record.serviceAccount()}
	if err := s.attachServiceAccountTokenConsumption(ctx, items); err != nil {
		return ServiceAccount{}, err
	}

	return items[0], nil
}

// DeleteUser permanently removes a user by ID, returning ErrUserNotFound if absent.
func (s *Service) DeleteUser(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := deleteAccountFirewallRulesTx(tx, "user", id); err != nil {
			return err
		}

		result := tx.Where("type = ?", auth.UserTypeUser).Delete(&User{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("delete user: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrUserNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.notifier.Notify(ctx, configevents.DomainAuth)
	s.notifier.Notify(ctx, configevents.DomainFirewall)
	return nil
}

// deleteAccountFirewallRulesTx removes account-scoped firewall rules when the
// optional firewall table is present.
func deleteAccountFirewallRulesTx(tx *gorm.DB, ruleType, id string) error {
	err := tx.Exec(
		"DELETE FROM firewall_rules WHERE type = ? AND referentiel_id = ?",
		ruleType,
		id,
	).Error
	if err == nil || isMissingFirewallRulesTable(err) {
		return nil
	}
	return fmt.Errorf("delete %s firewall rules: %w", ruleType, err)
}

// isMissingFirewallRulesTable reports whether a database lacks the optional firewall table.
func isMissingFirewallRulesTable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table: firewall_rules") ||
		strings.Contains(message, `relation "firewall_rules" does not exist`) ||
		strings.Contains(message, "sqlstate 42p01")
}

// normalizeServiceAccountInput validates and normalizes service account form input.
func normalizeServiceAccountInput(input ServiceAccountInput) (string, string, error) {
	identifier := strings.ToLower(strings.TrimSpace(input.Identifier))
	if !serviceAccountIdentifierRegexp.MatchString(identifier) {
		return "", "", ErrInvalidServiceAccountIdentifier
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return "", "", ErrInvalidServiceAccountName
	}

	return identifier, name, nil
}

// serviceAccountFirewallOverride returns the requested override flag or the preserved value.
func serviceAccountFirewallOverride(input ServiceAccountInput, fallback bool) bool {
	if input.FirewallOverrideEnabled == nil {
		return fallback
	}
	return *input.FirewallOverrideEnabled
}

// normalizeAccountNote validates account note length while preserving entered text.
func normalizeAccountNote(note string) (string, error) {
	if len([]rune(note)) > maxAccountNoteLength {
		return "", ErrInvalidNote
	}

	return note, nil
}

// ensureServiceAccountIdentifierAvailable checks service account identifier uniqueness.
func (s *Service) ensureServiceAccountIdentifierAvailable(ctx context.Context, tx *gorm.DB, identifier, exceptID string) error {
	var count int64
	query := tx.WithContext(ctx).
		Model(&User{}).
		Where("type = ? AND LOWER(preferred_username) = ?", auth.UserTypeService, strings.ToLower(identifier))
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}

	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("check service account identifier: %w", err)
	}
	if count > 0 {
		return ErrServiceAccountConflict
	}

	return nil
}

// findServiceAccount loads a service account user by ID.
func (s *Service) findServiceAccount(ctx context.Context, tx *gorm.DB, id string) (User, error) {
	var record User
	if err := tx.WithContext(ctx).
		Where("id = ? AND type = ?", id, auth.UserTypeService).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrUserNotFound
		}

		return User{}, fmt.Errorf("find service account: %w", err)
	}

	return record, nil
}

// normalizeUserListParams applies default user pagination and sorting values.
func normalizeUserListParams(params *ListParams) {
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
		params.SortBy = "lastLoginAt"
	}
	if params.SortDir == "" {
		params.SortDir = "desc"
	}
}

// normalizeServiceAccountListParams applies default service account list values.
func normalizeServiceAccountListParams(params *ServiceAccountListParams) {
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

// userSortNeedsConsumption reports whether sorting requires the token consumption join.
func userSortNeedsConsumption(sortBy string) bool {
	return sortBy == "inputTokens" || sortBy == "outputTokens"
}

// userTokenConsumptionJoin returns the SQL join for user token usage aggregates.
func userTokenConsumptionJoin() string {
	return `LEFT JOIN (
		SELECT interceptions.initiator_id AS user_id,
			COALESCE(SUM(token_usages.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(token_usages.output_tokens), 0) AS output_tokens
		FROM token_usages
		JOIN interceptions ON interceptions.id = token_usages.interception_id
		GROUP BY interceptions.initiator_id
	) AS token_consumption ON token_consumption.user_id = users.id`
}

// applyUserSort applies a validated user order to the query.
func applyUserSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"name":              "users.name",
		"email":             "users.email",
		"role":              "users.role",
		"isActive":          "users.is_active",
		"lastLoginAt":       "users.last_login_at",
		"createdAt":         "users.created_at",
		"updatedAt":         "users.updated_at",
		"inputTokens":       "COALESCE(token_consumption.input_tokens, 0)",
		"outputTokens":      "COALESCE(token_consumption.output_tokens, 0)",
		"preferredUsername": "users.preferred_username",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}

	query = query.Order(column + " " + dir)
	if sortBy == "lastLoginAt" {
		query = query.Order("users.created_at DESC")
	}
	return query.Order("users.id ASC"), nil
}

// applyServiceAccountSort applies a validated service account order to the query.
func applyServiceAccountSort(query *gorm.DB, sortBy, sortDir string) (*gorm.DB, error) {
	dir, err := normalizeSortDir(sortDir)
	if err != nil {
		return nil, err
	}

	columns := map[string]string{
		"name":         "users.name",
		"identifier":   "users.preferred_username",
		"isActive":     "users.is_active",
		"createdAt":    "users.created_at",
		"updatedAt":    "users.updated_at",
		"inputTokens":  "COALESCE(token_consumption.input_tokens, 0)",
		"outputTokens": "COALESCE(token_consumption.output_tokens, 0)",
	}

	column, ok := columns[sortBy]
	if !ok {
		return nil, ErrInvalidSort
	}
	return query.Order(column + " " + dir).Order("users.id ASC"), nil
}

// normalizeSortDir converts a user sort direction into SQL syntax.
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
