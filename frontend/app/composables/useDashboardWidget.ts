import type { MaybeRefOrGetter } from 'vue'
import { computed, shallowRef, toValue, watch } from 'vue'
import type { UsageWindow } from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_usage_window: 'Usage window must be 7 days, 30 days, or all time.',
}

// toDashboardWidgetErrorMessage converts dashboard widget API errors into user-facing text.
export function toDashboardWidgetErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected dashboard widget error.',
  )
}

// useDashboardWidget loads one dashboard widget independently for progressive rendering.
export function useDashboardWidget<T>(
  endpoint: MaybeRefOrGetter<string>,
  window: MaybeRefOrGetter<UsageWindow>,
) {
  const apiFetch = useApiFetch()

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

  watch(url, () => load(), { immediate: true })

  return {
    data,
    error,
    loading,
    reload: load,
  }
}
