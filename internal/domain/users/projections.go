package users

import "promptgate/backend/internal/domain/auth"

func (u *User) profile() auth.UserProfile {
	return auth.UserProfile{
		ID:                u.ID,
		Sub:               u.ExternalSub,
		PreferredUsername: u.PreferredUsername,
		Email:             u.Email,
		Name:              u.Name,
		Type:              u.Type,
		Role:              u.Role,
		IsActive:          u.IsActive,
		LastLoginAt:       u.LastLoginAt,
	}
}

func (u *User) adminUser() AdminUser {
	return AdminUser{
		ID:                 u.ID,
		Sub:                u.ExternalSub,
		PreferredUsername:  u.PreferredUsername,
		Email:              u.Email,
		Name:               u.Name,
		Type:               u.Type,
		Role:               u.Role,
		SubscriptionPlanID: u.SubscriptionPlanID,
		Note:               u.Note,
		IsActive:           u.IsActive,
		ExpiresAt:          u.ExpiresAt,
		LastLoginAt:        u.LastLoginAt,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

func (u *User) serviceAccount() ServiceAccount {
	return ServiceAccount{
		ID:                 u.ID,
		Identifier:         u.PreferredUsername,
		Name:               u.Name,
		Role:               auth.RoleUser,
		SubscriptionPlanID: u.SubscriptionPlanID,
		Note:               u.Note,
		IsActive:           u.IsActive,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}
