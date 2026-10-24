import type { ModelLabCatalogResponse } from '~/types/model-lab'
import { toApiErrorMessage } from '~/utils/api-error'

const MODEL_LAB_CATALOG_ERRORS = {
  model_lab_catalog_unavailable:
    'The available models are temporarily unavailable. Try again shortly.',
} as const

export function toModelLabCatalogErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    MODEL_LAB_CATALOG_ERRORS,
    'The available models could not be loaded.',
  )
}

// useModelLabCatalog loads only the current user's group-filtered catalog.
export function useModelLabCatalog() {
  const apiFetch = useApiFetch()
  const catalog = shallowRef<ModelLabCatalogResponse | null>(null)
  const error = shallowRef<string | null>(null)
  const loading = shallowRef(false)
  let controller: AbortController | null = null

  async function reload() {
    controller?.abort()
    const requestController = new AbortController()
    controller = requestController
    loading.value = true
    error.value = null

    try {
      const response = await apiFetch<ModelLabCatalogResponse>(
        '/api/v1/me/models',
        { signal: requestController.signal },
      )
      if (controller === requestController) catalog.value = response
    } catch (loadError) {
      if (
        controller === requestController &&
        !(loadError instanceof DOMException && loadError.name === 'AbortError')
      ) {
        error.value = toModelLabCatalogErrorMessage(loadError)
      }
    } finally {
      if (controller === requestController) {
        controller = null
        loading.value = false
      }
    }
  }

  onScopeDispose(() => controller?.abort())
  void reload()

  return {
    catalog: shallowReadonly(catalog),
    error: shallowReadonly(error),
    loading: shallowReadonly(loading),
    reload,
  }
}
