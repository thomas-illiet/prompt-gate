package database

import (
	"errors"
	"testing"
)

func TestIsUniqueViolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "sqlite", err: errors.New("UNIQUE constraint failed: providers.name"), want: true},
		{name: "postgres message", err: errors.New("duplicate key value violates unique constraint"), want: true},
		{name: "postgres state", err: errors.New("SQLSTATE 23505"), want: true},
		{name: "other", err: errors.New("connection refused"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsUniqueViolation(tt.err); got != tt.want {
				t.Fatalf("IsUniqueViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}
