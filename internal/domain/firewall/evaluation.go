package firewall

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// EnabledRules returns enabled firewall rules ordered by priority.
func (s *Service) EnabledRules(ctx context.Context) ([]FirewallRule, error) {
	var records []FirewallRule
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("type = ? AND referentiel_id IS NULL", RuleTypeGlobal).
		Order("priority ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list enabled firewall rules: %w", err)
	}
	return records, nil
}

// Allows reports whether clientIP is allowed by enabled firewall rules. No match allows.
func (s *Service) Allows(ctx context.Context, clientIP string) (bool, *RuleResponse, error) {
	addr, err := parseClientAddr(clientIP)
	if err != nil {
		return false, nil, err
	}

	var records []FirewallRule
	if err := s.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("type = ? AND referentiel_id IS NULL", RuleTypeGlobal).
		Order("priority ASC").
		Find(&records).Error; err != nil {
		return false, nil, fmt.Errorf("list enabled firewall rules: %w", err)
	}
	for _, record := range records {
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

// ServiceAccountAllows evaluates scoped rules for one service account. No match denies.
func (s *Service) ServiceAccountAllows(ctx context.Context, serviceAccountID string, clientIP string) (bool, *RuleResponse, error) {
	return s.scopedAllows(ctx, RuleTypeServiceAccount, serviceAccountID, clientIP)
}

// UserAllows evaluates scoped rules for one user. No match denies.
func (s *Service) UserAllows(ctx context.Context, userID string, clientIP string) (bool, *RuleResponse, error) {
	return s.scopedAllows(ctx, RuleTypeUser, userID, clientIP)
}

// scopedAllows evaluates scoped rules for one referential id. No match denies.
func (s *Service) scopedAllows(ctx context.Context, ruleType RuleType, referentielID string, clientIP string) (bool, *RuleResponse, error) {
	addr, err := parseClientAddr(clientIP)
	if err != nil {
		return false, nil, err
	}

	var records []FirewallRule
	if err := s.db.WithContext(ctx).
		Where("type = ? AND referentiel_id = ? AND enabled = ?", ruleType, referentielID, true).
		Order("priority ASC").
		Find(&records).Error; err != nil {
		return false, nil, fmt.Errorf("list enabled scoped firewall rules: %w", err)
	}
	for _, record := range records {
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

// parseClientAddr parses a client address and strips any port.
func parseClientAddr(clientIP string) (netip.Addr, error) {
	host := strings.TrimSpace(clientIP)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}, ErrInvalidAddress
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr, nil
	}
	if addr.IsLoopback() {
		return netip.MustParseAddr("127.0.0.1"), nil
	}
	return netip.Addr{}, ErrInvalidAddress
}

// ruleMatches checks whether a firewall rule address matches a client address.
func ruleMatches(raw string, addr netip.Addr) (bool, error) {
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return false, ErrInvalidAddress
		}
		return prefix.Contains(addr), nil
	}
	ruleAddr, err := netip.ParseAddr(raw)
	if err != nil {
		return false, ErrInvalidAddress
	}
	return ruleAddr == addr, nil
}
