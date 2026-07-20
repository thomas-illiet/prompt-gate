package httpapi

import "promptgate/backend/internal/transport/httpapi/admin"

func adminRoutes(handler *admin.Handler) []routeDefinition {
	routes := make([]routeDefinition, 0, 105)
	routes = append(routes, adminUserRoutes(handler)...)
	routes = append(routes, adminPromptRoutes(handler)...)
	routes = append(routes, adminDashboardRoutes(handler)...)
	routes = append(routes, adminServiceAccountRoutes(handler)...)
	routes = append(routes, adminFirewallRoutes(handler)...)
	routes = append(routes, adminSubscriptionRoutes(handler)...)
	routes = append(routes, adminGroupRoutes(handler)...)
	routes = append(routes, adminProviderRoutes(handler)...)
	routes = append(routes, adminPricingRoutes(handler)...)
	routes = append(routes, adminMCPRoutes(handler)...)
	routes = append(routes, adminMonitoringRoutes(handler)...)
	routes = append(routes, adminFAQRoutes(handler)...)
	routes = append(routes, adminSetupGuideRoutes(handler)...)
	return routes
}

func adminUserRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/users", handler: handler.HandleAdminListUsers},
		{pattern: "GET /api/v1/admin/users/{id}", handler: handler.HandleAdminGetUser},
		{pattern: "GET /api/v1/admin/users/{id}/statistics", handler: handler.HandleAdminUserStatistics},
		{pattern: "PATCH /api/v1/admin/users/{id}", handler: handler.HandleAdminUpdateUser},
		{pattern: "PATCH /api/v1/admin/users/{id}/note", handler: handler.HandleAdminUpdateUserNote},
		{pattern: "DELETE /api/v1/admin/users/{id}", handler: handler.HandleAdminDeleteUser},
		{pattern: "GET /api/v1/admin/users/{id}/tokens", handler: handler.HandleAdminListUserTokens},
		{pattern: "DELETE /api/v1/admin/users/{id}/tokens/{tokenId}", handler: handler.HandleAdminRevokeToken},
		{pattern: "GET /api/v1/admin/users/{id}/groups", handler: handler.HandleAdminListUserGroups},
		{pattern: "PUT /api/v1/admin/users/{id}/groups", handler: handler.HandleAdminReplaceUserGroups},
		{pattern: "PUT /api/v1/admin/users/{id}/subscription-plan", handler: handler.HandleAdminAssignUserSubscriptionPlan},
		{pattern: "GET /api/v1/admin/users/{id}/firewall/rules", handler: handler.HandleAdminListUserFirewallRules},
		{pattern: "POST /api/v1/admin/users/{id}/firewall/rules", handler: handler.HandleAdminCreateUserFirewallRule},
		{pattern: "GET /api/v1/admin/users/{id}/firewall/rules/{ruleId}", handler: handler.HandleAdminGetUserFirewallRule},
		{pattern: "PATCH /api/v1/admin/users/{id}/firewall/rules/{ruleId}", handler: handler.HandleAdminUpdateUserFirewallRule},
		{pattern: "PATCH /api/v1/admin/users/{id}/firewall/rules/{ruleId}/priority", handler: handler.HandleAdminMoveUserFirewallRulePriority},
		{pattern: "POST /api/v1/admin/users/{id}/firewall/simulate", handler: handler.HandleAdminSimulateUserFirewallRule},
		{pattern: "DELETE /api/v1/admin/users/{id}/firewall/rules/{ruleId}", handler: handler.HandleAdminDeleteUserFirewallRule},
	}
}

func adminPromptRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/prompts", handler: handler.HandleAdminListPrompts},
	}
}

func adminDashboardRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/dashboard/tokens", handler: handler.HandleAdminDashboardTokens},
		{pattern: "GET /api/v1/admin/dashboard/messages", handler: handler.HandleAdminDashboardMessages},
		{pattern: "GET /api/v1/admin/dashboard/duration", handler: handler.HandleAdminDashboardDuration},
		{pattern: "GET /api/v1/admin/dashboard/activity", handler: handler.HandleAdminDashboardActivity},
		{pattern: "GET /api/v1/admin/dashboard/top-models", handler: handler.HandleAdminDashboardTopModels},
		{pattern: "GET /api/v1/admin/dashboard/top-provider-names", handler: handler.HandleAdminDashboardTopProviderNames},
		{pattern: "GET /api/v1/admin/dashboard/top-provider-types", handler: handler.HandleAdminDashboardTopProviderTypes},
		{pattern: "GET /api/v1/admin/dashboard/adoption", handler: handler.HandleAdminDashboardAdoption},
		{pattern: "GET /api/v1/admin/dashboard/top-identities", handler: handler.HandleAdminDashboardTopIdentities},
	}
}

func adminServiceAccountRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/service-accounts", handler: handler.HandleAdminListServiceAccounts},
		{pattern: "POST /api/v1/admin/service-accounts", handler: handler.HandleAdminCreateServiceAccount},
		{pattern: "GET /api/v1/admin/service-accounts/{id}", handler: handler.HandleAdminGetServiceAccount},
		{pattern: "PATCH /api/v1/admin/service-accounts/{id}", handler: handler.HandleAdminUpdateServiceAccount},
		{pattern: "PATCH /api/v1/admin/service-accounts/{id}/note", handler: handler.HandleAdminUpdateServiceAccountNote},
		{pattern: "DELETE /api/v1/admin/service-accounts/{id}", handler: handler.HandleAdminDeleteServiceAccount},
		{pattern: "GET /api/v1/admin/service-accounts/{id}/tokens", handler: handler.HandleAdminListServiceAccountTokens},
		{pattern: "POST /api/v1/admin/service-accounts/{id}/tokens", handler: handler.HandleAdminCreateServiceAccountToken},
		{pattern: "DELETE /api/v1/admin/service-accounts/{id}/tokens/{tokenId}", handler: handler.HandleAdminRevokeServiceAccountToken},
		{pattern: "PUT /api/v1/admin/service-accounts/{id}/subscription-plan", handler: handler.HandleAdminAssignServiceAccountSubscriptionPlan},
		{pattern: "GET /api/v1/admin/service-accounts/{id}/firewall/rules", handler: handler.HandleAdminListServiceAccountFirewallRules},
		{pattern: "POST /api/v1/admin/service-accounts/{id}/firewall/rules", handler: handler.HandleAdminCreateServiceAccountFirewallRule},
		{pattern: "GET /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}", handler: handler.HandleAdminGetServiceAccountFirewallRule},
		{pattern: "PATCH /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}", handler: handler.HandleAdminUpdateServiceAccountFirewallRule},
		{pattern: "PATCH /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}/priority", handler: handler.HandleAdminMoveServiceAccountFirewallRulePriority},
		{pattern: "POST /api/v1/admin/service-accounts/{id}/firewall/simulate", handler: handler.HandleAdminSimulateServiceAccountFirewallRule},
		{pattern: "DELETE /api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}", handler: handler.HandleAdminDeleteServiceAccountFirewallRule},
	}
}

func adminFirewallRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/firewall/rules", handler: handler.HandleAdminListFirewallRules},
		{pattern: "POST /api/v1/admin/firewall/rules", handler: handler.HandleAdminCreateFirewallRule},
		{pattern: "GET /api/v1/admin/firewall/rules/{id}", handler: handler.HandleAdminGetFirewallRule},
		{pattern: "PATCH /api/v1/admin/firewall/rules/{id}", handler: handler.HandleAdminUpdateFirewallRule},
		{pattern: "PATCH /api/v1/admin/firewall/rules/{id}/priority", handler: handler.HandleAdminMoveFirewallRulePriority},
		{pattern: "POST /api/v1/admin/firewall/simulate", handler: handler.HandleAdminSimulateFirewallRule},
		{pattern: "DELETE /api/v1/admin/firewall/rules/{id}", handler: handler.HandleAdminDeleteFirewallRule},
	}
}

func adminSubscriptionRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/subscriptions", handler: handler.HandleAdminListSubscriptionPlans},
		{pattern: "POST /api/v1/admin/subscriptions", handler: handler.HandleAdminCreateSubscriptionPlan},
		{pattern: "GET /api/v1/admin/subscriptions/{id}", handler: handler.HandleAdminGetSubscriptionPlan},
		{pattern: "PATCH /api/v1/admin/subscriptions/{id}", handler: handler.HandleAdminUpdateSubscriptionPlan},
		{pattern: "PUT /api/v1/admin/subscriptions/{id}/default", handler: handler.HandleAdminSetDefaultSubscriptionPlan},
		{pattern: "DELETE /api/v1/admin/subscriptions/{id}", handler: handler.HandleAdminDeleteSubscriptionPlan},
	}
}

func adminGroupRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/groups", handler: handler.HandleAdminListGroups},
		{pattern: "POST /api/v1/admin/groups", handler: handler.HandleAdminCreateGroup},
		{pattern: "POST /api/v1/admin/groups/model-patterns/validate", handler: handler.HandleAdminValidateGroupModelPatterns},
		{pattern: "GET /api/v1/admin/groups/{id}", handler: handler.HandleAdminGetGroup},
		{pattern: "PATCH /api/v1/admin/groups/{id}", handler: handler.HandleAdminUpdateGroup},
		{pattern: "DELETE /api/v1/admin/groups/{id}", handler: handler.HandleAdminDeleteGroup},
		{pattern: "PUT /api/v1/admin/groups/{id}/members/{userId}", handler: handler.HandleAdminAddGroupMember},
		{pattern: "DELETE /api/v1/admin/groups/{id}/members/{userId}", handler: handler.HandleAdminRemoveGroupMember},
	}
}

func adminProviderRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/providers", handler: handler.HandleAdminListProviders},
		{pattern: "GET /api/v1/admin/providers/model-catalog", handler: handler.HandleAdminProviderModelCatalog},
		{pattern: "POST /api/v1/admin/providers", handler: handler.HandleAdminCreateProvider},
		{pattern: "GET /api/v1/admin/providers/{id}", handler: handler.HandleAdminGetProvider},
		{pattern: "PATCH /api/v1/admin/providers/{id}", handler: handler.HandleAdminUpdateProvider},
		{pattern: "DELETE /api/v1/admin/providers/{id}", handler: handler.HandleAdminDeleteProvider},
	}
}

func adminPricingRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/pricing", handler: handler.HandleAdminGetPricing},
		{pattern: "PUT /api/v1/admin/pricing", handler: handler.HandleAdminUpdatePricing},
		{pattern: "PATCH /api/v1/admin/pricing/fallback", handler: handler.HandleAdminUpdatePricingFallback},
		{pattern: "POST /api/v1/admin/pricing/models", handler: handler.HandleAdminCreateModelPrice},
		{pattern: "GET /api/v1/admin/pricing/models/{id}", handler: handler.HandleAdminGetModelPrice},
		{pattern: "PATCH /api/v1/admin/pricing/models/{id}", handler: handler.HandleAdminUpdateModelPrice},
		{pattern: "DELETE /api/v1/admin/pricing/models/{id}", handler: handler.HandleAdminDeleteModelPrice},
		{pattern: "GET /api/v1/admin/pricing/check", handler: handler.HandleAdminPricingConfigurationCheck},
	}
}

func adminMCPRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/mcp/servers", handler: handler.HandleAdminListMCPServers},
		{pattern: "POST /api/v1/admin/mcp/servers", handler: handler.HandleAdminCreateMCPServer},
		{pattern: "GET /api/v1/admin/mcp/servers/{id}", handler: handler.HandleAdminGetMCPServer},
		{pattern: "PATCH /api/v1/admin/mcp/servers/{id}", handler: handler.HandleAdminUpdateMCPServer},
		{pattern: "DELETE /api/v1/admin/mcp/servers/{id}", handler: handler.HandleAdminDeleteMCPServer},
	}
}

func adminMonitoringRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/monitoring/services", handler: handler.HandleAdminListMonitoringServices},
		{pattern: "POST /api/v1/admin/monitoring/services", handler: handler.HandleAdminCreateMonitoringService},
		{pattern: "GET /api/v1/admin/monitoring/services/{id}", handler: handler.HandleAdminGetMonitoringService},
		{pattern: "PATCH /api/v1/admin/monitoring/services/{id}", handler: handler.HandleAdminUpdateMonitoringService},
		{pattern: "DELETE /api/v1/admin/monitoring/services/{id}", handler: handler.HandleAdminDeleteMonitoringService},
		{pattern: "POST /api/v1/admin/monitoring/services/{id}/check", handler: handler.HandleAdminCheckMonitoringService},
	}
}

func adminFAQRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/faqs", handler: handler.HandleAdminListFAQ},
		{pattern: "POST /api/v1/admin/faqs", handler: handler.HandleAdminCreateFAQ},
		{pattern: "POST /api/v1/admin/faqs/preview", handler: handler.HandleAdminPreviewFAQ},
		{pattern: "GET /api/v1/admin/faqs/{id}", handler: handler.HandleAdminGetFAQ},
		{pattern: "PATCH /api/v1/admin/faqs/{id}", handler: handler.HandleAdminUpdateFAQ},
		{pattern: "PATCH /api/v1/admin/faqs/{id}/position", handler: handler.HandleAdminMoveFAQ},
		{pattern: "DELETE /api/v1/admin/faqs/{id}", handler: handler.HandleAdminDeleteFAQ},
	}
}

func adminSetupGuideRoutes(handler *admin.Handler) []routeDefinition {
	return []routeDefinition{
		{pattern: "GET /api/v1/admin/setup-guides", handler: handler.HandleAdminListSetupGuides},
		{pattern: "POST /api/v1/admin/setup-guides", handler: handler.HandleAdminCreateSetupGuide},
		{pattern: "POST /api/v1/admin/setup-guides/validate", handler: handler.HandleAdminValidateSetupGuide},
		{pattern: "PUT /api/v1/admin/setup-guides/reorder", handler: handler.HandleAdminReorderSetupGuides},
		{pattern: "GET /api/v1/admin/setup-guides/{id}", handler: handler.HandleAdminGetSetupGuide},
		{pattern: "PATCH /api/v1/admin/setup-guides/{id}", handler: handler.HandleAdminUpdateSetupGuide},
		{pattern: "DELETE /api/v1/admin/setup-guides/{id}", handler: handler.HandleAdminDeleteSetupGuide},
	}
}
