<script setup lang="ts">
import HelpSetupSnippetCard from '~/components/HelpSetup/HelpSetupSnippetCard.vue'
import type { HelpSetupProvider } from '~/types/user-service'
import { PROMPTGATE_TOKEN_PLACEHOLDER } from '~/utils/help-setup'

const props = defineProps<{
  model: string
  provider: HelpSetupProvider
}>()

const baseUrl = computed(() => props.provider.anthropicBaseUrl ?? '')
const subtitle = 'Point Claude Code to the PromptGate Anthropic route.'
</script>

<template>
  <HelpSetupSnippetCard :subtitle="subtitle" title="Claude Code">
    export ANTHROPIC_BASE_URL="{{ baseUrl }}"<br />
    export ANTHROPIC_API_KEY="{{ PROMPTGATE_TOKEN_PLACEHOLDER }}"<br />
    claude
  </HelpSetupSnippetCard>
</template>
