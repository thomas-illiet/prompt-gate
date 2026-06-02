package groups

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"promptgate/backend/internal/domain/provider"
)

var ErrLegacySnapshot = errors.New("legacy group access snapshot")
var ErrSnapshotServiceUnavailable = errors.New("group snapshot service unavailable")

type AccessRule struct {
	Providers             []string `json:"providers"`
	ModelPatterns         []string `json:"modelPatterns"`
	ExcludedModelPatterns []string `json:"excludedModelPatterns,omitempty"`
}

type UserAccess struct {
	Providers             []string     `json:"providers"`
	ModelPatterns         []string     `json:"modelPatterns"`
	ExcludedModelPatterns []string     `json:"excludedModelPatterns,omitempty"`
	Rules                 []AccessRule `json:"rules,omitempty"`
}

type Snapshot struct {
	KnownProviders []string                         `json:"knownProviders"`
	ProviderTypes  map[string]provider.ProviderType `json:"providerTypes"`
	Users          map[string]UserAccess            `json:"users"`
}

type compiledUserAccess struct {
	providers        map[string]struct{}
	patterns         []string
	excludedPatterns []string
	rules            []compiledAccessRule
}

type compiledAccessRule struct {
	providers        map[string]struct{}
	patterns         []string
	regexes          []*regexp.Regexp
	excludedPatterns []string
	excludedRegexes  []*regexp.Regexp
}

type compiledSnapshot struct {
	knownProviders map[string]struct{}
	providerTypes  map[string]provider.ProviderType
	users          map[string]compiledUserAccess
}

type SnapshotStore struct {
	service *Service
	value   atomic.Value
}

// NewSnapshotStore creates an initialized in-memory group access snapshot store.
func NewSnapshotStore(service *Service) *SnapshotStore {
	store := &SnapshotStore{service: service}
	store.value.Store(compiledSnapshot{
		knownProviders: map[string]struct{}{},
		providerTypes:  map[string]provider.ProviderType{},
		users:          map[string]compiledUserAccess{},
	})
	return store
}

// Snapshot builds the current group access snapshot from enabled providers and group memberships.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	var providerRows []provider.Provider
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("name ASC").
		Find(&providerRows).Error; err != nil {
		return Snapshot{}, fmt.Errorf("list known providers for groups: %w", err)
	}
	knownProviderSet := make(map[string]struct{}, len(providerRows))
	providerTypes := make(map[string]provider.ProviderType, len(providerRows))
	knownProviders := make([]string, 0, len(providerRows))
	for _, row := range providerRows {
		knownProviderSet[row.Name] = struct{}{}
		providerTypes[row.Name] = row.Type
		knownProviders = append(knownProviders, row.Name)
	}

	var records []Group
	if err := s.db.WithContext(ctx).
		Preload("Providers").
		Preload("ModelPatterns").
		Preload("Members").
		Order("name ASC").
		Find(&records).Error; err != nil {
		return Snapshot{}, fmt.Errorf("list groups for snapshot: %w", err)
	}

	users := map[string]UserAccess{}
	for _, group := range records {
		rule := groupAccessRule(group, knownProviderSet)
		for _, member := range group.Members {
			users[member.ID] = appendUserAccessRule(users[member.ID], rule)
		}
	}

	return Snapshot{
		KnownProviders: knownProviders,
		ProviderTypes:  providerTypes,
		Users:          users,
	}, nil
}

// UserAccess returns the access-group grants for one user.
func (s *Service) UserAccess(ctx context.Context, userID string) (UserAccess, error) {
	userID = strings.TrimSpace(userID)
	if err := s.ensureUserExists(ctx, s.db, userID); err != nil {
		return UserAccess{}, err
	}

	var records []Group
	if err := s.db.WithContext(ctx).
		Joins("JOIN access_group_members ON access_group_members.group_id = access_groups.id").
		Where("access_group_members.user_id = ?", userID).
		Preload("Providers").
		Preload("ModelPatterns").
		Order("access_groups.name ASC").
		Find(&records).Error; err != nil {
		return UserAccess{}, fmt.Errorf("load user access groups: %w", err)
	}

	access := UserAccess{}
	for _, group := range records {
		access = appendUserAccessRule(access, groupAccessRule(group, nil))
	}
	return normalizeUserAccess(access), nil
}

// Refresh reloads the snapshot from the backing service and stores the compiled result.
func (s *SnapshotStore) Refresh(ctx context.Context) error {
	if s.service == nil {
		return ErrSnapshotServiceUnavailable
	}
	snapshot, err := s.service.Snapshot(ctx)
	if err != nil {
		return err
	}
	return s.SetSnapshot(snapshot)
}

// SetSnapshot compiles and stores a new group access snapshot.
func (s *SnapshotStore) SetSnapshot(snapshot Snapshot) error {
	compiled, err := compileSnapshot(snapshot)
	if err != nil {
		return err
	}
	s.value.Store(compiled)
	return nil
}

// Snapshot returns a normalized copy of the currently stored group access snapshot.
func (s *SnapshotStore) Snapshot() Snapshot {
	compiled, ok := s.value.Load().(compiledSnapshot)
	if !ok {
		return Snapshot{
			KnownProviders: []string{},
			ProviderTypes:  map[string]provider.ProviderType{},
			Users:          map[string]UserAccess{},
		}
	}
	knownProviders := make([]string, 0, len(compiled.knownProviders))
	for providerName := range compiled.knownProviders {
		knownProviders = append(knownProviders, providerName)
	}
	sort.Strings(knownProviders)
	providerTypes := make(map[string]provider.ProviderType, len(compiled.providerTypes))
	for providerName, providerType := range compiled.providerTypes {
		providerTypes[providerName] = providerType
	}
	users := make(map[string]UserAccess, len(compiled.users))
	for userID, access := range compiled.users {
		providers := make([]string, 0, len(access.providers))
		for providerName := range access.providers {
			providers = append(providers, providerName)
		}
		sort.Strings(providers)
		patterns := append([]string{}, access.patterns...)
		sort.Strings(patterns)
		excludedPatterns := append([]string{}, access.excludedPatterns...)
		sort.Strings(excludedPatterns)
		rules := make([]AccessRule, 0, len(access.rules))
		for _, rule := range access.rules {
			ruleProviders := make([]string, 0, len(rule.providers))
			for providerName := range rule.providers {
				ruleProviders = append(ruleProviders, providerName)
			}
			sort.Strings(ruleProviders)
			rulePatterns := append([]string{}, rule.patterns...)
			sort.Strings(rulePatterns)
			ruleExcludedPatterns := append([]string{}, rule.excludedPatterns...)
			sort.Strings(ruleExcludedPatterns)
			rules = append(rules, AccessRule{
				Providers:             ruleProviders,
				ModelPatterns:         rulePatterns,
				ExcludedModelPatterns: ruleExcludedPatterns,
			})
		}
		users[userID] = UserAccess{
			Providers:             providers,
			ModelPatterns:         patterns,
			ExcludedModelPatterns: excludedPatterns,
			Rules:                 rules,
		}
	}
	return Snapshot{
		KnownProviders: knownProviders,
		ProviderTypes:  providerTypes,
		Users:          users,
	}
}

// KnownProvider reports whether a provider exists in the current snapshot.
func (s *SnapshotStore) KnownProvider(providerName string) bool {
	compiled, ok := s.value.Load().(compiledSnapshot)
	if !ok {
		return false
	}
	_, ok = compiled.knownProviders[providerName]
	return ok
}

// ProviderType returns the configured type for a known provider.
func (s *SnapshotStore) ProviderType(providerName string) (provider.ProviderType, bool) {
	compiled, ok := s.value.Load().(compiledSnapshot)
	if !ok {
		return "", false
	}
	providerType, ok := compiled.providerTypes[providerName]
	return providerType, ok
}

// AllowsProvider reports whether the current snapshot permits a user to access a provider.
func (s *SnapshotStore) AllowsProvider(userID, providerName string) bool {
	compiled, ok := s.value.Load().(compiledSnapshot)
	if !ok {
		return false
	}
	access, ok := compiled.users[userID]
	if !ok {
		return false
	}
	return compiledUserAccessAllowsProvider(access, providerName)
}

// Allows reports whether the current snapshot permits a user to access a provider/model pair.
func (s *SnapshotStore) Allows(userID, providerName, model string) bool {
	compiled, ok := s.value.Load().(compiledSnapshot)
	if !ok {
		return false
	}
	access, ok := compiled.users[userID]
	if !ok {
		return false
	}
	return compiledUserAccessAllows(access, providerName, model)
}

// Allows reports whether this user access permits a provider/model pair.
func (access UserAccess) Allows(providerName, model string) bool {
	compiled, err := compileUserAccess(access)
	if err != nil {
		return false
	}
	return compiledUserAccessAllows(compiled, providerName, model)
}

// compiledUserAccessAllowsProvider evaluates whether access rules include a provider grant.
func compiledUserAccessAllowsProvider(access compiledUserAccess, providerName string) bool {
	if providerName == "" {
		return false
	}
	_, ok := access.providers[providerName]
	return ok
}

// compiledUserAccessAllows evaluates compiled access rules for a provider/model pair.
func compiledUserAccessAllows(access compiledUserAccess, providerName, model string) bool {
	if model == "" {
		return false
	}
	allowed := false
	for _, rule := range access.rules {
		if len(rule.providers) > 0 {
			if _, ok := rule.providers[providerName]; !ok {
				continue
			}
		}
		for _, pattern := range rule.excludedRegexes {
			if pattern.MatchString(model) {
				return false
			}
		}
		if len(rule.regexes) == 0 {
			continue
		}
		for _, pattern := range rule.regexes {
			if pattern.MatchString(model) {
				allowed = true
				break
			}
		}
	}
	return allowed
}

// groupAccessRule converts a group record into a normalized access rule.
func groupAccessRule(group Group, knownProviderSet map[string]struct{}) AccessRule {
	providerNames := []string{}
	for _, item := range group.Providers {
		if !item.Enabled {
			continue
		}
		if knownProviderSet != nil {
			if _, ok := knownProviderSet[item.Name]; !ok {
				continue
			}
		}
		providerNames = append(providerNames, item.Name)
	}
	modelPatterns := make([]string, 0, len(group.ModelPatterns))
	excludedModelPatterns := []string{}
	for _, pattern := range group.ModelPatterns {
		switch pattern.PatternType {
		case "", GroupModelPatternTypeAllow:
			modelPatterns = append(modelPatterns, pattern.Pattern)
		case GroupModelPatternTypeExclude:
			excludedModelPatterns = append(excludedModelPatterns, pattern.Pattern)
		}
	}
	if len(modelPatterns) == 0 {
		modelPatterns = []string{defaultAllModelsPattern}
	}
	return normalizeAccessRule(AccessRule{
		Providers:             providerNames,
		ModelPatterns:         modelPatterns,
		ExcludedModelPatterns: excludedModelPatterns,
	})
}

// appendUserAccessRule appends a group rule and refreshes aggregate user access fields.
func appendUserAccessRule(access UserAccess, rule AccessRule) UserAccess {
	rule = normalizeAccessRule(rule)
	if len(rule.Providers) == 0 {
		return normalizeUserAccess(access)
	}
	access.Providers = append(access.Providers, rule.Providers...)
	access.ModelPatterns = append(access.ModelPatterns, rule.ModelPatterns...)
	access.ExcludedModelPatterns = append(access.ExcludedModelPatterns, rule.ExcludedModelPatterns...)
	access.Rules = append(access.Rules, rule)
	return normalizeUserAccess(access)
}

// compileAccessRule compiles one normalized access rule into provider and regex lookup structures.
func compileAccessRule(rule AccessRule) (compiledAccessRule, error) {
	normalized := normalizeAccessRule(rule)
	providers := make(map[string]struct{}, len(normalized.Providers))
	for _, providerName := range normalized.Providers {
		providers[providerName] = struct{}{}
	}
	regexes := make([]*regexp.Regexp, 0, len(normalized.ModelPatterns))
	for _, pattern := range normalized.ModelPatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return compiledAccessRule{}, fmt.Errorf("compile group model pattern %q: %w", pattern, err)
		}
		regexes = append(regexes, compiled)
	}
	excludedRegexes := make([]*regexp.Regexp, 0, len(normalized.ExcludedModelPatterns))
	for _, pattern := range normalized.ExcludedModelPatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return compiledAccessRule{}, fmt.Errorf("compile group excluded model pattern %q: %w", pattern, err)
		}
		excludedRegexes = append(excludedRegexes, compiled)
	}
	return compiledAccessRule{
		providers:        providers,
		patterns:         normalized.ModelPatterns,
		regexes:          regexes,
		excludedPatterns: normalized.ExcludedModelPatterns,
		excludedRegexes:  excludedRegexes,
	}, nil
}

// compileAccessRules compiles user access rules and rejects legacy aggregate-only snapshots.
func compileAccessRules(access UserAccess) ([]compiledAccessRule, error) {
	normalized := normalizeUserAccess(access)
	rules := normalized.Rules
	if len(rules) == 0 && (len(normalized.Providers) > 0 || len(normalized.ModelPatterns) > 0 || len(normalized.ExcludedModelPatterns) > 0) {
		return nil, ErrLegacySnapshot
	}

	compiled := make([]compiledAccessRule, 0, len(rules))
	for _, rule := range rules {
		next, err := compileAccessRule(rule)
		if err != nil {
			return nil, err
		}
		if len(next.regexes) == 0 {
			compiled = append(compiled, next)
			continue
		}
		compiled = append(compiled, next)
	}
	return compiled, nil
}

// compileUserAccess compiles normalized user access into aggregate and per-rule lookup data.
func compileUserAccess(access UserAccess) (compiledUserAccess, error) {
	normalized := normalizeUserAccess(access)
	providers, patterns, excludedPatterns := aggregateAccess(normalized)
	rules, err := compileAccessRules(normalized)
	if err != nil {
		return compiledUserAccess{}, err
	}
	return compiledUserAccess{
		providers:        providers,
		patterns:         patterns,
		excludedPatterns: excludedPatterns,
		rules:            rules,
	}, nil
}

// aggregateAccess derives aggregate provider and model pattern grants from user access rules.
func aggregateAccess(access UserAccess) (map[string]struct{}, []string, []string) {
	normalized := normalizeUserAccess(access)
	providerValues := append([]string{}, normalized.Providers...)
	patternValues := append([]string{}, normalized.ModelPatterns...)
	excludedPatternValues := append([]string{}, normalized.ExcludedModelPatterns...)
	for _, rule := range normalized.Rules {
		providerValues = append(providerValues, rule.Providers...)
		patternValues = append(patternValues, rule.ModelPatterns...)
		excludedPatternValues = append(excludedPatternValues, rule.ExcludedModelPatterns...)
	}

	providers := make(map[string]struct{})
	for _, providerName := range uniqueStrings(providerValues) {
		providers[providerName] = struct{}{}
	}
	patterns := uniqueStrings(patternValues)
	sort.Strings(patterns)
	excludedPatterns := uniqueStrings(excludedPatternValues)
	sort.Strings(excludedPatterns)
	return providers, patterns, excludedPatterns
}

// normalizeAccessRule sorts and de-duplicates one access rule.
func normalizeAccessRule(rule AccessRule) AccessRule {
	providers := uniqueStrings(rule.Providers)
	patterns := uniqueStrings(rule.ModelPatterns)
	excludedPatterns := uniqueStrings(rule.ExcludedModelPatterns)
	sort.Strings(providers)
	sort.Strings(patterns)
	sort.Strings(excludedPatterns)
	return AccessRule{
		Providers:             providers,
		ModelPatterns:         patterns,
		ExcludedModelPatterns: excludedPatterns,
	}
}

// normalizeAccessRules sorts, normalizes, and de-duplicates access rules.
func normalizeAccessRules(rules []AccessRule) []AccessRule {
	seen := map[string]struct{}{}
	out := make([]AccessRule, 0, len(rules))
	for _, rule := range rules {
		rule = normalizeAccessRule(rule)
		key := strings.Join(rule.Providers, "\x00") + "\x01" + strings.Join(rule.ModelPatterns, "\x00") + "\x01" + strings.Join(rule.ExcludedModelPatterns, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, rule)
	}
	return out
}

// compileSnapshot compiles a persisted snapshot into immutable lookup maps.
func compileSnapshot(snapshot Snapshot) (compiledSnapshot, error) {
	knownProviders := make(map[string]struct{}, len(snapshot.KnownProviders))
	providerTypes := make(map[string]provider.ProviderType, len(snapshot.KnownProviders))
	for _, providerName := range snapshot.KnownProviders {
		providerName = strings.TrimSpace(providerName)
		if providerName == "" {
			continue
		}
		providerType := snapshot.ProviderTypes[providerName]
		if providerType == "" {
			return compiledSnapshot{}, ErrLegacySnapshot
		}
		knownProviders[providerName] = struct{}{}
		providerTypes[providerName] = providerType
	}
	users := make(map[string]compiledUserAccess, len(snapshot.Users))
	for userID, access := range snapshot.Users {
		compiledAccess, err := compileUserAccess(access)
		if err != nil {
			return compiledSnapshot{}, err
		}
		users[userID] = compiledAccess
	}
	return compiledSnapshot{
		knownProviders: knownProviders,
		providerTypes:  providerTypes,
		users:          users,
	}, nil
}

// normalizeUserAccess sorts and de-duplicates aggregate and per-rule user access values.
func normalizeUserAccess(access UserAccess) UserAccess {
	providers := uniqueStrings(access.Providers)
	patterns := uniqueStrings(access.ModelPatterns)
	excludedPatterns := uniqueStrings(access.ExcludedModelPatterns)
	rules := normalizeAccessRules(access.Rules)
	sort.Strings(providers)
	sort.Strings(patterns)
	sort.Strings(excludedPatterns)
	return UserAccess{
		Providers:             providers,
		ModelPatterns:         patterns,
		ExcludedModelPatterns: excludedPatterns,
		Rules:                 rules,
	}
}

// uniqueStrings returns non-empty unique strings in first-seen order.
func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
