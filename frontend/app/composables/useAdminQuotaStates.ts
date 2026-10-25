import { computed, onScopeDispose, shallowRef, watch } from 'vue'

import type {
  AdminQuotaAccountTypeFilter,
  AdminQuotaStateItem,
  AdminQuotaStatesResponse,
  AdminQuotaStatesSortBy,
  AdminQuotaStatesSummary,
  AdminQuotaStatesView,
} from '~/types/quota-resets'
import { toApiErrorMessage } from '~/utils/api-error'
import { adminQuotaStatesPath, withApiQuery } from '~/utils/api-paths'
import type { AppTableSortDir } from '~/utils/table'

const emptySummary: AdminQuotaStatesSummary = {
  projectedAccounts: 0,
  atRiskAccounts: 0,
  exhaustedAccounts: 0,
}

function isAbortError(error: unknown) {
  return error instanceof Error && error.name === 'AbortError'
}

// useAdminQuotaStates owns the independently-filtered live quota projection list.
export function useAdminQuotaStates() {
  const apiFetch = useApiFetch()
  const items = shallowRef<AdminQuotaStateItem[]>([])
  const summary = shallowRef<AdminQuotaStatesSummary>(emptySummary)
  const generatedAt = shallowRef<string | null>(null)
  const projectionSyncedAt = shallowRef<string | null>(null)
  const total = shallowRef(0)
  const page = shallowRef(1)
  const pageSize = shallowRef(20)
  const view = shallowRef<AdminQuotaStatesView>('at-risk')
  const accountType = shallowRef<AdminQuotaAccountTypeFilter>('all')
  const search = shallowRef('')
  const debouncedSearch = shallowRef('')
  const sortBy = shallowRef<AdminQuotaStatesSortBy>('saturation')
  const sortDir = shallowRef<AppTableSortDir>('desc')
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)

  let controller: AbortController | null = null
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let requestVersion = 0

  const queryString = computed(() => {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
      sortBy: sortBy.value,
      sortDir: sortDir.value,
      view: view.value,
    })
    if (accountType.value !== 'all') {
      params.set('accountType', accountType.value)
    }
    const normalizedSearch = debouncedSearch.value.trim()
    if (normalizedSearch) {
      params.set('search', normalizedSearch)
    }
    return params.toString()
  })

  async function load() {
    const requestedQuery = queryString.value
    const version = ++requestVersion
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = null

    try {
      const response = await apiFetch<AdminQuotaStatesResponse>(
        withApiQuery(adminQuotaStatesPath, new URLSearchParams(requestedQuery)),
        { signal: controller.signal },
      )
      if (version !== requestVersion || queryString.value !== requestedQuery) {
        return
      }
      items.value = response.items
      summary.value = response.summary
      generatedAt.value = response.generatedAt
      projectionSyncedAt.value = response.projectionSyncedAt
      total.value = response.total
    } catch (cause) {
      if (version === requestVersion && !isAbortError(cause)) {
        error.value = toApiErrorMessage(
          cause,
          {},
          'Unable to load quota projections.',
        )
      }
    } finally {
      if (version === requestVersion) {
        loading.value = false
      }
    }
  }

  function setView(value: AdminQuotaStatesView) {
    view.value = value
    page.value = 1
  }

  function setAccountType(value: AdminQuotaAccountTypeFilter) {
    accountType.value = value
    page.value = 1
  }

  function setSearch(value: string) {
    search.value = value
    page.value = 1
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => {
      debouncedSearch.value = value
      debounceTimer = null
    }, 180)
  }

  function setPage(value: number) {
    page.value = value
  }

  function setPageSize(value: number) {
    pageSize.value = Math.min(Math.max(value, 1), 100)
    page.value = 1
  }

  function setSort(value: string, direction: AppTableSortDir) {
    const allowed: AdminQuotaStatesSortBy[] = [
      'saturation',
      'displayName',
      'resetAt',
      'syncedAt',
    ]
    sortBy.value = allowed.includes(value as AdminQuotaStatesSortBy)
      ? (value as AdminQuotaStatesSortBy)
      : 'saturation'
    sortDir.value = direction
    page.value = 1
  }

  watch(queryString, () => void load(), { immediate: true })

  onScopeDispose(() => {
    requestVersion += 1
    controller?.abort()
    if (debounceTimer) clearTimeout(debounceTimer)
  }, true)

  return {
    accountType,
    error,
    generatedAt,
    items,
    loading,
    page,
    pageSize,
    projectionSyncedAt,
    reload: load,
    search,
    setAccountType,
    setPage,
    setPageSize,
    setSearch,
    setSort,
    setView,
    sortBy,
    sortDir,
    summary,
    total,
    view,
  }
}
