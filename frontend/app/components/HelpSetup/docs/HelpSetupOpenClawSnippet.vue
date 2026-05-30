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
const primaryModel = computed(
  () => modelIds.value[0] ?? firstUsableModel(props.provider),
)
const openClawModels = computed(() =>
  modelIds.value
    .map(
      (model) => `          {
            id: "${model}",
            name: "${model}",
            input: ["text"],
          }`,
    )
    .join(',\n'),
)
const config = computed(
  () => `{
  env: {
    PROMPTGATE_TOKEN: "${PROMPTGATE_TOKEN_PLACEHOLDER}",
  },
  agents: {
    defaults: {
      model: { primary: "promptgate/${primaryModel.value}" },
    },
  },
  models: {
    mode: "merge",
    providers: {
      promptgate: {
        baseUrl: "${baseUrl.value}",
        apiKey: "\${PROMPTGATE_TOKEN}",
        api: "openai-completions",
        models: [
${openClawModels.value},
        ],
      },
    },
  },
}`,
)

const subtitle =
  'OpenClaw JSON5 config with a custom OpenAI-compatible provider.'
const filePaths = ['~/.openclaw/openclaw.json']
</script>

<template>
  <HelpSetupSnippetCard
    :file-paths="filePaths"
    :subtitle="subtitle"
    title="OpenClaw"
  >
    {{ config }}
  </HelpSetupSnippetCard>
</template>
