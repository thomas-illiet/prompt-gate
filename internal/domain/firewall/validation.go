package firewall

import (
	"net/netip"
	"strings"
)

// validateAddress validates an IP address or CIDR rule target.
func validateAddress(raw string) (string, error) {
	address := strings.TrimSpace(raw)
	if address == "" {
		return "", ErrInvalidAddress
	}

	if strings.Contains(address, "/") {
		prefix, err := netip.ParsePrefix(address)
		if err != nil || !prefix.Addr().Is4() {
			return "", ErrInvalidAddress
		}
		return address, nil
	}

	addr, err := netip.ParseAddr(address)
	if err != nil || !addr.Is4() {
		return "", ErrInvalidAddress
	}
	return address, nil
}

// validatePriority validates a positive firewall priority.
func validatePriority(priority int) error {
	if priority < MinPriority || priority > MaxPriority {
		return ErrPriorityOutOfRange
	}
	return nil
}

// validateAction checks whether a firewall action is accepted.
func validateAction(action Action) error {
	switch action {
	case ActionAllow, ActionDeny:
		return nil
	default:
		return ErrInvalidAction
	}
}

// isUniqueConstraintError detects database uniqueness violations.
func isUniqueConstraintError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505")
}
