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
	Providers     []string `json:"providers"`
	ModelPatterns []string `json:"modelPatterns"`
}

type UserAccess struct {
	Providers     []string     `json:"providers"`
	ModelPatterns []string     `json:"modelPatterns"`
	Rules         []AccessRule `json:"rules,omitempty"`
}

type Snapshot struct {
	KnownProviders []string              `json:"knownProviders"`
	Users          map[string]UserAccess `json:"users"`
}

type compiledUserAccess struct {
	providers map[string]struct{}
	patterns  []string
	rules     []compiledAccessRule
}

type compiledAccessRule struct {
	providers map[string]struct{}
	patterns  []string
	regexes   []*regexp.Regexp
}

type compiledSnapshot struct {
	knownProviders map[string]struct{}
	users          map[string]compiledUserAccess
}

type SnapshotStore struct {
	service *Service
	value   atomic.Value
}

func NewSnapshotStore(service *Service) *SnapshotStore {
	store := &SnapshotStore{service: service}
	store.value.Store(compiledSnapshot{
		knownProviders: map[string]struct{}{},
		users:          map[string]compiledUserAccess{},
	})
	return store
}

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	var providerRows []provider.Provider
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("name ASC").
		Find(&providerRows).Error; err != nil {
		return Snapshot{}, fmt.Errorf("list known providers for groups: %w", err)
	}
	knownProviderSet := make(map[string]struct{}, len(providerRows))
	knownProviders := make([]string, 0, len(providerRows))
	for _, row := range providerRows {
		knownProviderSet[row.Name] = struct{}{}
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

func (s *SnapshotStore) SetSnapshot(snapshot Snapshot) error {
	compiled, err := compileSnapshot(snapshot)
	if err != nil {
		return err
	}
	s.value.Store(compiled)
	return nil
}

func (s *SnapshotStore) Snapshot() Snapshot {
	compiled, ok := s.value.Load().(compiledSnapshot)
	if !ok {
		return Snapshot{KnownProviders: []string{}, Users: map[string]UserAccess{}}
	}
	knownProviders := make([]string, 0, len(compiled.knownProviders))
	for providerName := range compiled.knownProviders {
		knownProviders = append(knownProviders, providerName)
	}
	sort.Strings(knownProviders)
	users := make(map[string]UserAccess, len(compiled.users))
	for userID, access := range compiled.users {
		providers := make([]string, 0, len(access.providers))
		for providerName := range access.providers {
			providers = append(providers, providerName)
		}
		sort.Strings(providers)
		patterns := append([]string{}, access.patterns...)
		sort.Strings(patterns)
		rules := make([]AccessRule, 0, len(access.rules))
		for _, rule := range access.rules {
			ruleProviders := make([]string, 0, len(rule.providers))
			for providerName := range rule.providers {
				ruleProviders = append(ruleProviders, providerName)
			}
			sort.Strings(ruleProviders)
			rulePatterns := append([]string{}, rule.patterns...)
			sort.Strings(rulePatterns)
			rules = append(rules, AccessRule{
				Providers:     ruleProviders,
				ModelPatterns: rulePatterns,
			})
		}
		users[userID] = UserAccess{
			Providers:     providers,
			ModelPatterns: patterns,
			Rules:         rules,
		}
	}
	return Snapshot{
		KnownProviders: knownProviders,
		Users:          users,
	}
}

func (s *SnapshotStore) KnownProvider(providerName string) bool {
	compiled, ok := s.value.Load().(compiledSnapshot)
	if !ok {
		return false
	}
	_, ok = compiled.knownProviders[providerName]
	return ok
}

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

func compiledUserAccessAllows(access compiledUserAccess, providerName, model string) bool {
	if model == "" {
		return false
	}
	for _, rule := range access.rules {
		if len(rule.regexes) == 0 {
			continue
		}
		if len(rule.providers) > 0 {
			if _, ok := rule.providers[providerName]; !ok {
				continue
			}
		}
		for _, pattern := range rule.regexes {
			if pattern.MatchString(model) {
				return true
			}
		}
	}
	return false
}

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
	for _, pattern := range group.ModelPatterns {
		modelPatterns = append(modelPatterns, pattern.Pattern)
	}
	if len(modelPatterns) == 0 {
		modelPatterns = []string{defaultAllModelsPattern}
	}
	return normalizeAccessRule(AccessRule{
		Providers:     providerNames,
		ModelPatterns: modelPatterns,
	})
}

func appendUserAccessRule(access UserAccess, rule AccessRule) UserAccess {
	rule = normalizeAccessRule(rule)
	if len(rule.Providers) == 0 {
		return normalizeUserAccess(access)
	}
	access.Providers = append(access.Providers, rule.Providers...)
	access.ModelPatterns = append(access.ModelPatterns, rule.ModelPatterns...)
	access.Rules = append(access.Rules, rule)
	return normalizeUserAccess(access)
}

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
	return compiledAccessRule{
		providers: providers,
		patterns:  normalized.ModelPatterns,
		regexes:   regexes,
	}, nil
}

func compileAccessRules(access UserAccess) ([]compiledAccessRule, error) {
	normalized := normalizeUserAccess(access)
	rules := normalized.Rules
	if len(rules) == 0 && (len(normalized.Providers) > 0 || len(normalized.ModelPatterns) > 0) {
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

func compileUserAccess(access UserAccess) (compiledUserAccess, error) {
	normalized := normalizeUserAccess(access)
	providers, patterns := aggregateAccess(normalized)
	rules, err := compileAccessRules(normalized)
	if err != nil {
		return compiledUserAccess{}, err
	}
	return compiledUserAccess{
		providers: providers,
		patterns:  patterns,
		rules:     rules,
	}, nil
}

func aggregateAccess(access UserAccess) (map[string]struct{}, []string) {
	normalized := normalizeUserAccess(access)
	providerValues := append([]string{}, normalized.Providers...)
	patternValues := append([]string{}, normalized.ModelPatterns...)
	for _, rule := range normalized.Rules {
		providerValues = append(providerValues, rule.Providers...)
		patternValues = append(patternValues, rule.ModelPatterns...)
	}

	providers := make(map[string]struct{})
	for _, providerName := range uniqueStrings(providerValues) {
		providers[providerName] = struct{}{}
	}
	patterns := uniqueStrings(patternValues)
	sort.Strings(patterns)
	return providers, patterns
}

func normalizeAccessRule(rule AccessRule) AccessRule {
	providers := uniqueStrings(rule.Providers)
	patterns := uniqueStrings(rule.ModelPatterns)
	sort.Strings(providers)
	sort.Strings(patterns)
	return AccessRule{
		Providers:     providers,
		ModelPatterns: patterns,
	}
}

func normalizeAccessRules(rules []AccessRule) []AccessRule {
	seen := map[string]struct{}{}
	out := make([]AccessRule, 0, len(rules))
	for _, rule := range rules {
		rule = normalizeAccessRule(rule)
		key := strings.Join(rule.Providers, "\x00") + "\x01" + strings.Join(rule.ModelPatterns, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, rule)
	}
	return out
}

func compileSnapshot(snapshot Snapshot) (compiledSnapshot, error) {
	knownProviders := make(map[string]struct{}, len(snapshot.KnownProviders))
	for _, providerName := range snapshot.KnownProviders {
		if providerName != "" {
			knownProviders[providerName] = struct{}{}
		}
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
		users:          users,
	}, nil
}

func normalizeUserAccess(access UserAccess) UserAccess {
	providers := uniqueStrings(access.Providers)
	patterns := uniqueStrings(access.ModelPatterns)
	rules := normalizeAccessRules(access.Rules)
	sort.Strings(providers)
	sort.Strings(patterns)
	return UserAccess{
		Providers:     providers,
		ModelPatterns: patterns,
		Rules:         rules,
	}
}

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
