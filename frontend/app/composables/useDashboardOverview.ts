import type { MaybeRefOrGetter } from 'vue'
import {
  computed,
  onScopeDispose,
  shallowRef,
  toValue,
  watch,
} from 'vue'

import type {
  DashboardCockpitOverviewResponse,
  DashboardScope,
  UsageWindow,
} from '~/types/user-service'
import { formatDateTime } from '~/utils/formatters'

interface DashboardOverviewCacheEntry {
  data: DashboardCockpitOverviewResponse
  updatedAt: string
}

const overviewCache = new Map<string, DashboardOverviewCacheEntry>()

function isAbortError(error: unknown) {
  return (
    error instanceof DOMException
      ? error.name === 'AbortError'
      : error instanceof Error && error.name === 'AbortError'
  )
}

function overviewEndpoint(scope: DashboardScope) {
  return scope === 'global'
    ? '/api/v1/admin/dashboard/overview'
    : '/api/v1/me/dashboard/overview'
}

// useDashboardOverview owns the keyed dashboard request lifecycle.
export function useDashboardOverview(
  scope: MaybeRefOrGetter<DashboardScope>,
  window: MaybeRefOrGetter<UsageWindow>,
) {
  const apiFetch = useApiFetch()
  const authStore = useAuthStore()
  const data = shallowRef<DashboardCockpitOverviewResponse | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const staleWarning = shallowRef<string | null>(null)
  const updatedAt = shallowRef<string | null>(null)
  const activeKey = computed(
    () =>
      `${authStore.user?.id ?? 'anonymous'}:${toValue(scope)}:${toValue(window)}`,
  )

  let controller: AbortController | null = null
  let requestVersion = 0

  async function load() {
    const requestedScope = toValue(scope)
    const requestedWindow = toValue(window)
    const requestedKey = `${authStore.user?.id ?? 'anonymous'}:${requestedScope}:${requestedWindow}`
    const version = ++requestVersion

    controller?.abort()
    controller = new AbortController()
    loading.value = true
    error.value = null
    staleWarning.value = null

    try {
      const response = await apiFetch<DashboardCockpitOverviewResponse>(
        `${overviewEndpoint(requestedScope)}?window=${requestedWindow}`,
        { signal: controller.signal },
      )
      if (version !== requestVersion || activeKey.value !== requestedKey) {
        return
      }

      const refreshedAt = new Date().toISOString()
      overviewCache.set(requestedKey, { data: response, updatedAt: refreshedAt })
      data.value = response
      updatedAt.value = refreshedAt
    } catch (fetchError) {
      if (version !== requestVersion || isAbortError(fetchError)) {
        return
      }

      const cached = overviewCache.get(requestedKey)
      if (activeKey.value === requestedKey && (data.value || cached)) {
        if (!data.value && cached) {
          data.value = cached.data
          updatedAt.value = cached.updatedAt
        }
        staleWarning.value = `Update failed — showing data from ${formatDateTime(updatedAt.value)}`
        return
      }

      if (activeKey.value === requestedKey) {
        error.value = toDashboardWidgetErrorMessage(fetchError)
      }
    } finally {
      if (version === requestVersion) {
        loading.value = false
      }
    }
  }

  watch(
    activeKey,
    (key) => {
      const cached = overviewCache.get(key)
      data.value = cached?.data ?? null
      updatedAt.value = cached?.updatedAt ?? null
      error.value = null
      staleWarning.value = null
      void load()
    },
    { immediate: true },
  )

  onScopeDispose(() => {
    requestVersion += 1
    controller?.abort()
  }, true)

  return {
    data,
    error,
    loading,
    reload: load,
    staleWarning,
    updatedAt,
  }
}

// clearDashboardOverviewCache is exported for deterministic component tests.
export function clearDashboardOverviewCache() {
  overviewCache.clear()
}
