import type {
  CreatedUserToken,
  UserToken,
  UserTokenListResponse,
  UserTokenPayload,
  UserTokenStats,
  UserTokenStatusFilter,
} from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'
import { userTokenStatus } from '~/utils/user-tokens'

const ERROR_MESSAGES = {
  invalid_sort: 'Selected virtual key sort is invalid.',
  invalid_token_name:
    'Virtual key name must use lowercase letters, numbers, dashes, or underscores.',
  invalid_token_ttl: 'Virtual key lifetime must be between 1 and 365 days.',
  token_not_found: 'Virtual key no longer exists.',
}

// toUserTokenErrorMessage converts token API errors into user-facing text.
export function toUserTokenErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected virtual key management error.',
  )
}

// useUserTokens coordinates token list state, creation, and revocation.
export function useUserTokens() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const createdToken = shallowRef<CreatedUserToken | null>(null)
  const statusFilter = shallowRef<UserTokenStatusFilter>('active')
  const saving = shallowRef(false)

  const queryList = useQueryList<UserToken>({
    debounceMs: 80,
    fetch: (queryString) =>
      apiFetch<UserTokenListResponse>(`/api/v1/tokens?${queryString}`),
    initialSortBy: 'createdAt',
    initialSortDir: 'desc',
    params: () => ({
      status: statusFilter.value !== 'all' ? statusFilter.value : undefined,
    }),
    toErrorMessage: toUserTokenErrorMessage,
  })

  const tokenStats = computed<UserTokenStats>(() =>
    queryList.items.value.reduce(
      (stats, token) => {
        stats.all += 1
        stats[userTokenStatus(token)] += 1
        return stats
      },
      { active: 0, all: 0, expired: 0, revoked: 0 },
    ),
  )

  const activeTokensCount = computed(() => tokenStats.value.active)

  // reload refreshes the current token list.
  async function reload() {
    await queryList.reload()
  }

  // setSearch updates the token search filter.
  function setSearch(value: string) {
    queryList.setSearch(value)
  }

  // setStatusFilter updates token status filtering and resets pagination.
  function setStatusFilter(value: UserTokenStatusFilter) {
    statusFilter.value = value
    queryList.setPage(1)
  }

  // createToken creates a token and stores the one-time secret response.
  async function createToken(payload: UserTokenPayload) {
    createdToken.value = null

    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Virtual key created.',
        toErrorMessage: toUserTokenErrorMessage,
      },
      async () => {
        const response = await apiJson<CreatedUserToken>(
          '/api/v1/tokens',
          payload,
          { method: 'POST' },
        )

        createdToken.value = response
        await queryList.reload()
        return response
      },
    )
  }

  // revokeToken revokes a token and reloads the list.
  async function revokeToken(tokenId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Virtual key revoked.',
        toErrorMessage: toUserTokenErrorMessage,
      },
      async () => {
        await apiFetch<unknown>(`/api/v1/tokens/${tokenId}`, {
          method: 'DELETE',
        })
        await queryList.reload()
      },
    )
  }

  return {
    activeTokensCount,
    createToken,
    createdToken,
    filteredTokens: queryList.items,
    listError: queryList.listError,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    reload,
    revokeToken,
    saving,
    search: queryList.search,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSearch,
    setSort: queryList.setSort,
    setStatusFilter,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    statusFilter,
    tokenStats,
    tokens: queryList.items,
    total: queryList.total,
  }
}
