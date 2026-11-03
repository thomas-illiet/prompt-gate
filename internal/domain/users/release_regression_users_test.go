package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
)

func TestRegressionExpiredAccountMustLoseAccessImmediately(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	profile, err := s.SyncUser(ctx, auth.Identity{Sub: "audit-sub", PreferredUsername: "audit-user", Email: "audit@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&User{}).Where("id = ?", profile.ID).Updates(map[string]any{"expires_at": time.Now().Add(-time.Minute), "role": auth.RoleUser}).Error; err != nil {
		t.Fatal(err)
	}
	current, err := s.UserByID(ctx, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Role != auth.RoleNone || !current.AccessExpired(time.Now()) {
		t.Fatalf("expired user still receives active role %q", current.Role)
	}
}

func TestOIDCSubjectCannotTakeOverExistingUsername(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	original, err := s.SyncUser(ctx, auth.Identity{Sub: "owner", PreferredUsername: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncUser(ctx, auth.Identity{Sub: "attacker", PreferredUsername: "admin"}); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("identity takeover: %v", err)
	}
	current, err := s.UserByID(ctx, original.ID)
	if err != nil || current.Sub != "owner" || current.Role != original.Role {
		t.Fatalf("original identity modified: %#v %v", current, err)
	}
	renamed, err := s.SyncUser(ctx, auth.Identity{Sub: "owner", PreferredUsername: "renamed"})
	if err != nil || renamed.ID != original.ID || renamed.Role != original.Role {
		t.Fatalf("subject rename lost identity: %#v %v", renamed, err)
	}
	other, err := s.SyncUser(ctx, auth.Identity{Sub: "other", PreferredUsername: "occupied"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncUser(ctx, auth.Identity{Sub: "owner", PreferredUsername: other.PreferredUsername}); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("rename collision: %v", err)
	}
}
