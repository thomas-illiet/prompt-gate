package firewall

import (
	"context"
	"fmt"
	"sync/atomic"

	"promptgate/backend/internal/domain/auth"
)

type Snapshot struct {
	Global          []FirewallRule            `json:"global"`
	ServiceAccounts map[string][]FirewallRule `json:"serviceAccounts"`
	Users           map[string][]FirewallRule `json:"users"`
}

type SnapshotStore struct {
	service *Service
	value   atomic.Value
}

// Snapshot loads enabled global rules and scoped service-account override rules.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	global, err := s.EnabledRules(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{
		Global:          global,
		ServiceAccounts: map[string][]FirewallRule{},
		Users:           map[string][]FirewallRule{},
	}

	var rules []FirewallRule
	if err := s.db.WithContext(ctx).
		Where("type IN ? AND enabled = ? AND referentiel_id IS NOT NULL", []RuleType{RuleTypeServiceAccount, RuleTypeUser}, true).
		Order("type ASC").
		Order("referentiel_id ASC").
		Order("priority ASC").
		Find(&rules).Error; err != nil {
		return Snapshot{}, fmt.Errorf("list enabled scoped firewall rules: %w", err)
	}
	for _, rule := range rules {
		if rule.ReferentielID == nil {
			continue
		}
		switch rule.Type {
		case RuleTypeServiceAccount:
			snapshot.ServiceAccounts[*rule.ReferentielID] = append(snapshot.ServiceAccounts[*rule.ReferentielID], rule)
		case RuleTypeUser:
			snapshot.Users[*rule.ReferentielID] = append(snapshot.Users[*rule.ReferentielID], rule)
		}
	}

	return snapshot, nil
}

// NewSnapshotStore creates an in-memory firewall snapshot store.
func NewSnapshotStore(service *Service) *SnapshotStore {
	store := &SnapshotStore{service: service}
	store.value.Store(Snapshot{
		Global:          []FirewallRule{},
		ServiceAccounts: map[string][]FirewallRule{},
		Users:           map[string][]FirewallRule{},
	})
	return store
}

// Refresh reloads enabled firewall rules from the service.
func (s *SnapshotStore) Refresh(ctx context.Context) error {
	snapshot, err := s.service.Snapshot(ctx)
	if err != nil {
		return err
	}
	s.SetSnapshot(snapshot)
	return nil
}

// Set replaces the in-memory firewall rules snapshot.
func (s *SnapshotStore) Set(rules []FirewallRule) {
	s.SetSnapshot(Snapshot{Global: rules})
}

// SetSnapshot replaces the full in-memory firewall snapshot.
func (s *SnapshotStore) SetSnapshot(snapshot Snapshot) {
	s.value.Store(cloneSnapshot(snapshot))
}

// Rules returns a copy of the current firewall snapshot.
func (s *SnapshotStore) Rules() []FirewallRule {
	return s.Snapshot().Global
}

// Snapshot returns a copy of the current firewall snapshot.
func (s *SnapshotStore) Snapshot() Snapshot {
	snapshot, ok := s.value.Load().(Snapshot)
	if !ok {
		return Snapshot{Global: []FirewallRule{}, ServiceAccounts: map[string][]FirewallRule{}, Users: map[string][]FirewallRule{}}
	}
	return cloneSnapshot(snapshot)
}

// Allows evaluates a client IP against the current firewall snapshot.
func (s *SnapshotStore) Allows(clientIP string) (bool, *RuleResponse, error) {
	addr, err := parseClientAddr(clientIP)
	if err != nil {
		return false, nil, err
	}
	for _, record := range s.Rules() {
		matches, err := ruleMatches(record.Address, addr)
		if err != nil {
			return false, nil, err
		}
		if matches {
			response := record.toResponse()
			return record.Action == ActionAllow, &response, nil
		}
	}
	return true, nil, nil
}

// AllowsUser evaluates a client IP against global or scoped rules for a user.
func (s *SnapshotStore) AllowsUser(clientIP string, user auth.UserProfile) (bool, *RuleResponse, error) {
	if user.FirewallOverrideEnabled {
		switch user.Type {
		case auth.UserTypeService:
			return s.allowsScoped(clientIP, s.Snapshot().ServiceAccounts[user.ID])
		case auth.UserTypeUser:
			return s.allowsScoped(clientIP, s.Snapshot().Users[user.ID])
		}
	}
	return s.Allows(clientIP)
}

// allowsScoped evaluates scoped rules. No match denies.
func (s *SnapshotStore) allowsScoped(clientIP string, rules []FirewallRule) (bool, *RuleResponse, error) {
	addr, err := parseClientAddr(clientIP)
	if err != nil {
		return false, nil, err
	}
	for _, record := range rules {
		matches, err := ruleMatches(record.Address, addr)
		if err != nil {
			return false, nil, err
		}
		if matches {
			response := record.toResponse()
			return record.Action == ActionAllow, &response, nil
		}
	}
	return false, nil, nil
}

// cloneSnapshot copies snapshot slices and maps for safe atomic sharing.
func cloneSnapshot(snapshot Snapshot) Snapshot {
	global := make([]FirewallRule, len(snapshot.Global))
	copy(global, snapshot.Global)

	serviceAccounts := make(map[string][]FirewallRule, len(snapshot.ServiceAccounts))
	for id, rules := range snapshot.ServiceAccounts {
		cp := make([]FirewallRule, len(rules))
		copy(cp, rules)
		serviceAccounts[id] = cp
	}
	users := make(map[string][]FirewallRule, len(snapshot.Users))
	for id, rules := range snapshot.Users {
		cp := make([]FirewallRule, len(rules))
		copy(cp, rules)
		users[id] = cp
	}

	return Snapshot{
		Global:          global,
		ServiceAccounts: serviceAccounts,
		Users:           users,
	}
}
