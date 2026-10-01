import type { AccountIPAddress, AccountIPAddressListResponse } from '~/types/account-ips'
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
import {
  adminServiceAccountPath,
  adminServiceAccountsPath,
  withApiQuery,
} from '~/utils/api-paths'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  identifier_conflict: 'Another service account already uses this identifier.',
  invalid_identifier:
    'Identifier must use lowercase letters, numbers, dashes, or underscores.',
  invalid_name: 'Name is required.',
  invalid_note: 'Notes must be 2,000 characters or fewer.',
  invalid_sort: 'Selected service account sort is invalid.',
  invalid_subscription_assignment: 'Selected subscription plan is invalid.',
  invalid_token_name:
    'Virtual key name must use lowercase letters, numbers, dashes, or underscores.',
  invalid_token_ttl: 'Virtual key lifetime must be between 1 and 365 days.',
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
  const ipAddresses = shallowRef<AccountIPAddress[]>([])
  const ipLoading = shallowRef(false)
  const ipPage = shallowRef(1)
  const ipPageSize = shallowRef(10)
  const ipSortBy = shallowRef('lastSeen')
  const ipSortDir = shallowRef<'asc' | 'desc'>('desc')
  const ipTotal = shallowRef(0)
  const saving = shallowRef(false)
  const selectedAccount = shallowRef<ServiceAccount | null>(null)
  const createdToken = shallowRef<CreatedTokenResponse | null>(null)

  const accountList = useQueryList<ServiceAccount>({
    fetch: (queryString) =>
      apiFetch<ServiceAccountListResponse>(
        `${adminServiceAccountsPath}?${queryString}`,
      ),
    initialSortBy: 'createdAt',
    initialSortDir: 'desc',
    toErrorMessage: toAdminServiceAccountErrorMessage,
  })

  const activeAccountsCount = computed(
    () => accountList.items.value.filter((account) => account.isActive).length,
  )

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
          adminServiceAccountsPath,
          payload,
          { method: 'POST' },
        )

        await fetchAccounts()
        return account
      },
    )
  }

  async function createAccountWithSubscription(
    payload: ServiceAccountFormPayload,
  ) {
    const account = await createAccount({
      identifier: payload.identifier,
      name: payload.name,
      isActive: payload.isActive,
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
      adminServiceAccountPath(accountId),
    )
    return selectedAccount.value
  }

  // loadIPAddresses fetches observed proxy IP addresses for one service account.
  async function loadIPAddresses(accountId: string) {
    ipLoading.value = true
    try {
      const params = new URLSearchParams({
        page: ipPage.value.toString(),
        pageSize: ipPageSize.value.toString(),
        sortBy: ipSortBy.value,
        sortDir: ipSortDir.value,
      })
      const response = await apiFetch<AccountIPAddressListResponse>(
        withApiQuery(adminServiceAccountPath(accountId, 'ips'), params),
      )
      ipAddresses.value = response.items
      ipTotal.value = response.total
      return ipAddresses.value
    } catch (error) {
      Notify.error(toAdminServiceAccountErrorMessage(error))
      throw error
    } finally {
      ipLoading.value = false
    }
  }

  function setIPPage(value: number) { ipPage.value = value }
  function setIPPageSize(value: number) { ipPageSize.value = value }
  function setIPSort(sortBy: string, sortDir: 'asc' | 'desc') {
    ipSortBy.value = sortBy
    ipSortDir.value = sortDir
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
          adminServiceAccountPath(accountId),
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
          adminServiceAccountPath(accountId, 'subscription-plan'),
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
          adminServiceAccountPath(accountId, 'note'),
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
        await apiFetch<unknown>(adminServiceAccountPath(accountId), {
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
        withApiQuery(adminServiceAccountPath(accountId, 'tokens'), params),
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
          adminServiceAccountPath(accountId, 'tokens'),
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
          adminServiceAccountPath(accountId, 'tokens', tokenId),
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
    createdToken,
    createToken,
    deleteAccount,
    ipAddresses,
    ipLoading,
    ipPage,
    ipPageSize,
    ipSortBy,
    ipSortDir,
    ipTotal,
    listError: accountList.listError,
    loadAccount,
    loadIPAddresses,
    loading: accountList.loading,
    loadTokens,
    page: accountList.page,
    pageSize: accountList.pageSize,
    reload,
    revokeToken,
    saving,
    selectedAccount,
    setPage: accountList.setPage,
    setPageSize: accountList.setPageSize,
    setSort: accountList.setSort,
    setIPPage,
    setIPPageSize,
    setIPSort,
    setTokenPage,
    setTokenPageSize,
    setTokenSort,
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
  }
}
