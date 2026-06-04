import type {
  SubscriptionPlan,
  SubscriptionPlanListResponse,
  SubscriptionPlanPayload,
} from '~/types/subscriptions'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  default_plan_delete_denied: 'Default plan cannot be deleted.',
  invalid_sort: 'Selected subscription plan sort is invalid.',
  invalid_subscription_plan: 'Plan name and quotas must be valid.',
  subscription_plan_assigned:
    'This plan is assigned to one or more accounts.',
  subscription_plan_not_found: 'Subscription plan no longer exists.',
}

export function toAdminSubscriptionErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected subscription management error.',
  )
}

export function useAdminSubscriptions() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const saving = shallowRef(false)
  const selectedPlan = shallowRef<SubscriptionPlan | null>(null)

	const queryList = useQueryList<SubscriptionPlan>({
		fetch: (queryString) =>
			apiFetch<SubscriptionPlanListResponse>(
				`/api/v1/admin/subscriptions?${queryString}`,
			),
    initialSortBy: 'name',
    initialSortDir: 'asc',
    toErrorMessage: toAdminSubscriptionErrorMessage,
  })

  const defaultPlan = computed(
    () => queryList.items.value.find((plan) => plan.isDefault) ?? null,
  )

  async function reload() {
    await queryList.reload()
  }

	async function loadAllPlans() {
		const response = await apiFetch<SubscriptionPlanListResponse>(
			'/api/v1/admin/subscriptions?page=1&pageSize=100&sortBy=name&sortDir=asc',
		)
    queryList.items.value = response.items
    queryList.total.value = response.total
    return response.items
  }

	async function loadPlan(planId: string) {
		selectedPlan.value = await apiFetch<SubscriptionPlan>(
			`/api/v1/admin/subscriptions/${planId}`,
		)
    return selectedPlan.value
  }

  async function createPlan(payload: SubscriptionPlanPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Subscription plan created.',
        toErrorMessage: toAdminSubscriptionErrorMessage,
      },
			async () => {
				const plan = await apiJson<SubscriptionPlan>(
					'/api/v1/admin/subscriptions',
					payload,
					{ method: 'POST' },
        )
        await reload()
        return plan
      },
    )
  }

  async function updatePlan(planId: string, payload: SubscriptionPlanPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Subscription plan updated.',
        toErrorMessage: toAdminSubscriptionErrorMessage,
      },
			async () => {
				const plan = await apiJson<SubscriptionPlan>(
					`/api/v1/admin/subscriptions/${planId}`,
					payload,
					{ method: 'PATCH' },
        )
        selectedPlan.value = plan
        await reload()
        return plan
      },
    )
  }

  async function deletePlan(planId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Subscription plan deleted.',
        toErrorMessage: toAdminSubscriptionErrorMessage,
      },
			async () => {
				await apiFetch(`/api/v1/admin/subscriptions/${planId}`, {
					method: 'DELETE',
				})
        await reload()
      },
    )
  }

  async function setDefaultPlan(planId: string) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Default subscription plan updated.',
        toErrorMessage: toAdminSubscriptionErrorMessage,
      },
			async () => {
				const plan = await apiFetch<SubscriptionPlan>(
					`/api/v1/admin/subscriptions/${planId}/default`,
					{ method: 'PUT' },
				)
        await reload()
        return plan
      },
    )
  }

  return {
    createPlan,
    defaultPlan,
    deletePlan,
    listError: queryList.listError,
    loadAllPlans,
    loadPlan,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    plans: queryList.items,
    reload,
    saving,
    selectedPlan,
    setDefaultPlan,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSort: queryList.setSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
    updatePlan,
  }
}
