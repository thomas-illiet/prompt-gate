package database

import (
	"testing"
	"time"
)

func TestResolvePoolOptions(t *testing.T) {
	tests := []struct {
		name       string
		configured PoolOptions
		want       PoolOptions
	}{
		{
			name: "defaults",
			want: PoolOptions{
				ConnMaxLifetime: 30 * time.Minute,
				MaxIdleConns:    5,
				MaxOpenConns:    10,
			},
		},
		{
			name: "explicit values",
			configured: PoolOptions{
				ConnMaxLifetime: 45 * time.Minute,
				MaxIdleConns:    7,
				MaxOpenConns:    21,
			},
			want: PoolOptions{
				ConnMaxLifetime: 45 * time.Minute,
				MaxIdleConns:    7,
				MaxOpenConns:    21,
			},
		},
		{
			name: "zero fields retain defaults",
			configured: PoolOptions{
				MaxIdleConns: 8,
			},
			want: PoolOptions{
				ConnMaxLifetime: 30 * time.Minute,
				MaxIdleConns:    8,
				MaxOpenConns:    10,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolvePoolOptions(test.configured); got != test.want {
				t.Fatalf("resolvePoolOptions(%#v) = %#v, want %#v", test.configured, got, test.want)
			}
		})
	}
}
