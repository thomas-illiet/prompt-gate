package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"promptgate/backend/internal/domain/subscriptions"
	"promptgate/backend/internal/domain/users"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newSubscriptionsTestHandler(t *testing.T) (*Handler, *subscriptions.Service, *users.Service) {
	t.Helper()
	dsn := fmt.Sprintf("file:admin-subscriptions-%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	userService := users.NewService(db)
	if err := userService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	subscriptionService := subscriptions.NewService(db)
	if err := subscriptionService.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate subscriptions: %v", err)
	}
	createAdminUsersUsageTables(t, db)
	return NewHandler(Dependencies{Subscriptions: subscriptionService}), subscriptionService, userService
}

func subscriptionRequest(method, target, body, id string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if id != "" {
		request.SetPathValue("id", id)
	}
	return request
}

func createSubscriptionPlanThroughHandler(t *testing.T, handler *Handler, body string) subscriptions.PlanResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.HandleAdminCreateSubscriptionPlan(recorder, subscriptionRequest(http.MethodPost, "/api/v1/admin/subscriptions", body, ""))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create subscription plan: %d %s", recorder.Code, recorder.Body.String())
	}
	var plan subscriptions.PlanResponse
	if err := json.NewDecoder(recorder.Body).Decode(&plan); err != nil {
		t.Fatalf("decode created subscription plan: %v", err)
	}
	return plan
}

func TestAdminSubscriptionPlanCRUDAndAssignments(t *testing.T) {
	handler, _, userService := newSubscriptionsTestHandler(t)
	ctx := context.Background()
	user, err := userService.CreateUser(ctx, "coverage-user")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := userService.CreateServiceAccount(ctx, users.ServiceAccountInput{
		Identifier: "coverage-worker",
		Name:       "Coverage Worker",
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("create service account: %v", err)
	}

	defaultPlan := createSubscriptionPlanThroughHandler(t, handler, `{
		"name":" Starter ","description":" Default tier ",
		"quota5hTokens":100,"quota7dTokens":500,"isDefault":true
	}`)
	assignablePlan := createSubscriptionPlanThroughHandler(t, handler, `{
		"name":"Pro","description":"Assignable tier",
		"quota5hTokens":200,"quota7dTokens":1000,"isDefault":false
	}`)
	if defaultPlan.Name != "Starter" || !defaultPlan.IsDefault || assignablePlan.IsDefault {
		t.Fatalf("unexpected created plans: default=%#v assignable=%#v", defaultPlan, assignablePlan)
	}

	listRecorder := httptest.NewRecorder()
	handler.HandleAdminListSubscriptionPlans(
		listRecorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions?page=1&pageSize=1&sortBy=name&sortDir=desc", nil),
	)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list subscription plans: %d %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listed subscriptions.PlanListResult
	if err := json.NewDecoder(listRecorder.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed plans: %v", err)
	}
	if listed.Total != 2 || len(listed.Items) != 1 || listed.Items[0].Name != "Starter" {
		t.Fatalf("unexpected paginated plans: %#v", listed)
	}

	getRecorder := httptest.NewRecorder()
	handler.HandleAdminGetSubscriptionPlan(
		getRecorder,
		subscriptionRequest(http.MethodGet, "/api/v1/admin/subscriptions/"+assignablePlan.ID, "", assignablePlan.ID),
	)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get subscription plan: %d %s", getRecorder.Code, getRecorder.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	handler.HandleAdminUpdateSubscriptionPlan(
		updateRecorder,
		subscriptionRequest(http.MethodPut, "/api/v1/admin/subscriptions/"+assignablePlan.ID, `{
			"name":"Professional","description":"Updated tier",
			"quota5hTokens":250,"quota7dTokens":1250,"isDefault":false
		}`, assignablePlan.ID),
	)
	if updateRecorder.Code != http.StatusOK || !strings.Contains(updateRecorder.Body.String(), `"name":"Professional"`) {
		t.Fatalf("update subscription plan: %d %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	for name, tc := range map[string]struct {
		userID string
		planID string
		call   func(http.ResponseWriter, *http.Request)
	}{
		"user":            {userID: user.ID, planID: assignablePlan.ID, call: handler.HandleAdminAssignUserSubscriptionPlan},
		"service account": {userID: account.ID, planID: assignablePlan.ID, call: handler.HandleAdminAssignServiceAccountSubscriptionPlan},
	} {
		t.Run("assign "+name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, subscriptionRequest(http.MethodPut, "/assign/"+tc.userID, `{"planId":"`+tc.planID+`"}`, tc.userID))
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), tc.planID) {
				t.Fatalf("assign %s: %d %s", name, recorder.Code, recorder.Body.String())
			}
		})
	}

	assignedDelete := httptest.NewRecorder()
	handler.HandleAdminDeleteSubscriptionPlan(
		assignedDelete,
		subscriptionRequest(http.MethodDelete, "/api/v1/admin/subscriptions/"+assignablePlan.ID, "", assignablePlan.ID),
	)
	if assignedDelete.Code != http.StatusConflict || !strings.Contains(assignedDelete.Body.String(), "subscription_plan_assigned") {
		t.Fatalf("delete assigned plan: %d %s", assignedDelete.Code, assignedDelete.Body.String())
	}

	for name, tc := range map[string]struct {
		userID string
		call   func(http.ResponseWriter, *http.Request)
	}{
		"user":            {userID: user.ID, call: handler.HandleAdminAssignUserSubscriptionPlan},
		"service account": {userID: account.ID, call: handler.HandleAdminAssignServiceAccountSubscriptionPlan},
	} {
		t.Run("clear "+name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, subscriptionRequest(http.MethodPut, "/assign/"+tc.userID, `{"planId":null}`, tc.userID))
			if recorder.Code != http.StatusOK {
				t.Fatalf("clear %s assignment: %d %s", name, recorder.Code, recorder.Body.String())
			}
		})
	}

	deleteRecorder := httptest.NewRecorder()
	handler.HandleAdminDeleteSubscriptionPlan(
		deleteRecorder,
		subscriptionRequest(http.MethodDelete, "/api/v1/admin/subscriptions/"+assignablePlan.ID, "", assignablePlan.ID),
	)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("delete subscription plan: %d %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	defaultDelete := httptest.NewRecorder()
	handler.HandleAdminDeleteSubscriptionPlan(
		defaultDelete,
		subscriptionRequest(http.MethodDelete, "/api/v1/admin/subscriptions/"+defaultPlan.ID, "", defaultPlan.ID),
	)
	if defaultDelete.Code != http.StatusBadRequest || !strings.Contains(defaultDelete.Body.String(), "default_plan_delete_denied") {
		t.Fatalf("delete default plan: %d %s", defaultDelete.Code, defaultDelete.Body.String())
	}

	setDefaultRecorder := httptest.NewRecorder()
	handler.HandleAdminSetDefaultSubscriptionPlan(
		setDefaultRecorder,
		subscriptionRequest(http.MethodPut, "/api/v1/admin/subscriptions/"+defaultPlan.ID+"/default", "", defaultPlan.ID),
	)
	if setDefaultRecorder.Code != http.StatusOK {
		t.Fatalf("set default subscription plan: %d %s", setDefaultRecorder.Code, setDefaultRecorder.Body.String())
	}
}

func TestAdminSubscriptionPlanValidationAndErrorMapping(t *testing.T) {
	handler, _, userService := newSubscriptionsTestHandler(t)
	user, err := userService.CreateUser(context.Background(), "assignment-target")
	if err != nil {
		t.Fatalf("create assignment target: %v", err)
	}
	plan := createSubscriptionPlanThroughHandler(t, handler, `{"name":"Valid"}`)
	missingID := "00000000-0000-0000-0000-000000000099"

	tests := []struct {
		name       string
		call       func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantError  string
	}{
		{
			name:       "invalid create payload",
			call:       handler.HandleAdminCreateSubscriptionPlan,
			request:    subscriptionRequest(http.MethodPost, "/api/v1/admin/subscriptions", `{"name":"","quota5hTokens":-1}`, ""),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_subscription_plan",
		},
		{
			name:       "malformed create payload",
			call:       handler.HandleAdminCreateSubscriptionPlan,
			request:    subscriptionRequest(http.MethodPost, "/api/v1/admin/subscriptions", `{"name":`, ""),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request_body",
		},
		{
			name:       "invalid list sort",
			call:       handler.HandleAdminListSubscriptionPlans,
			request:    httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions?sortBy=unknown", nil),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_sort",
		},
		{
			name:       "missing get",
			call:       handler.HandleAdminGetSubscriptionPlan,
			request:    subscriptionRequest(http.MethodGet, "/api/v1/admin/subscriptions/"+missingID, "", missingID),
			wantStatus: http.StatusNotFound,
			wantError:  "subscription_plan_not_found",
		},
		{
			name:       "missing update",
			call:       handler.HandleAdminUpdateSubscriptionPlan,
			request:    subscriptionRequest(http.MethodPut, "/api/v1/admin/subscriptions/"+missingID, `{"name":"Missing"}`, missingID),
			wantStatus: http.StatusNotFound,
			wantError:  "subscription_plan_not_found",
		},
		{
			name:       "missing set default",
			call:       handler.HandleAdminSetDefaultSubscriptionPlan,
			request:    subscriptionRequest(http.MethodPut, "/api/v1/admin/subscriptions/"+missingID+"/default", "", missingID),
			wantStatus: http.StatusNotFound,
			wantError:  "subscription_plan_not_found",
		},
		{
			name:       "missing delete",
			call:       handler.HandleAdminDeleteSubscriptionPlan,
			request:    subscriptionRequest(http.MethodDelete, "/api/v1/admin/subscriptions/"+missingID, "", missingID),
			wantStatus: http.StatusNotFound,
			wantError:  "subscription_plan_not_found",
		},
		{
			name:       "invalid plan assignment",
			call:       handler.HandleAdminAssignUserSubscriptionPlan,
			request:    subscriptionRequest(http.MethodPut, "/assign/"+user.ID, `{"planId":"`+missingID+`"}`, user.ID),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_subscription_assignment",
		},
		{
			name:       "missing assignment target",
			call:       handler.HandleAdminAssignUserSubscriptionPlan,
			request:    subscriptionRequest(http.MethodPut, "/assign/"+missingID, `{"planId":"`+plan.ID+`"}`, missingID),
			wantStatus: http.StatusNotFound,
			wantError:  "user_not_found",
		},
		{
			name:       "malformed assignment payload",
			call:       handler.HandleAdminAssignServiceAccountSubscriptionPlan,
			request:    subscriptionRequest(http.MethodPut, "/assign/"+missingID, `{"planId":`, missingID),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request_body",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, tc.request)
			if recorder.Code != tc.wantStatus || !strings.Contains(recorder.Body.String(), tc.wantError) {
				t.Fatalf("status/body = %d %s, want %d containing %q", recorder.Code, recorder.Body.String(), tc.wantStatus, tc.wantError)
			}
		})
	}

	for _, tc := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "list", call: NewHandler(Dependencies{}).HandleAdminListSubscriptionPlans},
		{name: "create", call: NewHandler(Dependencies{}).HandleAdminCreateSubscriptionPlan},
		{name: "get", call: NewHandler(Dependencies{}).HandleAdminGetSubscriptionPlan},
		{name: "update", call: NewHandler(Dependencies{}).HandleAdminUpdateSubscriptionPlan},
		{name: "set default", call: NewHandler(Dependencies{}).HandleAdminSetDefaultSubscriptionPlan},
		{name: "delete", call: NewHandler(Dependencies{}).HandleAdminDeleteSubscriptionPlan},
		{name: "assign user", call: NewHandler(Dependencies{}).HandleAdminAssignUserSubscriptionPlan},
		{name: "assign service account", call: NewHandler(Dependencies{}).HandleAdminAssignServiceAccountSubscriptionPlan},
	} {
		t.Run("service unavailable "+tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.call(recorder, subscriptionRequest(http.MethodPost, "/unavailable", `{}`, missingID))
			if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "subscriptions service unavailable") {
				t.Fatalf("service unavailable %s: %d %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}

	genericRecorder := httptest.NewRecorder()
	if !writeSubscriptionPlanError(genericRecorder, fmt.Errorf("database unavailable")) || genericRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("generic plan error: %d %s", genericRecorder.Code, genericRecorder.Body.String())
	}
	assignmentRecorder := httptest.NewRecorder()
	if !writeSubscriptionAssignmentError(assignmentRecorder, fmt.Errorf("database unavailable")) || assignmentRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("generic assignment error: %d %s", assignmentRecorder.Code, assignmentRecorder.Body.String())
	}
}
