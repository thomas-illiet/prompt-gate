import type {
  FirewallMoveDirection,
  FirewallRule,
  FirewallRuleListResponse,
  FirewallRulePayload,
  FirewallSimulationResponse,
} from '~/types/firewall'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  firewall_rule_not_found: 'Firewall rule no longer exists.',
  invalid_action: 'Selected firewall action is invalid.',
  invalid_direction: 'Priority direction is invalid.',
  invalid_sort: 'Selected firewall sort is invalid.',
  invalid_ipv4_address: 'Address must be an IPv4 address.',
  priority_conflict: 'Another firewall rule already uses this priority.',
  priority_out_of_range: 'Priority must be between 1 and 9999.',
}

// toAdminFirewallErrorMessage converts firewall API errors into user-facing text.
export function toAdminFirewallErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected firewall management error.',
  )
}

// useAdminFirewall coordinates firewall list state, mutations, and simulation.
export function useAdminFirewall() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const saving = shallowRef(false)
  const simulating = shallowRef(false)
  const selectedRule = shallowRef<FirewallRule | null>(null)

  const queryList = useQueryList<FirewallRule>({
    fetch: (queryString) =>
      apiFetch<FirewallRuleListResponse>(
        `/api/v1/admin/firewall/rules?${queryString}`,
      ),
    initialSortBy: 'priority',
    initialSortDir: 'asc',
    toErrorMessage: toAdminFirewallErrorMessage,
  })

  const enabledRulesCount = computed(
    () => queryList.items.value.filter((rule) => rule.enabled).length,
  )
  const nextPriority = computed(() => {
    const maxPriority = queryList.items.value.reduce(
      (max, rule) => Math.max(max, rule.priority),
      0,
    )
    return Math.min(maxPriority + 1, 9999)
  })

  // fetchRules refreshes firewall rules through the shared list composable.
  async function fetchRules() {
    await queryList.reload()
  }

  // reload exposes a stable refresh action for views and tables.
  async function reload() {
    await fetchRules()
  }

  // createRule stores a new firewall rule and reloads the list.
  async function createRule(payload: FirewallRulePayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule created.',
        toErrorMessage: toAdminFirewallErrorMessage,
      },
      async () => {
        const rule = await apiJson<FirewallRule>(
          '/api/v1/admin/firewall/rules',
          payload,
          { method: 'POST' },
        )

        await fetchRules()
        return rule
      },
    )
  }

  // loadRule fetches one firewall rule for editing.
  async function loadRule(ruleId: string) {
    selectedRule.value = await apiFetch<FirewallRule>(
      `/api/v1/admin/firewall/rules/${ruleId}`,
    )
    return selectedRule.value
  }

  // updateRule patches a firewall rule and keeps the selected copy fresh.
  async function updateRule(ruleId: string, payload: FirewallRulePayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule updated.',
        toErrorMessage: toAdminFirewallErrorMessage,
      },
      async () => {
        const rule = await apiJson<FirewallRule>(
          `/api/v1/admin/firewall/rules/${ruleId}`,
          payload,
          { method: 'PATCH' },
        )

        selectedRule.value = rule
        await fetchRules()
        return rule
      },
    )
  }

  // moveRulePriority asks the backend to swap a rule with its neighbor.
  async function moveRulePriority(
    ruleId: string,
    direction: FirewallMoveDirection,
  ) {
    return await runApiMutation(
      { loading: saving, toErrorMessage: toAdminFirewallErrorMessage },
      async () => {
        const rule = await apiJson<FirewallRule>(
          `/api/v1/admin/firewall/rules/${ruleId}/priority`,
          { direction },
          { method: 'PATCH' },
        )

        selectedRule.value = rule
        await fetchRules()
        return rule
      },
    )
  }

  // deleteRule removes a firewall rule and refreshes the list.
  async function deleteRule(ruleId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule deleted.',
        toErrorMessage: toAdminFirewallErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(`/api/v1/admin/firewall/rules/${ruleId}`, {
          method: 'DELETE',
        })
        await fetchRules()
      },
    )
  }

  // simulateIp runs a firewall match simulation for one client IP.
  async function simulateIp(clientIp: string) {
    simulating.value = true

    try {
      return await apiJson<FirewallSimulationResponse>(
        '/api/v1/admin/firewall/simulate',
        { clientIp },
        { method: 'POST' },
      )
    } catch (error) {
      throw new Error(toAdminFirewallErrorMessage(error), { cause: error })
    } finally {
      simulating.value = false
    }
  }

  return {
    createRule,
    deleteRule,
    enabledRulesCount,
    listError: queryList.listError,
    loadRule,
    loading: queryList.loading,
    moveRulePriority,
    nextPriority,
    page: queryList.page,
    pageSize: queryList.pageSize,
    reload,
    rules: queryList.items,
    saving,
    selectedRule,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSort: queryList.setSort,
    simulateIp,
    simulating,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
    updateRule,
  }
}
