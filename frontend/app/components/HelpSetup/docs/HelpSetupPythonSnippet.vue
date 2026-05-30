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
  () => `from openai import OpenAI

client = OpenAI(
    api_key="${PROMPTGATE_TOKEN_PLACEHOLDER}",
    base_url="${baseUrl.value}",
)

completion = client.chat.completions.create(
    model="${effectiveModel.value}",
    messages=[
        {"role": "user", "content": "Hello from PromptGate"},
    ],
)

print(completion.choices[0].message.content)`,
)

const subtitle = 'Official OpenAI Python SDK.'
</script>

<template>
  <HelpSetupSnippetCard :code="code" :subtitle="subtitle" title="Python" />
</template>
