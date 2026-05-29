import { Notify } from '~/stores/notification'
import type { Ref } from 'vue'

interface ApiMutationOptions {
  loading: Ref<boolean>
  successMessage?: string
  toErrorMessage: (error: unknown) => string
}

// runApiMutation wraps write operations with loading state and notifications.
export async function runApiMutation<T>(
  options: ApiMutationOptions,
  action: () => Promise<T>,
) {
  options.loading.value = true

  try {
    const result = await action()
    if (options.successMessage) {
      Notify.success(options.successMessage)
    }
    return result
  } catch (error) {
    Notify.error(options.toErrorMessage(error))
    throw error
  } finally {
    options.loading.value = false
  }
}
