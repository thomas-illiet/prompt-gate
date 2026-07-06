package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// SyncUser upserts a user from the given OIDC identity, assigning admin role to the first user.
func (s *Service) SyncUser(ctx context.Context, identity auth.Identity) (auth.UserProfile, error) {
	var record User
	now := time.Now().UTC()

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("external_sub = ?", identity.Sub).Take(&record).Error
		if err == nil {
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

// ListServiceAccounts returns all service accounts.
func (s *Service) ListServiceAccounts(ctx context.Context) ([]ServiceAccount, error) {
	result, err := s.ListServiceAccountsPaged(ctx, ServiceAccountListParams{
		Page:     1,
		PageSize: 100,
		SortBy:   "createdAt",
		SortDir:  "desc",
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListServiceAccountsPaged returns service accounts with pagination and sorting.
func (s *Service) ListServiceAccountsPaged(ctx context.Context, params ServiceAccountListParams) (ServiceAccountListResult, error) {
	normalizeServiceAccountListParams(&params)

	query := s.db.WithContext(ctx).
		Model(&User{}).
		Where("type = ?", auth.UserTypeService)
	if userSortNeedsConsumption(params.SortBy) {
		query = query.Joins(userTokenConsumptionJoin())
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ServiceAccountListResult{}, fmt.Errorf("count service accounts: %w", err)
	}

	var records []User
	var err error
	query, err = applyServiceAccountSort(query, params.SortBy, params.SortDir)
	if err != nil {
		return ServiceAccountListResult{}, err
	}
	if err := query.
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&records).Error; err != nil {
		return ServiceAccountListResult{}, fmt.Errorf("list service accounts: %w", err)
	}

	items := make([]ServiceAccount, 0, len(records))
	for _, record := range records {
		items = append(items, record.serviceAccount())
	}
	if err := s.attachServiceAccountTokenConsumption(ctx, items); err != nil {
		return ServiceAccountListResult{}, err
	}

	return ServiceAccountListResult{
		Items:    items,
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}, nil
}

// GetServiceAccount returns one service account by ID.
func (s *Service) GetServiceAccount(ctx context.Context, id string) (ServiceAccount, error) {
	record, err := s.findServiceAccount(ctx, s.db, id)
	if err != nil {
		return ServiceAccount{}, err
	}

	items := []ServiceAccount{record.serviceAccount()}
	if err := s.attachServiceAccountTokenConsumption(ctx, items); err != nil {
		return ServiceAccount{}, err
	}

	return items[0], nil
}

// ServiceAccountProfile returns an auth profile for a service account.
func (s *Service) ServiceAccountProfile(ctx context.Context, id string) (auth.UserProfile, error) {
	record, err := s.findServiceAccount(ctx, s.db, id)
	if err != nil {
		return auth.UserProfile{}, err
	}

	return record.profile(), nil
}

// CreateServiceAccount creates an active service account with role user.
func (s *Service) CreateServiceAccount(ctx context.Context, input ServiceAccountInput) (ServiceAccount, error) {
	normalized, name, err := normalizeServiceAccountInput(input)
	if err != nil {
		return ServiceAccount{}, err
	}

	var record User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensureServiceAccountIdentifierAvailable(ctx, tx, normalized, ""); err != nil {
			return err
		}

		now := time.Now().UTC()
		record = User{
			ExternalSub:             "service:" + uuid.NewString(),
			Email:                   "",
			PreferredUsername:       normalized,
			Name:                    name,
			Type:                    auth.UserTypeService,
			Role:                    auth.RoleUser,
			IsActive:                input.IsActive,
			FirewallOverrideEnabled: serviceAccountFirewallOverride(input, false),
			LastLoginAt:             now,
		}

		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("create service account: %w", err)
		}

		return nil
	})
	if err != nil {
		return ServiceAccount{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainAuth)
	return record.serviceAccount(), nil
}

// UpdateServiceAccount updates service-account metadata and active status.
func (s *Service) UpdateServiceAccount(ctx context.Context, id string, input ServiceAccountInput) (ServiceAccount, error) {
	normalized, name, err := normalizeServiceAccountInput(input)
	if err != nil {
		return ServiceAccount{}, err
	}

	var record User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		loaded, err := s.findServiceAccount(ctx, tx, id)
		if err != nil {
			return err
		}
		record = loaded

		if err := s.ensureServiceAccountIdentifierAvailable(ctx, tx, normalized, id); err != nil {
			return err
		}

		record.PreferredUsername = normalized
		record.Name = name
		record.Type = auth.UserTypeService
		record.Role = auth.RoleUser
		record.IsActive = input.IsActive
		record.FirewallOverrideEnabled = serviceAccountFirewallOverride(input, record.FirewallOverrideEnabled)

		if err := tx.Save(&record).Error; err != nil {
			return fmt.Errorf("update service account: %w", err)
		}

		return nil
	})
	if err != nil {
		return ServiceAccount{}, err
	}

	s.notifier.Notify(ctx, configevents.DomainAuth)
	items := []ServiceAccount{record.serviceAccount()}
	if err := s.attachServiceAccountTokenConsumption(ctx, items); err != nil {
		return ServiceAccount{}, err
	}

	return items[0], nil
}

// DeleteServiceAccount permanently removes a service account by ID.
func (s *Service) DeleteServiceAccount(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"DELETE FROM firewall_rules WHERE type = ? AND referentiel_id = ?",
			"service_account",
			id,
		).Error; err != nil {
			return fmt.Errorf("delete service account firewall rules: %w", err)
		}

		result := tx.Where("type = ?", auth.UserTypeService).Delete(&User{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("delete service account: %w", result.Error)
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

// ExpireAccess removes roles whose access expiration date has passed and revokes their tokens.
func (s *Service) ExpireAccess(ctx context.Context, now time.Time) (int64, error) {
	now = now.UTC()
	var expiredIDs []string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&User{}).
			Where("expires_at IS NOT NULL AND expires_at <= ? AND role <> ?", now, auth.RoleNone).
			Pluck("id", &expiredIDs).Error; err != nil {
			return fmt.Errorf("list expired user access: %w", err)
		}
		if len(expiredIDs) == 0 {
			return nil
		}

		if err := tx.Model(&User{}).
			Where("id IN ?", expiredIDs).
			Updates(map[string]any{
				"role":       auth.RoleNone,
				"expires_at": nil,
			}).Error; err != nil {
			return fmt.Errorf("expire user access: %w", err)
		}

		if s.tokenRevoker != nil {
			if _, err := s.tokenRevoker.RevokeUserTokensTx(ctx, tx, expiredIDs, now); err != nil {
				return fmt.Errorf("revoke expired user tokens: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(expiredIDs) == 0 {
		return 0, nil
	}

	s.notifier.Notify(ctx, configevents.DomainAuth)
	return int64(len(expiredIDs)), nil
}

// StartAccessExpiration starts a background goroutine that periodically expires user access.
func (s *Service) StartAccessExpiration(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}

	go func() {
		s.expireAccessLog(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.expireAccessLog(ctx)
			}
		}
	}()
}

// expireAccessLog expires user access and writes an operational log entry.
func (s *Service) expireAccessLog(ctx context.Context) {
	count, err := s.ExpireAccess(ctx, time.Now().UTC())
	if err != nil {
		slog.Error("failed to expire user access", "error", err)
		return
	}
	if count > 0 {
		slog.Info("expired user access", "users", count)
	}
}

// DeleteUser permanently removes a user by ID, returning ErrUserNotFound if absent.
func (s *Service) DeleteUser(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := deleteUserFirewallRulesTx(tx, id); err != nil {
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

// deleteUserFirewallRulesTx removes user-scoped firewall rules when the firewall table is present.
func deleteUserFirewallRulesTx(tx *gorm.DB, id string) error {
	err := tx.Exec(
		"DELETE FROM firewall_rules WHERE type = ? AND referentiel_id = ?",
		"user",
		id,
	).Error
	if err == nil || isMissingFirewallRulesTable(err) {
		return nil
	}
	return fmt.Errorf("delete user firewall rules: %w", err)
}

// isMissingFirewallRulesTable reports whether a database lacks the optional firewall table.
func isMissingFirewallRulesTable(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table: firewall_rules") ||
		strings.Contains(message, `relation "firewall_rules" does not exist`)
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

// attachAdminUserTokenConsumption fills token totals on admin user rows.
func (s *Service) attachAdminUserTokenConsumption(ctx context.Context, items []AdminUser) error {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	consumptionByUserID, err := s.loadTokenConsumption(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		consumption := consumptionByUserID[items[i].ID]
		items[i].InputTokens = consumption.InputTokens
		items[i].OutputTokens = consumption.OutputTokens
	}
	return nil
}

// attachServiceAccountTokenConsumption fills token totals on service account rows.
func (s *Service) attachServiceAccountTokenConsumption(ctx context.Context, items []ServiceAccount) error {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	consumptionByUserID, err := s.loadTokenConsumption(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		consumption := consumptionByUserID[items[i].ID]
		items[i].InputTokens = consumption.InputTokens
		items[i].OutputTokens = consumption.OutputTokens
	}
	return nil
}

// loadTokenConsumption aggregates token input and output totals by user ID.
func (s *Service) loadTokenConsumption(ctx context.Context, userIDs []string) (map[string]tokenConsumption, error) {
	if len(userIDs) == 0 {
		return map[string]tokenConsumption{}, nil
	}

	var rows []tokenConsumption
	if err := s.db.WithContext(ctx).
		Table("token_usages").
		Select(`interceptions.initiator_id AS user_id,
			COALESCE(SUM(token_usages.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(token_usages.output_tokens), 0) AS output_tokens`).
		Joins("JOIN interceptions ON interceptions.id = token_usages.interception_id").
		Where("interceptions.initiator_id IN ?", userIDs).
		Group("interceptions.initiator_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load token consumption: %w", err)
	}

	consumptionByUserID := make(map[string]tokenConsumption, len(rows))
	for _, row := range rows {
		consumptionByUserID[row.UserID] = row
	}
	return consumptionByUserID, nil
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

// profile maps the database User record to an auth.UserProfile.
func (u *User) profile() auth.UserProfile {
	return auth.UserProfile{
		ID:                      u.ID,
		Sub:                     u.ExternalSub,
		PreferredUsername:       u.PreferredUsername,
		Email:                   u.Email,
		Name:                    u.Name,
		Type:                    u.Type,
		Role:                    u.Role,
		IsActive:                u.IsActive,
		FirewallOverrideEnabled: u.FirewallOverrideEnabled,
		LastLoginAt:             u.LastLoginAt,
	}
}

// adminUser maps the database User record to an AdminUser response with audit timestamps.
func (u *User) adminUser() AdminUser {
	return AdminUser{
		ID:                      u.ID,
		Sub:                     u.ExternalSub,
		PreferredUsername:       u.PreferredUsername,
		Email:                   u.Email,
		Name:                    u.Name,
		Type:                    u.Type,
		Role:                    u.Role,
		SubscriptionPlanID:      u.SubscriptionPlanID,
		Note:                    u.Note,
		IsActive:                u.IsActive,
		FirewallOverrideEnabled: u.FirewallOverrideEnabled,
		ExpiresAt:               u.ExpiresAt,
		LastLoginAt:             u.LastLoginAt,
		CreatedAt:               u.CreatedAt,
		UpdatedAt:               u.UpdatedAt,
	}
}

// serviceAccount maps the database User record to the service-account API response.
func (u *User) serviceAccount() ServiceAccount {
	return ServiceAccount{
		ID:                      u.ID,
		Identifier:              u.PreferredUsername,
		Name:                    u.Name,
		Role:                    auth.RoleUser,
		SubscriptionPlanID:      u.SubscriptionPlanID,
		Note:                    u.Note,
		IsActive:                u.IsActive,
		FirewallOverrideEnabled: u.FirewallOverrideEnabled,
		CreatedAt:               u.CreatedAt,
		UpdatedAt:               u.UpdatedAt,
	}
}
