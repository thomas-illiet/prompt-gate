interface QueryListResponse<TItem> {
  items: TItem[]
  total: number
}

export type QueryListSortDir = 'asc' | 'desc'

interface QueryListOptions<TItem> {
  debounceMs?: number
  fetch: (queryString: string) => Promise<QueryListResponse<TItem>>
  initialSortBy?: string
  initialSortDir?: QueryListSortDir
  params?: () => Record<string, number | string | null | undefined>
  reloadMinLoadingMs?: number
  toErrorMessage: (error: unknown) => string
}

// useQueryList manages debounced search, pagination, sorting, and stale requests.
export function useQueryList<TItem>(options: QueryListOptions<TItem>) {
  const debounceMs = options.debounceMs ?? 120
  const reloadMinLoadingMs = options.reloadMinLoadingMs ?? 500
  const items = shallowRef<TItem[]>([])
  const total = shallowRef(0)
  const page = shallowRef(1)
  const pageSize = shallowRef(10)
  const search = shallowRef('')
  const debouncedSearch = shallowRef('')
  const sortBy = shallowRef(options.initialSortBy ?? 'createdAt')
  const sortDir = shallowRef<QueryListSortDir>(options.initialSortDir ?? 'desc')
  const loading = shallowRef(false)
  const listError = shallowRef<string | null>(null)

  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let requestVersion = 0

  async function waitForMinimumLoading(startedAt: number, minLoadingMs: number) {
    const remainingMs = minLoadingMs - (Date.now() - startedAt)
    if (remainingMs > 0) {
      await new Promise<void>((resolve) => setTimeout(resolve, remainingMs))
    }
  }

  const queryString = computed(() => {
    const params = new URLSearchParams({
      page: page.value.toString(),
      pageSize: pageSize.value.toString(),
      sortBy: sortBy.value,
      sortDir: sortDir.value,
    })

    const normalizedSearch = debouncedSearch.value.trim()
    if (normalizedSearch) {
      params.set('search', normalizedSearch)
    }

    const extraParams = options.params?.() ?? {}
    for (const [key, value] of Object.entries(extraParams)) {
      if (value !== undefined && value !== null && value !== '') {
        params.set(key, String(value))
      }
    }

    return params.toString()
  })

  watch(
    search,
    (value) => {
      if (debounceTimer) {
        clearTimeout(debounceTimer)
      }

      debounceTimer = setTimeout(() => {
        debouncedSearch.value = value
      }, debounceMs)
    },
    { immediate: true },
  )

  // fetchList loads the current query and ignores stale responses.
  async function fetchList(minLoadingMs = 0) {
    const version = ++requestVersion
    const startedAt = Date.now()
    loading.value = true
    listError.value = null

    try {
      const response = await options.fetch(queryString.value)
      if (version !== requestVersion) {
        return
      }

      items.value = response.items
      total.value = response.total
    } catch (error) {
      if (version === requestVersion) {
        listError.value = options.toErrorMessage(error)
      }
    } finally {
      if (version === requestVersion) {
        await waitForMinimumLoading(startedAt, minLoadingMs)
        if (version === requestVersion) {
          loading.value = false
        }
      }
    }
  }

  // setSearch updates search text and returns to the first page.
  function setSearch(value: string) {
    search.value = value
    page.value = 1
  }

  // setPage updates the current page.
  function setPage(value: number) {
    page.value = value
  }

  // setPageSize updates page size and resets pagination.
  function setPageSize(value: number) {
    pageSize.value = value
    page.value = 1
  }

  // setSort updates sorting and returns to the first page.
  function setSort(nextSortBy: string, nextSortDir: QueryListSortDir) {
    sortBy.value = nextSortBy
    sortDir.value = nextSortDir
    page.value = 1
  }

  // reload fetches the current query immediately.
  async function reload() {
    await fetchList(reloadMinLoadingMs)
  }

  watch(queryString, async () => fetchList(), { immediate: true })

  onScopeDispose(() => {
    if (debounceTimer) {
      clearTimeout(debounceTimer)
    }
    requestVersion += 1
  }, true)

  return {
    items,
    listError,
    loading,
    page,
    pageSize,
    queryString,
    reload,
    search,
    setPage,
    setPageSize,
    setSearch,
    setSort,
    sortBy,
    sortDir,
    total,
  }
}
