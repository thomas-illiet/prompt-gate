package auth

import "testing"

func TestAppRoleValidation(t *testing.T) {
	for _, role := range []AppRole{RoleNone, RoleLimited, RoleUser, RoleAdmin} {
		if !role.IsValid() {
			t.Errorf("expected %q to be valid", role)
		}
	}

	if AppRole("manager").IsValid() {
		t.Fatal("expected legacy manager role to be invalid")
	}
}
