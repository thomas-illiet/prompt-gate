import type { HelpSetupProvider } from '~/types/user-service'
import type { SetupGuide } from '~/types/setup-guides'

export const SETUP_GUIDE_VARIABLES = [
  'token',
  'baseUrl',
  'openaiBaseUrl',
  'anthropicBaseUrl',
  'model',
  'models',
  'providerName',
  'providerDisplayName',
] as const

export interface SetupGuideTemplateContext {
  token: string
  baseUrl: string
  openaiBaseUrl: string
  anthropicBaseUrl: string
  model: string
  models: string[]
  providerName: string
  providerDisplayName: string
}

const tagPattern = /{{\s*([#/]?)([a-zA-Z][a-zA-Z0-9]*)\s*}}/g
const modelsSectionPattern = /{{\s*#models\s*}}([\s\S]*?){{\s*\/models\s*}}/g

export function validateSetupGuideTemplate(template: string): string | null {
  if (!template.trim()) return 'Template is required.'
  const allowed = new Set<string>(SETUP_GUIDE_VARIABLES)
  const stack: string[] = []
  for (const match of template.matchAll(tagPattern)) {
    const [, kind, name = ''] = match
    if (!allowed.has(name)) return `Unknown template variable "${name}".`
    if (kind === '#') {
      if (name !== 'models') return 'Only models can be used as a section.'
      stack.push(name)
    } else if (kind === '/') {
      if (stack.pop() !== name) return `Unmatched closing section "${name}".`
    }
  }
  if (stack.length) return `Unclosed section "${stack.at(-1)}".`
  if (template.replace(tagPattern, '').includes('{{'))
    return 'Malformed template tag.'
  return null
}

export function renderSetupGuideTemplate(
  template: string,
  context: SetupGuideTemplateContext,
) {
  const withModels = template.replace(
    modelsSectionPattern,
    (_match, body: string) =>
      context.models
        .map((model) => replaceVariables(body, { ...context, model }))
        .join(''),
  )
  return replaceVariables(withModels, context)
}

function replaceVariables(
  template: string,
  context: SetupGuideTemplateContext,
) {
  return template.replace(
    tagPattern,
    (match, kind: string, name: keyof SetupGuideTemplateContext) => {
      if (kind) return match
      const value = context[name]
      return Array.isArray(value) ? value.join(', ') : String(value ?? '')
    },
  )
}

export function setupGuideContext(
  provider: HelpSetupProvider,
  selectedModel: string,
): SetupGuideTemplateContext {
  const models = provider.models.length
    ? provider.models
    : [selectedModel].filter(Boolean)
  const model = selectedModel || models[0] || 'model-id'
  const openaiBaseUrl = provider.openaiBaseUrl ?? ''
  const anthropicBaseUrl = provider.anthropicBaseUrl ?? ''
  return {
    token: '<PROMPTGATE_TOKEN>',
    baseUrl: openaiBaseUrl || anthropicBaseUrl,
    openaiBaseUrl,
    anthropicBaseUrl,
    model,
    models,
    providerName: provider.name,
    providerDisplayName: provider.displayName,
  }
}

export function guideSupportsProvider(
  guide: SetupGuide,
  provider: HelpSetupProvider,
) {
  if (guide.compatibility === 'both') return true
  return (
    guide.compatibility ===
    (provider.type === 'anthropic' ? 'anthropic' : 'openai')
  )
}
