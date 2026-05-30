<script setup lang="ts">
import HelpSetupSnippetCard from '~/components/HelpSetup/HelpSetupSnippetCard.vue'
import type { HelpSetupProvider } from '~/types/user-service'
import {
  PROMPTGATE_TOKEN_PLACEHOLDER,
  firstUsableModel,
} from '~/utils/help-setup'

const props = defineProps<{
  model: string
  provider: HelpSetupProvider
}>()

const baseUrl = computed(() => props.provider.openaiBaseUrl ?? '')
const modelIds = computed(() =>
  props.provider.models.length > 0
    ? props.provider.models
    : [props.model || firstUsableModel(props.provider)],
)

// quoteYaml emits a JSON-style quoted scalar, which is valid YAML.
function quoteYaml(value: string) {
  return JSON.stringify(value)
}

const models = computed(() =>
  modelIds.value
    .map(
      (model) => `  - name: ${quoteYaml(model)}
    provider: openai
    model: ${quoteYaml(model)}
    apiKey: ${quoteYaml(PROMPTGATE_TOKEN_PLACEHOLDER)}
    apiBase: ${quoteYaml(baseUrl.value)}`,
    )
    .join('\n'),
)

const code = computed(
  () => `name: PromptGate
version: 0.0.1
schema: v1

models:
${models.value}`,
)

const subtitle = 'Continue config with the OpenAI-compatible provider.'
const filePaths = [
  '~/.continue/config.yaml',
  '%USERPROFILE%\\.continue\\config.yaml',
]
</script>

<template>
  <HelpSetupSnippetCard
    :code="code"
    :file-paths="filePaths"
    :subtitle="subtitle"
    title="Continue"
  />
</template>
