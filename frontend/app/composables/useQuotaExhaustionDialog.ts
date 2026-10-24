import { QUOTA_RESET_MAXIMUM } from '~/types/quota-resets'
import type { CurrentQuotaStatus } from '~/types/subscriptions'

export type QuotaWindowKey = '5h' | '7d'

export interface ExhaustedQuotaWindow {
  key: QuotaWindowKey
  label: string
  limit: number
  resetAt: string | null
  used: number
}

interface LoadQuotaOptions {
  preserveExisting?: boolean
}

// exhaustedQuotaWindows applies the same finite-window rule as the proxy.
export function exhaustedQuotaWindows(
  quota: CurrentQuotaStatus | null,
): ExhaustedQuotaWindow[] {
  if (!quota?.hasSubscription) {
    return []
  }

  const windows: Array<{
    key: QuotaWindowKey
    label: string
    limit: number | null
    resetAt: string | null
    used: number
  }> = [
    {
      key: '5h',
      label: '5-hour window',
      limit: quota.quota5hTokens,
      resetAt: quota.reset5hAt,
      used: quota.used5hTokens,
    },
    {
      key: '7d',
      label: '7-day window',
      limit: quota.quota7dTokens,
      resetAt: quota.reset7dAt,
      used: quota.used7dTokens,
    },
  ]

  return windows.flatMap((window) => {
    if (window.limit == null || window.used < window.limit) {
      return []
    }

    return [{ ...window, limit: window.limit }]
  })
}

// useQuotaExhaustionDialog coordinates the one-per-page-load quota warning.
export function useQuotaExhaustionDialog() {
  const apiFetch = useApiFetch()
  const quotaResets = useProfileQuotaResets({ immediate: false })
  const quota = shallowRef<CurrentQuotaStatus | null>(null)
  const quotaLoading = shallowRef(false)
  const dismissed = shallowRef(false)
  let quotaRequestVersion = 0

  const windows = computed(() => exhaustedQuotaWindows(quota.value))
  const isOpen = computed({
    get: () => !dismissed.value && windows.value.length > 0,
    set: (value: boolean) => {
      if (!value) {
        dismissed.value = true
      }
    },
  })
  const resetAvailable = computed(
    () => quotaResets.summary.value?.available ?? 0,
  )
  const resetMaximum = computed(
    () => quotaResets.summary.value?.maximum ?? QUOTA_RESET_MAXIMUM,
  )
  const resetPending = computed(
    () => quotaResets.summary.value?.pendingReset ?? false,
  )
  const canReset = computed(
    () =>
      windows.value.length > 0 &&
      resetAvailable.value > 0 &&
      Boolean(quotaResets.summary.value?.hasResettableQuota) &&
      !resetPending.value,
  )

  async function loadQuota(options: LoadQuotaOptions = {}) {
    const version = ++quotaRequestVersion
    quotaLoading.value = true

    try {
      const response = await apiFetch<CurrentQuotaStatus>('/api/v1/me/quota')
      if (version === quotaRequestVersion) {
        quota.value = response
        if (exhaustedQuotaWindows(response).length > 0) {
          void quotaResets.ensureLoaded()
        }
      }
      return response
    } catch {
      if (version === quotaRequestVersion && !options.preserveExisting) {
        quota.value = null
      }
      return null
    } finally {
      if (version === quotaRequestVersion) {
        quotaLoading.value = false
      }
    }
  }

  async function useReset() {
    if (!canReset.value || quotaResets.using.value) {
      return null
    }

    try {
      const response = await quotaResets.useReset()
      if (response?.status === 'applied') {
        quota.value = response.quota
      }
      return response
    } catch {
      // The shared reset composable reports a readable notification.
      return null
    }
  }

  watch(
    () => quotaResets.summary.value?.pendingReset ?? false,
    (pending, wasPending) => {
      if (!pending && wasPending && windows.value.length > 0) {
        void loadQuota({ preserveExisting: true })
      }
    },
  )

  void loadQuota()
  onScopeDispose(() => {
    quotaRequestVersion += 1
  }, true)

  return {
    canReset,
    isOpen,
    quotaLoading,
    resetAvailable,
    resetError: quotaResets.error,
    resetLoading: quotaResets.loading,
    resetMaximum,
    resetPending,
    resetUsing: quotaResets.using,
    useReset,
    windows,
  }
}
