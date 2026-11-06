package subscriptions

import (
	"context"
	"errors"
	"testing"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/users"
)

func TestCommittedPlanChangeInvalidatesCacheEvenWhenUsageProjectionFails(t *testing.T) {
	for _, kind := range []auth.UserType{auth.UserTypeUser, auth.UserTypeService} {
		t.Run(string(kind), func(t *testing.T) {
			service, store, user := newSubscriptionTestStore(t)
			sqlDB, _ := service.db.DB()
			defer sqlDB.Close()
			ctx := context.Background()
			service.SetNotifier(store.store)
			if err := service.db.Model(&users.User{}).Where("id = ?", user.ID).Update("type", kind).Error; err != nil {
				t.Fatal(err)
			}
			old, err := service.CreatePlan(ctx, PlanInput{Name: "Old", IsDefault: true})
			if err != nil {
				t.Fatal(err)
			}
			next, err := service.CreatePlan(ctx, PlanInput{Name: "Next"})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.RefreshIfStale(ctx); err != nil {
				t.Fatal(err)
			}
			plan, ok, err := store.ResolveEffectivePlan(ctx, user.ID)
			if err != nil || !ok || plan.ID != old.ID {
				t.Fatalf("cache not primed: %#v %v", plan, err)
			}
			if err := service.db.Exec("DROP TABLE proxy_daily_usage_kpis").Error; err != nil {
				t.Fatal(err)
			}
			if kind == auth.UserTypeUser {
				_, err = service.AssignUserPlan(ctx, user.ID, &next.ID)
			} else {
				_, err = service.AssignServiceAccountPlan(ctx, user.ID, &next.ID)
			}
			if err == nil {
				t.Fatal("fixture did not trigger the usage projection failure")
			}
			assigned, exists, err := service.UserPlanID(ctx, user.ID)
			if err != nil || !exists || assigned == nil || *assigned != next.ID {
				t.Fatalf("assignment did not commit: %v %v", assigned, err)
			}
			if err := store.RefreshIfStale(ctx); err != nil {
				t.Fatal(err)
			}
			plan, ok, err = store.ResolveEffectivePlan(ctx, user.ID)
			if err != nil || !ok || plan.ID != next.ID {
				t.Fatalf("committed plan change left stale Redis assignment: %#v %v", plan, err)
			}
		})
	}
}

func TestDefaultPlanReplacementAssignmentAndSafeDeletion(t *testing.T) {
	service, store, user := newSubscriptionTestStore(t)
	sqlDB, _ := service.db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	service.SetNotifier(store.store)
	first, err := service.CreatePlan(ctx, PlanInput{Name: "First", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreatePlan(ctx, PlanInput{Name: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeletePlan(ctx, first.ID); !errors.Is(err, ErrDefaultPlanDelete) {
		t.Fatalf("default plan deleted: %v", err)
	}
	if _, err := service.SetDefaultPlan(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	list, err := service.ListPlans(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("plan listing: %#v %v", list, err)
	}
	defaults := 0
	for _, p := range list {
		if p.IsDefault {
			defaults++
			if p.ID != second.ID {
				t.Fatal("old default retained")
			}
		}
	}
	if defaults != 1 {
		t.Fatal("default plan uniqueness lost")
	}
	if _, err := service.AssignUserPlan(ctx, user.ID, &first.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeletePlan(ctx, first.ID); !errors.Is(err, ErrPlanAssigned) {
		t.Fatalf("assigned plan deleted: %v", err)
	}
	if _, err := service.AssignUserPlan(ctx, user.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := service.DeletePlan(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetPlan(ctx, first.ID); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("deleted plan remains: %v", err)
	}
	if _, err := service.SetDefaultPlan(ctx, "invalid"); !errors.Is(err, ErrPlanNotFound) {
		t.Fatal(err)
	}
	if err := service.DeletePlan(ctx, "invalid"); !errors.Is(err, ErrPlanNotFound) {
		t.Fatal(err)
	}
	plan, ok, err := store.ResolveEffectivePlan(ctx, user.ID)
	if err != nil || !ok || plan.ID != second.ID {
		t.Fatalf("fallback plan not updated: %#v %v", plan, err)
	}
}
