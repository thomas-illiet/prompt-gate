import type {
  ModelLabModel,
  ModelLabProvider,
  ModelLabTarget,
} from '~/types/model-lab'

export const MODEL_LAB_MAX_TARGETS = 3
export const MODEL_LAB_MAX_SELECTED_TARGETS = 2
export const MODEL_LAB_MAX_PROMPT_BYTES = 32_768

export function modelLabPromptByteLength(value: string) {
  return new TextEncoder().encode(value).byteLength
}

// modelLabTargetKey keeps identically named models from different providers distinct.
export function modelLabTargetKey(
  target: Pick<ModelLabTarget, 'model' | 'providerName'>,
) {
  return `${target.providerName}\u0000${target.model}`
}

// toModelLabTarget turns a user-visible catalog entry into an execution target.
export function toModelLabTarget(
  provider: ModelLabProvider,
  model: ModelLabModel,
): ModelLabTarget {
  return {
    model: model.id,
    providerDisplayName: provider.displayName || provider.name,
    providerName: provider.name,
    providerType: provider.type,
    protocol: model.playground.protocol,
  }
}

// flattenModelLabCatalog returns only models that the backend marked runnable.
export function flattenModelLabCatalog(providers: ModelLabProvider[]) {
  return providers.flatMap((provider) =>
    provider.models
      .filter((model) => model.playground.supported)
      .map((model) => toModelLabTarget(provider, model)),
  )
}

// modelLabTargetLabel makes provider ownership explicit in every selector.
export function modelLabTargetLabel(target: ModelLabTarget) {
  return `${target.providerDisplayName} · ${target.model}`
}

export function formatModelLabPrice(value: number | null | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 'Unknown'
  }

  return new Intl.NumberFormat('en-US', {
    maximumFractionDigits: 6,
  }).format(value)
}
