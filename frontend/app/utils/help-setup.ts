import type { HelpSetupProvider } from '~/types/user-service'

export const PROMPTGATE_TOKEN_PLACEHOLDER = 'YOUR_PROMPTGATE_TOKEN'
export const ANTHROPIC_MODEL_PLACEHOLDER = 'No model required'
export const FALLBACK_MODEL_ID = 'MODEL_ID'

// providerLabel returns the preferred display name for a setup provider.
export function providerLabel(provider: HelpSetupProvider) {
  return provider.displayName || provider.name
}

// providerHasModels returns whether a provider can produce setup snippets.
export function providerHasModels(provider: HelpSetupProvider) {
  return provider.models.length > 0
}

// availableSetupProviders keeps setup docs focused on providers with models.
export function availableSetupProviders(providers: HelpSetupProvider[]) {
  return providers.filter(providerHasModels)
}

// providerBaseUrl returns the provider-specific base URL shown in snippets.
export function providerBaseUrl(provider: HelpSetupProvider) {
  if (provider.type === 'anthropic') {
    return provider.anthropicBaseUrl ?? ''
  }

  return provider.openaiBaseUrl ?? ''
}

// firstUsableModel returns the first model or the snippet placeholder.
export function firstUsableModel(provider: HelpSetupProvider | null) {
  return provider?.models[0] ?? FALLBACK_MODEL_ID
}
