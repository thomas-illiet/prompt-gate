import type { UserListResponse } from '~/types/users'
import type {
  AdminPromptHistoryItem,
  AdminPromptHistoryResponse,
} from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_pagination: 'Prompt history pagination is invalid.',
  invalid_sort: 'Selected prompt history sort is invalid.',
}

// toAdminPromptHistoryErrorMessage converts admin prompt API errors into text.
export function toAdminPromptHistoryErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected admin prompt history error.',
  )
}

// useAdminPromptHistory exposes global prompt history state for admins.
export function useAdminPromptHistory() {
  const apiFetch = useApiFetch()
  const userId = shallowRef('')
  const users = shallowRef<UserListResponse['items']>([])
  const usersError = shallowRef<string | null>(null)
  const loadingUsers = shallowRef(false)
  let usersRequestVersion = 0

  const queryList = useQueryList<AdminPromptHistoryItem>({
    debounceMs: 80,
    fetch: (queryString) =>
      apiFetch<AdminPromptHistoryResponse>(
        `/api/v1/admin/prompts?${queryString}`,
      ),
    initialSortBy: 'createdAt',
    initialSortDir: 'desc',
    params: () => ({
      userId: userId.value || undefined,
    }),
    toErrorMessage: toAdminPromptHistoryErrorMessage,
  })

  // loadUsers refreshes admin user options for the user filter.
  async function loadUsers(search = '') {
    const version = ++usersRequestVersion
    loadingUsers.value = true
    usersError.value = null

    const params = new URLSearchParams({
      page: '1',
      pageSize: '100',
      sortBy: 'preferredUsername',
      sortDir: 'asc',
    })
    const normalizedSearch = search.trim()
    if (normalizedSearch) {
      params.set('search', normalizedSearch)
    }

    try {
      const response = await apiFetch<UserListResponse>(
        `/api/v1/admin/users?${params.toString()}`,
      )
      if (version !== usersRequestVersion) {
        return
      }

      users.value = response.items
    } catch (error) {
      if (version === usersRequestVersion) {
        usersError.value = toApiErrorMessage(
          error,
          {},
          'Unable to load user filter options.',
        )
      }
    } finally {
      if (version === usersRequestVersion) {
        loadingUsers.value = false
      }
    }
  }

  // setUserId updates the exact user filter and resets pagination.
  function setUserId(value: string | null) {
    userId.value = value ?? ''
    queryList.setPage(1)
  }

  void loadUsers()

  onScopeDispose(() => {
    usersRequestVersion += 1
  }, true)

  return {
    listError: queryList.listError,
    loading: queryList.loading,
    loadingUsers,
    page: queryList.page,
    pageSize: queryList.pageSize,
    prompts: queryList.items,
    reload: queryList.reload,
    search: queryList.search,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSearch: queryList.setSearch,
    setSort: queryList.setSort,
    setUserId,
    setUserSearch: loadUsers,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
    userId,
    users,
    usersError,
  }
}
