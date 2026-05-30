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

const effectiveModel = computed(
  () => props.model || firstUsableModel(props.provider),
)
const baseUrl = computed(() => props.provider.openaiBaseUrl ?? '')
const code = computed(
  () => `# Cline settings
API Provider: OpenAI Compatible
Base URL: ${baseUrl.value}
API Key: ${PROMPTGATE_TOKEN_PLACEHOLDER}
Model ID: ${effectiveModel.value}

# Cline CLI
cline auth -p openai -k ${PROMPTGATE_TOKEN_PLACEHOLDER} -b ${baseUrl.value} -m ${effectiveModel.value}`,
)

const subtitle = 'OpenAI-compatible provider for Cline.'
</script>

<template>
  <HelpSetupSnippetCard :code="code" :subtitle="subtitle" title="Cline" />
</template>
