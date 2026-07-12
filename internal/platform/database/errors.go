package database

import "strings"

// IsUniqueViolation reports whether err represents a database uniqueness
// violation. It supports the PostgreSQL SQLSTATE as well as the messages
// emitted by PostgreSQL and SQLite drivers.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "23505")
}
