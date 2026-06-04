import type {
  FirewallMoveDirection,
  FirewallRule,
  FirewallRuleListResponse,
  FirewallRulePayload,
  FirewallSimulationResponse,
} from '~/types/firewall'
import type {
  CreatedTokenResponse,
  ServiceAccount,
  ServiceAccountFormPayload,
  ServiceAccountListResponse,
  ServiceAccountPayload,
  TokenListResponse,
  TokenPayload,
  TokenResponse,
} from '~/types/service-accounts'
import type { AssignSubscriptionPlanPayload } from '~/types/subscriptions'
import { Notify } from '~/stores/notification'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  firewall_rule_not_found: 'Firewall rule no longer exists.',
  identifier_conflict: 'Another service account already uses this identifier.',
  invalid_action: 'Selected firewall action is invalid.',
  invalid_direction: 'Priority direction is invalid.',
  invalid_identifier:
    'Identifier must use lowercase letters, numbers, dashes, or underscores.',
  invalid_ipv4_address: 'Address must be an IPv4 address.',
  invalid_name: 'Name is required.',
  invalid_note: 'Notes must be 2,000 characters or fewer.',
  invalid_sort: 'Selected service account sort is invalid.',
  invalid_subscription_assignment: 'Selected subscription plan is invalid.',
  invalid_token_name:
    'Virtual key name must use lowercase letters, numbers, dashes, or underscores.',
  invalid_token_ttl: 'Virtual key lifetime must be between 1 and 365 days.',
  priority_conflict: 'Another firewall rule already uses this priority.',
  priority_out_of_range: 'Priority must be between 1 and 9999.',
  service_account_not_found: 'Service account no longer exists.',
  token_not_found: 'Virtual key no longer exists.',
}

// toAdminServiceAccountErrorMessage converts service account API errors into text.
export function toAdminServiceAccountErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected service account management error.',
  )
}

// useAdminServiceAccounts coordinates account list, account mutations, and tokens.
export function useAdminServiceAccounts() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const tokens = shallowRef<TokenResponse[]>([])
  const tokenLoading = shallowRef(false)
  const tokenPage = shallowRef(1)
  const tokenPageSize = shallowRef(10)
  const tokenSortBy = shallowRef('createdAt')
  const tokenSortDir = shallowRef<'asc' | 'desc'>('desc')
  const tokenTotal = shallowRef(0)
  const firewallRules = shallowRef<FirewallRule[]>([])
  const firewallLoading = shallowRef(false)
  const firewallPage = shallowRef(1)
  const firewallPageSize = shallowRef(10)
  const firewallSortBy = shallowRef('priority')
  const firewallSortDir = shallowRef<'asc' | 'desc'>('asc')
  const firewallTotal = shallowRef(0)
  const saving = shallowRef(false)
  const simulatingFirewall = shallowRef(false)
  const selectedAccount = shallowRef<ServiceAccount | null>(null)
  const createdToken = shallowRef<CreatedTokenResponse | null>(null)

  const accountList = useQueryList<ServiceAccount>({
    fetch: (queryString) =>
      apiFetch<ServiceAccountListResponse>(
        `/api/v1/admin/service-accounts?${queryString}`,
      ),
    initialSortBy: 'createdAt',
    initialSortDir: 'desc',
    toErrorMessage: toAdminServiceAccountErrorMessage,
  })

  const activeAccountsCount = computed(
    () => accountList.items.value.filter((account) => account.isActive).length,
  )
  const nextFirewallPriority = computed(() => {
    const maxPriority = firewallRules.value.reduce(
      (max, rule) => Math.max(max, rule.priority),
      0,
    )
    return Math.min(maxPriority + 1, 9999)
  })

  // fetchAccounts refreshes service accounts through the shared list composable.
  async function fetchAccounts() {
    await accountList.reload()
  }

  // reload exposes a stable refresh action for service account views.
  async function reload() {
    await fetchAccounts()
  }

  // createAccount stores a new service account and reloads the list.
  async function createAccount(payload: ServiceAccountPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Service account created.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        const account = await apiJson<ServiceAccount>(
          '/api/v1/admin/service-accounts',
          payload,
          { method: 'POST' },
        )

        await fetchAccounts()
        return account
      },
    )
  }

  async function createAccountWithSubscription(payload: ServiceAccountFormPayload) {
    const account = await createAccount({
      identifier: payload.identifier,
      name: payload.name,
      isActive: payload.isActive,
      firewallOverrideEnabled: payload.firewallOverrideEnabled,
    })
    if (payload.subscriptionPlanId !== account.subscriptionPlanId) {
      return await assignServiceAccountSubscriptionPlan(
        account.id,
        payload.subscriptionPlanId,
      )
    }
    return account
  }

  // loadAccount fetches one service account for editing.
  async function loadAccount(accountId: string) {
    selectedAccount.value = await apiFetch<ServiceAccount>(
      `/api/v1/admin/service-accounts/${accountId}`,
    )
    return selectedAccount.value
  }

  // updateAccount patches a service account and keeps the selected copy fresh.
  async function updateAccount(
    accountId: string,
    payload: ServiceAccountPayload,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Service account updated.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        const account = await apiJson<ServiceAccount>(
          `/api/v1/admin/service-accounts/${accountId}`,
          payload,
          { method: 'PATCH' },
        )

        selectedAccount.value = account
        await fetchAccounts()
        return account
      },
    )
  }

  async function updateAccountWithSubscription(
    accountId: string,
    payload: ServiceAccountFormPayload,
  ) {
    const account = await updateAccount(accountId, {
      identifier: payload.identifier,
      name: payload.name,
      isActive: payload.isActive,
      firewallOverrideEnabled: payload.firewallOverrideEnabled,
    })
    if (account.subscriptionPlanId !== payload.subscriptionPlanId) {
      return await assignServiceAccountSubscriptionPlan(
        accountId,
        payload.subscriptionPlanId,
      )
    }
    return account
  }

  async function assignServiceAccountSubscriptionPlan(
    accountId: string,
    planId: string | null,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Service account subscription updated.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        const payload: AssignSubscriptionPlanPayload = { planId }
        const account = await apiJson<ServiceAccount>(
          `/api/v1/admin/service-accounts/${accountId}/subscription-plan`,
          payload,
          { method: 'PUT' },
        )

        selectedAccount.value = account
        await fetchAccounts()
        return account
      },
    )
  }

  // updateAccountNote stores the dedicated admin note for one service account.
  async function updateAccountNote(accountId: string, note: string) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Service account note updated.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        const account = await apiJson<ServiceAccount>(
          `/api/v1/admin/service-accounts/${accountId}/note`,
          { note },
          { method: 'PATCH' },
        )

        if (selectedAccount.value?.id === account.id) {
          selectedAccount.value = account
        }
        await fetchAccounts()
        return account
      },
    )
  }

  // deleteAccount removes a service account and refreshes the list.
  async function deleteAccount(accountId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Service account deleted.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(`/api/v1/admin/service-accounts/${accountId}`, {
          method: 'DELETE',
        })
        await fetchAccounts()
      },
    )
  }

  // loadTokens fetches paged service account tokens with current token filters.
  async function loadTokens(accountId: string, includeRevoked = false) {
    tokenLoading.value = true

    try {
      const params = new URLSearchParams({
        page: tokenPage.value.toString(),
        pageSize: tokenPageSize.value.toString(),
        sortBy: tokenSortBy.value,
        sortDir: tokenSortDir.value,
      })
      if (includeRevoked) {
        params.set('includeRevoked', 'true')
      }

      const response = await apiFetch<TokenListResponse>(
        `/api/v1/admin/service-accounts/${accountId}/tokens?${params.toString()}`,
      )
      tokens.value = response.items
      tokenTotal.value = response.total
      return tokens.value
    } catch (error) {
      Notify.error(toAdminServiceAccountErrorMessage(error))
      throw error
    } finally {
      tokenLoading.value = false
    }
  }

  // setTokenPage updates the token list page.
  function setTokenPage(value: number) {
    tokenPage.value = value
  }

  // setTokenPageSize updates token page size and resets pagination.
  function setTokenPageSize(value: number) {
    tokenPageSize.value = value
    tokenPage.value = 1
  }

  // setTokenSort updates token sorting and returns to the first page.
  function setTokenSort(sortBy: string, sortDir: 'asc' | 'desc') {
    tokenSortBy.value = sortBy
    tokenSortDir.value = sortDir
    tokenPage.value = 1
  }

  // loadFirewallRules fetches scoped firewall rules for one service account.
  async function loadFirewallRules(accountId: string) {
    firewallLoading.value = true

    try {
      const params = new URLSearchParams({
        page: firewallPage.value.toString(),
        pageSize: firewallPageSize.value.toString(),
        sortBy: firewallSortBy.value,
        sortDir: firewallSortDir.value,
      })
      const response = await apiFetch<FirewallRuleListResponse>(
        `/api/v1/admin/service-accounts/${accountId}/firewall/rules?${params.toString()}`,
      )
      firewallRules.value = response.items
      firewallTotal.value = response.total
      return firewallRules.value
    } catch (error) {
      Notify.error(toAdminServiceAccountErrorMessage(error))
      throw error
    } finally {
      firewallLoading.value = false
    }
  }

  // setFirewallPage updates scoped firewall pagination.
  function setFirewallPage(value: number) {
    firewallPage.value = value
  }

  // setFirewallPageSize updates scoped firewall page size and resets pagination.
  function setFirewallPageSize(value: number) {
    firewallPageSize.value = value
    firewallPage.value = 1
  }

  // setFirewallSort updates scoped firewall sorting and returns to the first page.
  function setFirewallSort(sortBy: string, sortDir: 'asc' | 'desc') {
    firewallSortBy.value = sortBy
    firewallSortDir.value = sortDir
    firewallPage.value = 1
  }

  // createFirewallRule creates a scoped firewall rule and reloads rows.
  async function createFirewallRule(
    accountId: string,
    payload: FirewallRulePayload,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule created.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        const response = await apiJson<FirewallRule>(
          `/api/v1/admin/service-accounts/${accountId}/firewall/rules`,
          payload,
          { method: 'POST' },
        )

        await loadFirewallRules(accountId)
        return response
      },
    )
  }

  // updateFirewallRule patches a scoped firewall rule and reloads rows.
  async function updateFirewallRule(
    accountId: string,
    ruleId: string,
    payload: FirewallRulePayload,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule updated.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        const response = await apiJson<FirewallRule>(
          `/api/v1/admin/service-accounts/${accountId}/firewall/rules/${ruleId}`,
          payload,
          { method: 'PATCH' },
        )

        await loadFirewallRules(accountId)
        return response
      },
    )
  }

  // moveFirewallRulePriority swaps a scoped firewall rule with its neighbor.
  async function moveFirewallRulePriority(
    accountId: string,
    ruleId: string,
    direction: FirewallMoveDirection,
  ) {
    return await runApiMutation(
      { loading: saving, toErrorMessage: toAdminServiceAccountErrorMessage },
      async () => {
        const response = await apiJson<FirewallRule>(
          `/api/v1/admin/service-accounts/${accountId}/firewall/rules/${ruleId}/priority`,
          { direction },
          { method: 'PATCH' },
        )

        await loadFirewallRules(accountId)
        return response
      },
    )
  }

  // deleteFirewallRule removes a scoped firewall rule and refreshes rows.
  async function deleteFirewallRule(accountId: string, ruleId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule deleted.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(
          `/api/v1/admin/service-accounts/${accountId}/firewall/rules/${ruleId}`,
          { method: 'DELETE' },
        )
        await loadFirewallRules(accountId)
      },
    )
  }

  // simulateFirewallIp runs a scoped firewall match simulation for one client IP.
  async function simulateFirewallIp(accountId: string, clientIp: string) {
    simulatingFirewall.value = true

    try {
      return await apiJson<FirewallSimulationResponse>(
        `/api/v1/admin/service-accounts/${accountId}/firewall/simulate`,
        { clientIp },
        { method: 'POST' },
      )
    } catch (error) {
      throw new Error(toAdminServiceAccountErrorMessage(error))
    } finally {
      simulatingFirewall.value = false
    }
  }

  // createToken creates a service account token and stores the one-time secret.
  async function createToken(
    accountId: string,
    payload: TokenPayload,
    includeRevoked = false,
  ) {
    createdToken.value = null

    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Virtual key created.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        const response = await apiJson<CreatedTokenResponse>(
          `/api/v1/admin/service-accounts/${accountId}/tokens`,
          payload,
          { method: 'POST' },
        )

        createdToken.value = response
        await loadTokens(accountId, includeRevoked)
        return response
      },
    )
  }

  // revokeToken revokes a service account token and reloads token rows.
  async function revokeToken(
    accountId: string,
    tokenId: string,
    includeRevoked = false,
  ) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Virtual key revoked.',
        toErrorMessage: toAdminServiceAccountErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(
          `/api/v1/admin/service-accounts/${accountId}/tokens/${tokenId}`,
          { method: 'DELETE' },
        )
        await loadTokens(accountId, includeRevoked)
      },
    )
  }

  return {
    accounts: accountList.items,
    activeAccountsCount,
    assignServiceAccountSubscriptionPlan,
    createAccount,
    createAccountWithSubscription,
    createFirewallRule,
    createdToken,
    createToken,
    deleteAccount,
    deleteFirewallRule,
    firewallLoading,
    firewallPage,
    firewallPageSize,
    firewallRules,
    firewallSortBy,
    firewallSortDir,
    firewallTotal,
    listError: accountList.listError,
    loadAccount,
    loadFirewallRules,
    loading: accountList.loading,
    loadTokens,
    moveFirewallRulePriority,
    nextFirewallPriority,
    page: accountList.page,
    pageSize: accountList.pageSize,
    reload,
    revokeToken,
    saving,
    selectedAccount,
    setPage: accountList.setPage,
    setPageSize: accountList.setPageSize,
    setSort: accountList.setSort,
    setFirewallPage,
    setFirewallPageSize,
    setFirewallSort,
    setTokenPage,
    setTokenPageSize,
    setTokenSort,
    simulateFirewallIp,
    simulatingFirewall,
    sortBy: accountList.sortBy,
    sortDir: accountList.sortDir,
    tokenLoading,
    tokenPage,
    tokenPageSize,
    tokenSortBy,
    tokenSortDir,
    tokenTotal,
    tokens,
    total: accountList.total,
    updateAccount,
    updateAccountWithSubscription,
    updateAccountNote,
    updateFirewallRule,
  }
}
