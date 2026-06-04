import type { CurrentQuotaStatus } from '~/types/subscriptions'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  load_subscription_quota_failed: 'Unable to load subscription quota.',
}

export function toProfileQuotaErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected subscription quota error.',
  )
}

export function useProfileQuota() {
  const apiFetch = useApiFetch()
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const quota = shallowRef<CurrentQuotaStatus | null>(null)

  async function fetchQuota() {
    loading.value = true
    error.value = null
    try {
      quota.value = await apiFetch<CurrentQuotaStatus>('/api/v1/me/quota')
    } catch (fetchError) {
      error.value = toProfileQuotaErrorMessage(fetchError)
    } finally {
      loading.value = false
    }
  }

  async function reload() {
    await fetchQuota()
  }

  void fetchQuota()

  return {
    error,
    loading,
    quota,
    reload,
  }
}
