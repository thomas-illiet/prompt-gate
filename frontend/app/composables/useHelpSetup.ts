import type { HelpSetupResponse } from '~/types/user-service'
import { toApiErrorMessage } from '~/utils/api-error'

export const HELP_SETUP_MIN_LOADING_MS = 350

// toHelpSetupErrorMessage converts setup guide API errors into user-facing text.
export function toHelpSetupErrorMessage(error: unknown) {
  return toApiErrorMessage(error, {}, 'Unexpected setup guide error.')
}

// wait delays setup loading state so the UI does not flicker.
function wait(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// useHelpSetup loads provider setup metadata for the help page.
export function useHelpSetup() {
  const apiFetch = useApiFetch()

  const setup = shallowRef<HelpSetupResponse | null>(null)
  const loading = shallowRef(false)
  const error = shallowRef<string | null>(null)

  // fetchSetup refreshes setup data while enforcing the minimum loading time.
  async function fetchSetup() {
    loading.value = true
    error.value = null
    const minimumLoading = wait(HELP_SETUP_MIN_LOADING_MS)
    let nextError: string | null = null

    try {
      setup.value = await apiFetch<HelpSetupResponse>('/api/v1/me/help/setup')
    } catch (fetchError) {
      nextError = toHelpSetupErrorMessage(fetchError)
    } finally {
      await minimumLoading
      error.value = nextError
      loading.value = false
    }
  }

  // reload exposes a stable setup refresh action.
  async function reload() {
    await fetchSetup()
  }

  void fetchSetup()

  return {
    error,
    loading,
    reload,
    setup,
  }
}
