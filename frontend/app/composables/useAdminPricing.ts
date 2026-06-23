import type {
  MissingModelPrice,
  ModelPricePayload,
  ModelPriceRecord,
  PriceRates,
  PricingCheckResponse,
  PricingConfigResponse,
} from '~/types/pricing'
import type { Provider, ProviderModelCatalog } from '~/types/providers'
import { toApiErrorMessage } from '~/utils/api-error'

const ERROR_MESSAGES = {
  invalid_price: 'Prices must be greater than or equal to zero.',
  invalid_price_target: 'Provider name and model are required.',
  immutable_price_target:
    'Provider name and model cannot be changed after creation.',
  pricing_conflict: 'A price already exists for this provider and model.',
  pricing_not_found: 'This price configuration no longer exists.',
  provider_not_found: 'Provider no longer exists.',
}

export function toAdminPricingErrorMessage(error: unknown) {
  return toApiErrorMessage(
    error,
    ERROR_MESSAGES,
    'Unexpected pricing configuration error.',
  )
}

function normalizeModelPrice(payload: ModelPricePayload): ModelPricePayload {
  return {
    providerName: payload.providerName.trim(),
    model: payload.model.trim(),
    input: Number(payload.input),
    output: Number(payload.output),
  }
}

export function useAdminPricing() {
  const apiFetch = useApiFetch()
  const apiJson = useApiJson()

  const fallback = shallowRef<PriceRates>({ input: 0, output: 0 })
  const models = shallowRef<ModelPriceRecord[]>([])
  const modelCatalog = shallowRef<ProviderModelCatalog[]>([])
  const providerOptions = shallowRef<Provider[]>([])
  const check = shallowRef<PricingCheckResponse | null>(null)
  const loading = shallowRef(false)
  const checking = shallowRef(false)
  const optionsLoading = shallowRef(false)
  const saving = shallowRef(false)
  const listError = shallowRef<string | null>(null)

  const configuredModelsCount = computed(() => models.value.length)
  const missingPricesCount = computed(
    () => check.value?.missingPrices.length ?? 0,
  )
  const providerErrorsCount = computed(
    () => check.value?.providerErrors.length ?? 0,
  )
  const isConfigured = computed(() => check.value?.configured ?? false)

  async function loadAllPages<T>(path: string, sortBy = 'name') {
    const items: T[] = []
    const pageSize = 100

    for (let page = 1; ; page += 1) {
      const params = new URLSearchParams({
        page: page.toString(),
        pageSize: pageSize.toString(),
        sortBy,
        sortDir: 'asc',
      })
      const response = await apiFetch<{ items: T[]; total: number }>(
        `${path}?${params}`,
      )
      items.push(...response.items)
      if (items.length >= response.total || response.items.length === 0) {
        return items
      }
    }
  }

  async function loadConfig() {
    loading.value = true
    listError.value = null
    try {
      const config = await apiFetch<PricingConfigResponse>(
        '/api/v1/admin/pricing',
      )
      fallback.value = { ...config.fallback }
      models.value = config.models
      return config
    } catch (error) {
      listError.value = toAdminPricingErrorMessage(error)
      throw error
    } finally {
      loading.value = false
    }
  }

  async function loadCheck() {
    checking.value = true
    try {
      check.value = await apiFetch<PricingCheckResponse>(
        '/api/v1/admin/pricing/check',
      )
      return check.value
    } catch (error) {
      listError.value = toAdminPricingErrorMessage(error)
      throw error
    } finally {
      checking.value = false
    }
  }

  async function loadPricingOptions() {
    optionsLoading.value = true
    try {
      const providers = await loadAllPages<Provider>('/api/v1/admin/providers')
      providerOptions.value = providers

      if (providers.length === 0) {
        modelCatalog.value = []
        return []
      }

      const params = new URLSearchParams()
      for (const provider of providers) {
        params.append('providerId', provider.id)
      }
      modelCatalog.value = await apiFetch<ProviderModelCatalog[]>(
        `/api/v1/admin/providers/model-catalog?${params.toString()}`,
      )
      return modelCatalog.value
    } catch (error) {
      listError.value = toAdminPricingErrorMessage(error)
      throw error
    } finally {
      optionsLoading.value = false
    }
  }

  async function reload() {
    await Promise.all([loadConfig(), loadCheck(), loadPricingOptions()])
  }

  async function saveFallback(payload: PriceRates) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Fallback pricing updated.',
        toErrorMessage: toAdminPricingErrorMessage,
      },
      async () => {
        const updated = await apiJson<PriceRates>(
          '/api/v1/admin/pricing/fallback',
          payload,
          { method: 'PATCH' },
        )
        fallback.value = updated
        return updated
      },
    )
  }

  async function createModelPrice(payload: ModelPricePayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Model price created.',
        toErrorMessage: toAdminPricingErrorMessage,
      },
      async () => {
        const created = await apiJson<ModelPriceRecord>(
          '/api/v1/admin/pricing/models',
          normalizeModelPrice(payload),
          { method: 'POST' },
        )
        await reload()
        return created
      },
    )
  }

  async function updateModelPrice(priceId: string, payload: ModelPricePayload) {
    return await runApiMutation(
      {
        loading: saving,
        successMessage: 'Model price updated.',
        toErrorMessage: toAdminPricingErrorMessage,
      },
      async () => {
        const updated = await apiJson<ModelPriceRecord>(
          `/api/v1/admin/pricing/models/${priceId}`,
          normalizeModelPrice(payload),
          { method: 'PATCH' },
        )
        await reload()
        return updated
      },
    )
  }

  async function deleteModelPrice(priceId: string) {
    await runApiMutation(
      {
        loading: saving,
        successMessage: 'Model price deleted.',
        toErrorMessage: toAdminPricingErrorMessage,
      },
      async () => {
        await apiFetch(`/api/v1/admin/pricing/models/${priceId}`, {
          method: 'DELETE',
        })
        await reload()
      },
    )
  }

  function priceFromMissing(missing: MissingModelPrice): ModelPricePayload {
    return {
      providerName: missing.providerName,
      model: missing.model,
      input: fallback.value.input,
      output: fallback.value.output,
    }
  }

  onMounted(() => {
    void reload()
  })

  return {
    check,
    checking,
    configuredModelsCount,
    createModelPrice,
    deleteModelPrice,
    fallback,
    isConfigured,
    listError,
    loadCheck,
    loadConfig,
    loadPricingOptions,
    loading,
    modelCatalog,
    missingPricesCount,
    models,
    optionsLoading,
    priceFromMissing,
    providerOptions,
    providerErrorsCount,
    reload,
    saveFallback,
    saving,
    updateModelPrice,
  }
}
