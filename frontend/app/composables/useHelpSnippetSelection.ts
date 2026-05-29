import type { HelpSetupProvider } from '~/types/user-service'
import type { ComputedRef } from 'vue'
import {
  ANTHROPIC_MODEL_PLACEHOLDER,
  FALLBACK_MODEL_ID,
  firstUsableModel,
} from '~/utils/help-setup'

// useHelpSnippetSelection keeps provider and model snippet choices in sync.
export function useHelpSnippetSelection(
  providers: ComputedRef<HelpSetupProvider[]>,
) {
  const selectedProviderName = shallowRef('')
  const selectedModel = shallowRef('')

  const selectedProvider = computed<HelpSetupProvider | null>(
    () =>
      providers.value.find(
        (provider) => provider.name === selectedProviderName.value,
      ) ?? null,
  )

  const modelOptions = computed(() => {
    const provider = selectedProvider.value
    if (!provider) {
      return []
    }

    if (provider.type === 'anthropic') {
      return [ANTHROPIC_MODEL_PLACEHOLDER]
    }

    if (provider.models.length === 0) {
      return [selectedModel.value || FALLBACK_MODEL_ID]
    }

    return provider.models
  })

  watch(
    providers,
    (items) => {
      if (items.length === 0) {
        selectedProviderName.value = ''
        selectedModel.value = ''
        return
      }

      const stillExists = items.some(
        (provider) => provider.name === selectedProviderName.value,
      )
      if (!stillExists) {
        selectedProviderName.value = items[0]?.name ?? ''
      }
    },
    { immediate: true },
  )

  watch(
    selectedProvider,
    (provider) => {
      if (!provider) {
        selectedModel.value = ''
        return
      }

      if (provider.type === 'anthropic') {
        selectedModel.value = ANTHROPIC_MODEL_PLACEHOLDER
        return
      }

      if (!provider.models.includes(selectedModel.value)) {
        selectedModel.value = firstUsableModel(provider)
      }
    },
    { immediate: true },
  )

  return {
    modelOptions,
    selectedModel,
    selectedProvider,
    selectedProviderName,
  }
}
