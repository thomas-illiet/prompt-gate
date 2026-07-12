import type { PublicFAQEntry } from '~/types/faq'
import { toApiErrorMessage } from '~/utils/api-error'

export function useFAQ() {
  const apiFetch = useApiFetch()
  const entries = shallowRef<PublicFAQEntry[]>([])
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)

  async function load() {
    loading.value = true
    error.value = null
    try {
      entries.value = await apiFetch<PublicFAQEntry[]>('/api/v1/faq')
    } catch (cause) {
      error.value = toApiErrorMessage(cause, {}, 'Unable to load the FAQ.')
    } finally {
      loading.value = false
    }
  }

  onMounted(load)
  return { entries, error, load, loading }
}
