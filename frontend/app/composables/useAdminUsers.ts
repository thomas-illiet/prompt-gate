import type { UserToken, UserTokenListResponse } from '~/types/user-service'
import type {
  FirewallMoveDirection,
  FirewallRule,
  FirewallRuleListResponse,
  FirewallRulePayload,
  FirewallSimulationResponse,
} from '~/types/firewall'
import type {
  AccessGroup,
  GroupListResponse,
  ReplaceUserGroupsPayload,
} from '~/types/groups'
import type {
  AdminUser,
  UpdateUserAccessPayload,
  UpdateUserPayload,
  UserListResponse,
  UserRoleFilter,
  UserStatusFilter,
} from '~/types/users'
import type { AssignSubscriptionPlanPayload } from '~/types/subscriptions'
import { Notify } from '~/stores/notification'
import {
  BLOCKED_ROUTE_PATH,
  hasRequiredRole,
  isBlockedUser,
} from '~/utils/auth'
import {
  adminGroupsPath,
  adminUserPath,
  adminUsersPath,
  withApiQuery,
} from '~/utils/api-paths'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  firewall_rule_not_found: 'Firewall rule no longer exists.',
  invalid_action: 'Selected firewall action is invalid.',
  invalid_direction: 'Priority direction is invalid.',
  invalid_expiration: 'Expiration date must be in the future.',
  invalid_ipv4_address: 'Address must be an IPv4 address.',
  invalid_note: 'Notes must be 2,000 characters or fewer.',
  group_not_found: 'Group no longer exists.',
  invalid_role: 'Selected role is invalid.',
  invalid_sort: 'Selected user sort is invalid.',
  invalid_subscription_assignment: 'Selected subscription plan is invalid.',
  priority_conflict: 'Another firewall rule already uses this priority.',
  priority_out_of_range: 'Priority must be between 1 and 9999.',
  token_not_found: 'Virtual key no longer exists.',
  user_not_found: 'User no longer exists.',
}

// toAdminUserErrorMessage converts user API errors into user-facing text.
export function toAdminUserErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected user management error.',
  )
}

// useAdminUsers coordinates user filters, list state, and admin mutations.
export function useAdminUsers() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()
  const authStore = useAuthStore()
  const route = useRoute()

  const role = shallowRef<UserRoleFilter>('all')
  const status = shallowRef<UserStatusFilter>('all')
  const saving = shallowRef(false)
  const selectedUser = shallowRef<AdminUser | null>(null)
  const tokens = shallowRef<UserToken[]>([])
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
  const simulatingFirewall = shallowRef(false)
  const groupOptionsLoading = shallowRef(false)
  const userGroupsLoading = shallowRef(false)
  const groupLoading = computed(
    () => groupOptionsLoading.value || userGroupsLoading.value,
  )
  const groupOptions = shallowRef<AccessGroup[]>([])
  const userGroups = shallowRef<AccessGroup[]>([])

  const queryList = useQueryList<AdminUser>({
    debounceMs: 80,
    fetch: (queryString) =>
      apiFetch<UserListResponse>(`${adminUsersPath}?${queryString}`),
    initialSortBy: 'lastLoginAt',
    initialSortDir: 'desc',
    params: () => ({
      role: role.value !== 'all' ? role.value : undefined,
      status: status.value !== 'all' ? status.value : undefined,
    }),
    toErrorMessage: toAdminUserErrorMessage,
  })

  const nextFirewallPriority = computed(() => {
    const maxPriority = firewallRules.value.reduce(
      (max, rule) => Math.max(max, rule.priority),
      0,
    )
    return Math.min(maxPriority + 1, 9999)
  })

  // setSearch updates the user search filter.
  function setSearch(value: string) {
    queryList.setSearch(value)
  }

  // setRole updates the role filter and resets pagination.
  function setRole(value: UserRoleFilter) {
    role.value = value
    queryList.setPage(1)
  }

  // setStatus updates the status filter and resets pagination.
  function setStatus(value: UserStatusFilter) {
    status.value = value
    queryList.setPage(1)
  }

  // setPage updates the current user list page.
  function setPage(value: number) {
    queryList.setPage(value)
  }

  // setPageSize updates the user list page size.
  function setPageSize(value: number) {
    queryList.setPageSize(value)
  }

  // setSort updates user list sorting.
  function setSort(sortBy: string, sortDir: 'asc' | 'desc') {
    queryList.setSort(sortBy, sortDir)
  }

  // reload refreshes the current user list.
  async function reload() {
    await queryList.reload()
  }

  // loadUser fetches one user for editing.
  async function loadUser(userId: string) {
    selectedUser.value = await apiFetch<AdminUser>(adminUserPath(userId))
    return selectedUser.value
  }

  // loadTokens fetches paged virtual key rows for a selected user.
  async function loadTokens(userId: string) {
    tokenLoading.value = true

    try {
      const params = new URLSearchParams({
        page: tokenPage.value.toString(),
        pageSize: tokenPageSize.value.toString(),
        sortBy: tokenSortBy.value,
        sortDir: tokenSortDir.value,
      })

      const response = await apiFetch<UserTokenListResponse>(
        withApiQuery(adminUserPath(userId, 'tokens'), params),
      )
      tokens.value = response.items
      tokenTotal.value = response.total
      return tokens.value
    } catch (error) {
      Notify.error(toAdminUserErrorMessage(error))
      throw error
    } finally {
      tokenLoading.value = false
    }
  }

  // loadGroups fetches group options for user membership management.
  async function loadGroups() {
    groupOptionsLoading.value = true
    try {
      const items: AccessGroup[] = []
      const pageSize = 100

      for (let page = 1; ; page += 1) {
        const params = new URLSearchParams({
          page: page.toString(),
          pageSize: pageSize.toString(),
          sortBy: 'name',
          sortDir: 'asc',
        })
        const response = await apiFetch<GroupListResponse>(
          withApiQuery(adminGroupsPath, params),
        )
        items.push(...response.items)
        if (items.length >= response.total || response.items.length === 0) {
          break
        }
      }

      groupOptions.value = items
      return groupOptions.value
    } catch (error) {
      Notify.error(toAdminUserErrorMessage(error))
      throw error
    } finally {
      groupOptionsLoading.value = false
    }
  }

  // loadUserGroups fetches one user's current group memberships.
  async function loadUserGroups(userId: string) {
    userGroupsLoading.value = true
    try {
      userGroups.value = await apiFetch<AccessGroup[]>(
        adminUserPath(userId, 'groups'),
      )
      return userGroups.value
    } catch (error) {
      Notify.error(toAdminUserErrorMessage(error))
      throw error
    } finally {
      userGroupsLoading.value = false
    }
  }

  // replaceUserGroups replaces the selected user's group memberships.
  async function replaceUserGroups(userId: string, groupIds: string[]) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'User groups updated.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        const payload: ReplaceUserGroupsPayload = { groupIds }
        userGroups.value = await apiJson<AccessGroup[]>(
          adminUserPath(userId, 'groups'),
          payload,
          { method: 'PUT' },
        )
        await queryList.reload()
        return userGroups.value
      },
    )
  }

  // setTokenPage updates token pagination.
  function setTokenPage(value: number) {
    tokenPage.value = value
  }

  // setTokenPageSize updates token page size and returns to the first page.
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

  // loadFirewallRules fetches scoped firewall rules for one user.
  async function loadFirewallRules(userId: string) {
    firewallLoading.value = true

    try {
      const params = new URLSearchParams({
        page: firewallPage.value.toString(),
        pageSize: firewallPageSize.value.toString(),
        sortBy: firewallSortBy.value,
        sortDir: firewallSortDir.value,
      })
      const response = await apiFetch<FirewallRuleListResponse>(
        withApiQuery(adminUserPath(userId, 'firewall', 'rules'), params),
      )
      firewallRules.value = response.items
      firewallTotal.value = response.total
      return firewallRules.value
    } catch (error) {
      Notify.error(toAdminUserErrorMessage(error))
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
    userId: string,
    payload: FirewallRulePayload,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule created.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        const response = await apiJson<FirewallRule>(
          adminUserPath(userId, 'firewall', 'rules'),
          payload,
          { method: 'POST' },
        )

        await loadFirewallRules(userId)
        return response
      },
    )
  }

  // updateFirewallRule patches a scoped firewall rule and reloads rows.
  async function updateFirewallRule(
    userId: string,
    ruleId: string,
    payload: FirewallRulePayload,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule updated.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        const response = await apiJson<FirewallRule>(
          adminUserPath(userId, 'firewall', 'rules', ruleId),
          payload,
          { method: 'PATCH' },
        )

        await loadFirewallRules(userId)
        return response
      },
    )
  }

  // moveFirewallRulePriority swaps a scoped firewall rule with its neighbor.
  async function moveFirewallRulePriority(
    userId: string,
    ruleId: string,
    direction: FirewallMoveDirection,
  ) {
    return await runApiMutation(
      { loading: saving, toErrorMessage: toAdminUserErrorMessage },
      async () => {
        const response = await apiJson<FirewallRule>(
          adminUserPath(userId, 'firewall', 'rules', ruleId, 'priority'),
          { direction },
          { method: 'PATCH' },
        )

        await loadFirewallRules(userId)
        return response
      },
    )
  }

  // deleteFirewallRule removes a scoped firewall rule and refreshes rows.
  async function deleteFirewallRule(userId: string, ruleId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Firewall rule deleted.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(
          adminUserPath(userId, 'firewall', 'rules', ruleId),
          { method: 'DELETE' },
        )
        await loadFirewallRules(userId)
      },
    )
  }

  // simulateFirewallIp runs a scoped firewall match simulation for one client IP.
  async function simulateFirewallIp(userId: string, clientIp: string) {
    simulatingFirewall.value = true

    try {
      return await apiJson<FirewallSimulationResponse>(
        adminUserPath(userId, 'firewall', 'simulate'),
        { clientIp },
        { method: 'POST' },
      )
    } catch (error) {
      throw new Error(toAdminUserErrorMessage(error), { cause: error })
    } finally {
      simulatingFirewall.value = false
    }
  }

  // updateUser patches a user and reconciles current-user access changes.
  async function updateUser(userId: string, payload: UpdateUserPayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'User updated.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        const updatedUser = await apiJson<AdminUser>(
          adminUserPath(userId),
          payload,
          { method: 'PATCH' },
        )

        selectedUser.value = updatedUser
        await handleCurrentUserMutation(updatedUser.id)
        await queryList.reload()
        return updatedUser
      },
    )
  }

  async function assignUserSubscriptionPlan(
    userId: string,
    planId: string | null,
  ) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'User subscription updated.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        const payload: AssignSubscriptionPlanPayload = { planId }
        const updatedUser = await apiJson<AdminUser>(
          adminUserPath(userId, 'subscription-plan'),
          payload,
          { method: 'PUT' },
        )

        if (selectedUser.value?.id === updatedUser.id) {
          selectedUser.value = updatedUser
        }
        await queryList.reload()
        return updatedUser
      },
    )
  }

  async function updateUserAccess(
    userId: string,
    payload: UpdateUserAccessPayload,
  ) {
    const updatedUser = await updateUser(userId, {
      role: payload.role,
      isActive: payload.isActive,
      firewallOverrideEnabled: payload.firewallOverrideEnabled,
      expiresAt: payload.expiresAt,
    })
    if (updatedUser.subscriptionPlanId !== payload.subscriptionPlanId) {
      return await assignUserSubscriptionPlan(
        userId,
        payload.subscriptionPlanId,
      )
    }
    return updatedUser
  }

  // updateUserNote stores the dedicated admin note for one user.
  async function updateUserNote(userId: string, note: string) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'User note updated.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        const updatedUser = await apiJson<AdminUser>(
          adminUserPath(userId, 'note'),
          { note },
          { method: 'PATCH' },
        )

        if (selectedUser.value?.id === updatedUser.id) {
          selectedUser.value = updatedUser
        }
        await queryList.reload()
        return updatedUser
      },
    )
  }

  // deleteUser removes a user and handles current-user side effects.
  async function deleteUser(userId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'User deleted.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        await apiFetch(adminUserPath(userId), {
          method: 'DELETE',
        })

        await handleCurrentUserMutation(userId)

        if (queryList.items.value.length === 1 && queryList.page.value > 1) {
          queryList.setPage(queryList.page.value - 1)
        } else {
          await queryList.reload()
        }
      },
    )
  }

  // revokeUserToken revokes one token for the selected user and reloads rows.
  async function revokeUserToken(userId: string, tokenId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Virtual key revoked.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(adminUserPath(userId, 'tokens', tokenId), {
          method: 'DELETE',
        })
        await loadTokens(userId)
      },
    )
  }

  // handleCurrentUserMutation redirects when the current user's access changes.
  async function handleCurrentUserMutation(userId: string) {
    if (authStore.user?.id !== userId) {
      return
    }

    const stillAuthenticated = await authStore.refresh()
    if (!stillAuthenticated) {
      await navigateTo({
        path: '/login',
        query: { redirect: route.fullPath },
      })
      return
    }

    if (
      isBlockedUser(authStore.user) ||
      !hasRequiredRole(authStore.user, ['admin'])
    ) {
      await navigateTo(BLOCKED_ROUTE_PATH)
    }
  }

  return {
    deleteUser,
    assignUserSubscriptionPlan,
    createFirewallRule,
    deleteFirewallRule,
    firewallLoading,
    firewallPage,
    firewallPageSize,
    firewallRules,
    firewallSortBy,
    firewallSortDir,
    firewallTotal,
    listError: queryList.listError,
    groupLoading,
    groupOptions,
    loadFirewallRules,
    loadTokens,
    loadGroups,
    loadUserGroups,
    loadUser,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    moveFirewallRulePriority,
    nextFirewallPriority,
    reload,
    replaceUserGroups,
    role,
    saving,
    search: queryList.search,
    selectedUser,
    setPage,
    setPageSize,
    setRole,
    setSearch,
    setSort,
    setStatus,
    setFirewallPage,
    setFirewallPageSize,
    setFirewallSort,
    setTokenPage,
    setTokenPageSize,
    setTokenSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    status,
    simulatingFirewall,
    simulateFirewallIp,
    tokenLoading,
    tokenPage,
    tokenPageSize,
    tokens,
    tokenSortBy,
    tokenSortDir,
    tokenTotal,
    total: queryList.total,
    updateUser,
    updateUserAccess,
    updateFirewallRule,
    updateUserNote,
    userGroups,
    users: queryList.items,
    revokeUserToken,
  }
}
