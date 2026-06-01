import type { InjectionKey, MaybeRefOrGetter, Ref } from 'vue'
import {
  computed,
  hasInjectionContext,
  inject,
  provide,
  readonly,
  shallowRef,
  toValue,
  watch,
} from 'vue'
import type { UsageWindow } from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_usage_window: 'Usage window must be 7 days, 30 days, or all time.',
}
const DASHBOARD_REFRESH_KEY: InjectionKey<Ref<number>> = Symbol(
  'dashboard-refresh-version',
)

// toDashboardWidgetErrorMessage converts dashboard widget API errors into user-facing text.
export function toDashboardWidgetErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected dashboard widget error.',
  )
}

// useDashboardRefresh exposes a shared refresh signal for every dashboard widget.
export function useDashboardRefresh() {
  const version = shallowRef(0)
  provide(DASHBOARD_REFRESH_KEY, version)

  function refresh() {
    version.value += 1
  }

  return {
    refresh,
    version: readonly(version),
  }
}

function useDashboardRefreshVersion() {
  if (!hasInjectionContext()) {
    return shallowRef(0)
  }

  return inject(DASHBOARD_REFRESH_KEY, shallowRef(0))
}

// useDashboardWidget loads one dashboard widget independently for progressive rendering.
export function useDashboardWidget<T>(
  endpoint: MaybeRefOrGetter<string>,
  window: MaybeRefOrGetter<UsageWindow>,
) {
  const apiFetch = useApiFetch()
  const dashboardRefreshVersion = useDashboardRefreshVersion()

  const data = shallowRef<T | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const requestId = shallowRef(0)
  const url = computed(() => `${toValue(endpoint)}?window=${toValue(window)}`)

  async function load() {
    const currentRequestId = requestId.value + 1
    requestId.value = currentRequestId
    loading.value = true
    error.value = null

    try {
      const response = await apiFetch<T>(url.value)
      if (requestId.value === currentRequestId) {
        data.value = response
      }
    } catch (fetchError) {
      if (requestId.value === currentRequestId) {
        error.value = toDashboardWidgetErrorMessage(fetchError)
      }
    } finally {
      if (requestId.value === currentRequestId) {
        loading.value = false
      }
    }
  }

  watch([url, dashboardRefreshVersion], () => load(), { immediate: true })

  return {
    data,
    error,
    loading,
    reload: load,
  }
}
