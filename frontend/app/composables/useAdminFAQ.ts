import type { FAQEntry, FAQListResponse, FAQPayload } from '~/types/faq'
import { toApiErrorMessage } from '~/utils/api-error'

const FAQ_ERRORS = {
  answer_required: 'Answer is required.',
  faq_not_found: 'This FAQ entry no longer exists.',
  invalid_position: 'The requested position is invalid.',
  invalid_sort: 'The selected FAQ sort is invalid.',
  question_required: 'Question is required.',
  question_too_long: 'Question must contain at most 300 characters.',
}

function faqError(error: unknown) {
  return toApiErrorMessage(error, FAQ_ERRORS, 'Unexpected FAQ management error.')
}

export function useAdminFAQ() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()
  const saving = shallowRef(false)
  const previewing = shallowRef(false)
  const selectedEntry = shallowRef<FAQEntry | null>(null)

  const list = useQueryList<FAQEntry>({
    fetch: (query) => apiFetch<FAQListResponse>(`/api/v1/admin/faqs?${query}`),
    initialSortBy: 'position',
    initialSortDir: 'asc',
    toErrorMessage: faqError,
  })

  async function mutate<T>(message: string, operation: () => Promise<T>) {
    return await runApiMutation(
      { loading: saving, successMessage: message, toErrorMessage: faqError },
      operation,
    )
  }

  async function create(payload: FAQPayload) {
    return await mutate('FAQ entry created.', async () => {
      const entry = await apiJson<FAQEntry>('/api/v1/admin/faqs', payload, { method: 'POST' })
      await list.reload()
      return entry
    })
  }

  async function update(id: string, payload: FAQPayload) {
    return await mutate('FAQ entry updated.', async () => {
      const entry = await apiJson<FAQEntry>(`/api/v1/admin/faqs/${id}`, payload, { method: 'PATCH' })
      selectedEntry.value = entry
      await list.reload()
      return entry
    })
  }

  async function remove(id: string) {
    await mutate('FAQ entry deleted.', async () => {
      await apiFetch(`/api/v1/admin/faqs/${id}`, { method: 'DELETE' })
      await list.reload()
    })
  }

  async function move(entry: FAQEntry, position: number) {
    await mutate('FAQ order updated.', async () => {
      await apiJson(`/api/v1/admin/faqs/${entry.id}/position`, { position }, { method: 'PATCH' })
      await list.reload()
    })
  }

  async function preview(markdown: string) {
    previewing.value = true
    try {
      return await apiJson<{ renderedHtml: string }>('/api/v1/admin/faqs/preview', { markdown }, { method: 'POST' })
    } finally {
      previewing.value = false
    }
  }

  return {
    create,
    entries: list.items,
    listError: list.listError,
    loading: list.loading,
    move,
    page: list.page,
    pageSize: list.pageSize,
    preview,
    previewing,
    reload: list.reload,
    remove,
    saving,
    selectedEntry,
    setPage: list.setPage,
    setPageSize: list.setPageSize,
    setSort: list.setSort,
    sortBy: list.sortBy,
    sortDir: list.sortDir,
    total: list.total,
    update,
  }
}
