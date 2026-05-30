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

const requestBody = computed(() =>
  JSON.stringify(
    {
      model: effectiveModel.value,
      messages: [
        {
          role: 'user',
          content: 'Hello from PromptGate',
        },
      ],
      stream: true,
    },
    null,
    2,
  ),
)

const subtitle = 'OpenAI-compatible chat completions request.'
</script>

<template>
  <HelpSetupSnippetCard :subtitle="subtitle" title="curl">
    curl {{ baseUrl }}/chat/completions \<br />
    -H "Authorization: Bearer {{ PROMPTGATE_TOKEN_PLACEHOLDER }}" \<br />
    -H "Content-Type: application/json" \<br />
    -d '{{ requestBody }}'
  </HelpSetupSnippetCard>
</template>
