<script setup lang="ts">
import HelpSetupSnippetCard from '~/components/HelpSetup/HelpSetupSnippetCard.vue'
import type { HelpSetupProvider } from '~/types/user-service'
import { firstUsableModel } from '~/utils/help-setup'

const props = defineProps<{
  model: string
  provider: HelpSetupProvider
}>()

const modelIds = computed(() =>
  props.provider.models.length > 0
    ? props.provider.models
    : [props.model || firstUsableModel(props.provider)],
)
const openCodeModels = computed(() =>
  Object.fromEntries(modelIds.value.map((model) => [model, {}])),
)

const config = computed(() =>
  JSON.stringify(
    {
      $schema: 'https://opencode.ai/config.json',
      provider: {
        promptgate: {
          npm: '@ai-sdk/openai-compatible',
          options: {
            baseURL: props.provider.openaiBaseUrl ?? '',
            apiKey: '{env:PROMPTGATE_TOKEN}',
          },
          models: openCodeModels.value,
        },
      },
    },
    null,
    2,
  ),
)

const subtitle = 'OpenAI-compatible provider.'
const filePaths = ['~/.config/opencode/opencode.json', './opencode.json']
</script>

<template>
  <HelpSetupSnippetCard
    :file-paths="filePaths"
    :subtitle="subtitle"
    title="OpenCode"
  >
    {{ config }}
  </HelpSetupSnippetCard>
</template>
