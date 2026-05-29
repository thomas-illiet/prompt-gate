import type { UserToken, UserTokenListResponse } from '~/types/user-service'
import type {
  AdminUser,
  UpdateUserPayload,
  UserListResponse,
  UserRoleFilter,
  UserStatusFilter,
} from '~/types/users'
import { Notify } from '~/stores/notification'
import {
  BLOCKED_ROUTE_PATH,
  hasRequiredRole,
  isBlockedUser,
} from '~/utils/auth'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_expiration: 'Expiration date must be in the future.',
  invalid_role: 'Selected role is invalid.',
  invalid_sort: 'Selected user sort is invalid.',
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

  const queryList = useQueryList<AdminUser>({
    debounceMs: 80,
    fetch: (queryString) =>
      apiFetch<UserListResponse>(`/api/v1/admin/users?${queryString}`),
    initialSortBy: 'lastLoginAt',
    initialSortDir: 'desc',
    params: () => ({
      role: role.value !== 'all' ? role.value : undefined,
      status: status.value !== 'all' ? status.value : undefined,
    }),
    toErrorMessage: toAdminUserErrorMessage,
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
    selectedUser.value = await apiFetch<AdminUser>(
      `/api/v1/admin/users/${userId}`,
    )
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
        `/api/v1/admin/users/${userId}/tokens?${params.toString()}`,
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
          `/api/v1/admin/users/${userId}`,
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

  // deleteUser removes a user and handles current-user side effects.
  async function deleteUser(userId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'User deleted.',
        toErrorMessage: toAdminUserErrorMessage,
      },
      async () => {
        await apiFetch(`/api/v1/admin/users/${userId}`, {
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
        await apiFetch<unknown>(
          `/api/v1/admin/users/${userId}/tokens/${tokenId}`,
          { method: 'DELETE' },
        )
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
    listError: queryList.listError,
    loadTokens,
    loadUser,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    reload,
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
    setTokenPage,
    setTokenPageSize,
    setTokenSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    status,
    tokenLoading,
    tokenPage,
    tokenPageSize,
    tokens,
    tokenSortBy,
    tokenSortDir,
    tokenTotal,
    total: queryList.total,
    updateUser,
    users: queryList.items,
    revokeUserToken,
  }
}
