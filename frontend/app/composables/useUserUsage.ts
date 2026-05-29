import type { UsageDays, UserUsageSummary } from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_usage_window: 'Usage window must be 7 days, 30 days, or all time.',
}

// toUserUsageErrorMessage converts usage API errors into user-facing text.
export function toUserUsageErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected usage dashboard error.',
  )
}

// useUserUsage loads dashboard usage summaries for the selected time window.
export function useUserUsage() {
  const apiFetch = useApiFetch()

  const days = shallowRef<UsageDays>(30)
  const usage = shallowRef<UserUsageSummary | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)

  // fetchUsage loads usage for the currently selected window.
  async function fetchUsage() {
    loading.value = true
    error.value = null

    try {
      usage.value = await apiFetch<UserUsageSummary>(
        `/api/v1/me/usage?days=${days.value}`,
      )
    } catch (fetchError) {
      error.value = toUserUsageErrorMessage(fetchError)
    } finally {
      loading.value = false
    }
  }

  // setDays changes the usage window and triggers the watcher.
  function setDays(value: UsageDays) {
    days.value = value
  }

  // reload refreshes usage for the current window.
  async function reload() {
    await fetchUsage()
  }

  watch(days, async () => fetchUsage(), { immediate: true })

  return {
    days,
    error,
    loading,
    reload,
    setDays,
    usage,
  }
}
