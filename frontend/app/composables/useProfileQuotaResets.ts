import type {
  QuotaResetHistoryItem,
  QuotaResetHistoryResponse,
  QuotaResetHistorySortBy,
  QuotaResetSummary,
  UseQuotaResetResponse,
} from '~/types/quota-resets'
import { Notify } from '~/stores/notification'
import { apiErrorCode, toApiErrorMessage } from '~/utils/api-error'
import { createIdempotencyKey } from '~/utils/idempotency'
import { withApiQuery } from '~/utils/api-paths'

const quotaResetPath = '/api/v1/me/quota-resets'
const pendingPollIntervalMs = 5_000

interface UseProfileQuotaResetsOptions {
  immediate?: boolean
}

const ERROR_MESSAGES = {
  credit_unavailable: 'No quota reset is currently available.',
  invalid_sort: 'Selected quota reset history sort is invalid.',
  load_quota_reset_history_failed: 'Unable to load quota reset history.',
  load_quota_resets_failed: 'Unable to load quota resets.',
  missing_idempotency_key: 'Unable to safely submit the reset. Try again.',
  quota_reset_credit_unavailable: 'No quota reset is currently available.',
  quota_reset_not_applicable: 'Your current quota cannot be reset.',
  quota_not_exhausted: 'Your current quota cannot be reset.',
  quota_not_resettable: 'Your current plan has no finite quota to reset.',
  quota_reset_pending: 'A quota reset is already pending.',
  subscription_quota_unavailable:
    'Quota information is temporarily unavailable. Try again shortly.',
}

export function toProfileQuotaResetErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected quota reset error.',
  )
}

function createProfileQuotaResets() {
  const apiFetch = useApiFetch()
  const summary = shallowRef<QuotaResetSummary | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const using = shallowRef(false)
  const historyOpen = shallowRef(false)
  const history = shallowRef<QuotaResetHistoryItem[]>([])
  const historyLoading = shallowRef(false)
  const historyError = shallowRef<string | null>(null)
  const historyPage = shallowRef(1)
  const historyPageSize = shallowRef(20)
  const historySortBy = shallowRef<QuotaResetHistorySortBy>('createdAt')
  const historySortDir = shallowRef<'asc' | 'desc'>('desc')
  const historyTotal = shallowRef(0)

  let summaryRequestVersion = 0
  let historyRequestVersion = 0
  let pendingPollTimer: ReturnType<typeof setTimeout> | null = null
  let pendingPollInFlight = false
  let pendingUseIdempotencyKey: string | null = null
  let initialSummaryPromise: Promise<QuotaResetSummary | null> | null = null
  let disposed = false

  function stopPendingPoll() {
    if (pendingPollTimer) {
      clearTimeout(pendingPollTimer)
      pendingPollTimer = null
    }
  }

  function schedulePendingPoll() {
    if (disposed || pendingPollTimer || !summary.value?.pendingReset) {
      return
    }

    pendingPollTimer = setTimeout(async () => {
      pendingPollTimer = null
      if (pendingPollInFlight || !summary.value?.pendingReset) {
        schedulePendingPoll()
        return
      }

      pendingPollInFlight = true
      try {
        await loadSummary({ background: true })
      } finally {
        pendingPollInFlight = false
        schedulePendingPoll()
      }
    }, pendingPollIntervalMs)
  }

  function applySummary(nextSummary: QuotaResetSummary) {
    const wasPending = summary.value?.pendingReset ?? false
    summary.value = nextSummary

    if (nextSummary.pendingReset) {
      schedulePendingPoll()
      return
    }

    stopPendingPoll()
    if (wasPending && historyOpen.value) {
      void loadHistory()
    }
  }

  async function loadSummary(options: { background?: boolean } = {}) {
    const version = ++summaryRequestVersion
    if (!options.background) {
      loading.value = true
      error.value = null
    }

    try {
      const response = await apiFetch<QuotaResetSummary>(quotaResetPath)
      if (version === summaryRequestVersion) {
        applySummary(response)
      }
      return response
    } catch (fetchError) {
      if (!options.background && version === summaryRequestVersion) {
        error.value = toProfileQuotaResetErrorMessage(fetchError)
      }
      return null
    } finally {
      if (!options.background && version === summaryRequestVersion) {
        loading.value = false
      }
    }
  }

  // ensureLoaded shares the first summary request between simultaneous consumers.
  function ensureLoaded() {
    if (summary.value) {
      return Promise.resolve(summary.value)
    }
    if (!initialSummaryPromise) {
      initialSummaryPromise = loadSummary().finally(() => {
        initialSummaryPromise = null
      })
    }
    return initialSummaryPromise
  }

  async function loadHistory() {
    const version = ++historyRequestVersion
    historyLoading.value = true
    historyError.value = null

    const params = new URLSearchParams({
      page: historyPage.value.toString(),
      pageSize: historyPageSize.value.toString(),
      sortBy: historySortBy.value,
      sortDir: historySortDir.value,
    })

    try {
      const response = await apiFetch<QuotaResetHistoryResponse>(
        withApiQuery(`${quotaResetPath}/history`, params),
      )
      if (version === historyRequestVersion) {
        history.value = response.items
        historyTotal.value = response.total
      }
      return response
    } catch (fetchError) {
      if (version === historyRequestVersion) {
        historyError.value = toProfileQuotaResetErrorMessage(fetchError)
      }
      return null
    } finally {
      if (version === historyRequestVersion) {
        historyLoading.value = false
      }
    }
  }

  async function openHistory() {
    historyOpen.value = true
    await loadHistory()
  }

  async function setHistoryPage(value: number) {
    historyPage.value = value
    await loadHistory()
  }

  async function setHistoryPageSize(value: number) {
    historyPageSize.value = value
    historyPage.value = 1
    await loadHistory()
  }

  async function setHistorySort(
    sortBy: QuotaResetHistorySortBy,
    sortDir: 'asc' | 'desc',
  ) {
    historySortBy.value = sortBy
    historySortDir.value = sortDir
    historyPage.value = 1
    await loadHistory()
  }

  async function useReset() {
    if (using.value) {
      return null
    }

    pendingUseIdempotencyKey ??= createIdempotencyKey()
    const idempotencyKey = pendingUseIdempotencyKey

    try {
      return await runApiMutation(
        {
          loading: using,
          toErrorMessage: toProfileQuotaResetErrorMessage,
        },
        async () => {
          const response = await apiFetch<UseQuotaResetResponse>(
            `${quotaResetPath}/use`,
            {
              method: 'POST',
              headers: { 'Idempotency-Key': idempotencyKey },
            },
          )

          pendingUseIdempotencyKey = null
          summaryRequestVersion += 1
          applySummary(response.summary)

          if (response.status === 'pending') {
            Notify.success('Quota reset queued. It will be applied shortly.')
          } else {
            Notify.success('Quota reset applied. You can resume requests.')
            if (historyOpen.value) {
              await loadHistory()
            }
          }

          return response
        },
      )
    } catch (mutationError) {
      if (apiErrorCode(mutationError) === 'quota_reset_pending') {
        pendingUseIdempotencyKey = null
        await loadSummary({ background: true })
      }
      throw mutationError
    }
  }

  onScopeDispose(() => {
    disposed = true
    stopPendingPoll()
  }, true)

  return {
    error,
    history,
    historyError,
    historyLoading,
    historyOpen,
    historyPage,
    historyPageSize,
    historySortBy,
    historySortDir,
    historyTotal,
    ensureLoaded,
    loadHistory,
    loading,
    openHistory,
    reload: loadSummary,
    setHistoryPage,
    setHistoryPageSize,
    setHistorySort,
    summary,
    useReset,
    using,
  }
}

const useSharedProfileQuotaResets = createSharedComposable(
  createProfileQuotaResets,
)

// useProfileQuotaResets exposes one app-scoped reset wallet to every consumer.
export function useProfileQuotaResets(
  options: UseProfileQuotaResetsOptions = {},
) {
  const state = useSharedProfileQuotaResets()
  if (options.immediate !== false) {
    void state.ensureLoaded()
  }
  return state
}
