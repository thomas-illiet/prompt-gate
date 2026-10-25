import { computed, onScopeDispose, shallowRef, watch } from 'vue'

import type {
  AdminQuotaResetEventItem,
  AdminQuotaResetEventsResponse,
  AdminQuotaResetEventsSortBy,
  AdminQuotaResetEventsSummary,
  AdminQuotaResetEventStatusFilter,
  AdminQuotaResetEventTypeFilter,
  AdminQuotaResetEventWindow,
} from '~/types/quota-resets'
import { toApiErrorMessage } from '~/utils/api-error'
import { adminQuotaResetEventsPath, withApiQuery } from '~/utils/api-paths'
import type { AppTableSortDir } from '~/utils/table'

const emptySummary: AdminQuotaResetEventsSummary = {
  monthlyGrants: 0,
  adminGrants: 0,
  appliedResetUses: 0,
  pendingResetUsesTotal: 0,
}

function isAbortError(error: unknown) {
  return error instanceof Error && error.name === 'AbortError'
}

// useAdminQuotaResetEvents owns the audit-style credit and reset event list.
export function useAdminQuotaResetEvents() {
  const apiFetch = useApiFetch()
  const items = shallowRef<AdminQuotaResetEventItem[]>([])
  const summary = shallowRef<AdminQuotaResetEventsSummary>(emptySummary)
  const startsAt = shallowRef<string | null>(null)
  const endsAt = shallowRef<string | null>(null)
  const total = shallowRef(0)
  const page = shallowRef(1)
  const pageSize = shallowRef(20)
  const window = shallowRef<AdminQuotaResetEventWindow>('30d')
  const type = shallowRef<AdminQuotaResetEventTypeFilter>('all')
  const status = shallowRef<AdminQuotaResetEventStatusFilter>('all')
  const search = shallowRef('')
  const debouncedSearch = shallowRef('')
  const sortBy = shallowRef<AdminQuotaResetEventsSortBy>('createdAt')
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
      window: window.value,
    })
    if (type.value !== 'all') params.set('type', type.value)
    if (status.value !== 'all') params.set('status', status.value)
    const normalizedSearch = debouncedSearch.value.trim()
    if (normalizedSearch) params.set('search', normalizedSearch)
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
      const response = await apiFetch<AdminQuotaResetEventsResponse>(
        withApiQuery(
          adminQuotaResetEventsPath,
          new URLSearchParams(requestedQuery),
        ),
        { signal: controller.signal },
      )
      if (version !== requestVersion || queryString.value !== requestedQuery) {
        return
      }
      items.value = response.items
      summary.value = response.summary
      startsAt.value = response.startsAt
      endsAt.value = response.endsAt
      total.value = response.total
    } catch (cause) {
      if (version === requestVersion && !isAbortError(cause)) {
        error.value = toApiErrorMessage(
          cause,
          {},
          'Unable to load credit and reset events.',
        )
      }
    } finally {
      if (version === requestVersion) loading.value = false
    }
  }

  function setWindow(value: AdminQuotaResetEventWindow) {
    window.value = value
    page.value = 1
  }

  function setType(value: AdminQuotaResetEventTypeFilter) {
    type.value = value
    page.value = 1
  }

  function setStatus(value: AdminQuotaResetEventStatusFilter) {
    status.value = value
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
    const allowed: AdminQuotaResetEventsSortBy[] = [
      'createdAt',
      'appliedAt',
      'user',
      'type',
      'status',
    ]
    sortBy.value = allowed.includes(value as AdminQuotaResetEventsSortBy)
      ? (value as AdminQuotaResetEventsSortBy)
      : 'createdAt'
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
    endsAt,
    error,
    items,
    loading,
    page,
    pageSize,
    reload: load,
    search,
    setPage,
    setPageSize,
    setSearch,
    setSort,
    setStatus,
    setType,
    setWindow,
    sortBy,
    sortDir,
    startsAt,
    status,
    summary,
    total,
    type,
    window,
  }
}
