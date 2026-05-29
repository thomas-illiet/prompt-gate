import type {
  PromptHistoryItem,
  PromptHistoryResponse,
} from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_pagination: 'Prompt history pagination is invalid.',
  invalid_sort: 'Selected prompt history sort is invalid.',
}

// toPromptHistoryErrorMessage converts prompt history API errors into text.
export function toPromptHistoryErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected prompt history error.',
  )
}

// usePromptHistory exposes searchable, sortable prompt history state.
export function usePromptHistory() {
  const apiFetch = useApiFetch()
  const queryList = useQueryList<PromptHistoryItem>({
    debounceMs: 80,
    fetch: (queryString) =>
      apiFetch<PromptHistoryResponse>(`/api/v1/me/prompts?${queryString}`),
    initialSortBy: 'createdAt',
    initialSortDir: 'desc',
    toErrorMessage: toPromptHistoryErrorMessage,
  })

  return {
    listError: queryList.listError,
    loading: queryList.loading,
    page: queryList.page,
    pageSize: queryList.pageSize,
    prompts: queryList.items,
    reload: queryList.reload,
    search: queryList.search,
    setPage: queryList.setPage,
    setPageSize: queryList.setPageSize,
    setSearch: queryList.setSearch,
    setSort: queryList.setSort,
    sortBy: queryList.sortBy,
    sortDir: queryList.sortDir,
    total: queryList.total,
  }
}
