<script setup lang="ts">
import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '~/types/user-service'
import {
  formatDateTime,
  formatDurationMs,
  formatNumber,
} from '~/utils/formatters'

type PromptHistoryGraphItem = PromptHistoryItem | AdminPromptHistoryItem

const props = defineProps<{
  actorLabel: string
  prompt: PromptHistoryGraphItem | null
}>()

const isOpen = defineModel<boolean>({ default: false })

const subtitle = computed(() => {
  if (!props.prompt) {
    return 'Prompt request details.'
  }

  return `${props.prompt.provider} / ${props.prompt.model}`
})

const summaryItems = computed(() => {
  const prompt = props.prompt
  if (!prompt) {
    return []
  }

  const items = [
    { label: 'Requester', value: props.actorLabel || 'You' },
    { label: 'Provider', value: prompt.provider },
    { label: 'Model', value: prompt.model },
    { label: 'Input tokens', value: formatNumber(prompt.inputTokens) },
    { label: 'Output tokens', value: formatNumber(prompt.outputTokens) },
    { label: 'Total tokens', value: formatNumber(prompt.totalTokens) },
    { label: 'Duration', value: formatDurationMs(prompt.durationMs) },
    { label: 'Created', value: formatDateTime(prompt.createdAt) },
  ]

  const clientIP = clientIpLabel(prompt)
  if (clientIP) {
    items.splice(1, 0, { label: 'Client IP', value: clientIP })
  }

  return items
})

function clientIpLabel(prompt: PromptHistoryGraphItem) {
  if (!('clientIp' in prompt)) {
    return ''
  }

  return prompt.clientIp.trim() || 'IP unknown'
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-transit-connection-variant"
    max-width="980"
    :subtitle="subtitle"
    title="Request graph"
  >
    <div v-if="props.prompt" class="prompt-history-request-dialog">
      <PromptHistoryRequestGraph
        :actor-label="props.actorLabel"
        :prompt="props.prompt"
      />

      <section class="prompt-history-request-dialog__section">
        <h2 class="prompt-history-request-dialog__section-title">
          Request summary
        </h2>

        <dl class="prompt-history-request-dialog__grid">
          <div
            v-for="item in summaryItems"
            :key="item.label"
            class="prompt-history-request-dialog__item"
          >
            <dt>{{ item.label }}</dt>
            <dd>{{ item.value }}</dd>
          </div>
        </dl>
      </section>

      <section class="prompt-history-request-dialog__section">
        <h2 class="prompt-history-request-dialog__section-title">Prompt</h2>
        <p class="prompt-history-request-dialog__prompt">
          {{ props.prompt.prompt }}
        </p>
      </section>
    </div>

    <template #actions>
      <v-spacer />
      <AppDialogCloseButton @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>

<style scoped>
.prompt-history-request-dialog {
  display: grid;
  gap: 16px;
}

.prompt-history-request-dialog__section {
  display: grid;
  gap: 12px;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.5);
  border-radius: 8px;
}

.prompt-history-request-dialog__section-title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
}

.prompt-history-request-dialog__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin: 0;
}

.prompt-history-request-dialog__item {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.prompt-history-request-dialog__item dt {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
}

.prompt-history-request-dialog__item dd {
  min-width: 0;
  margin: 0;
  color: rgb(var(--app-shell-text-primary));
  font-weight: 700;
  overflow-wrap: anywhere;
}

.prompt-history-request-dialog__prompt {
  min-width: 0;
  max-height: 220px;
  margin: 0;
  overflow: auto;
  color: rgb(var(--app-shell-text-primary));
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

@media (max-width: 900px) {
  .prompt-history-request-dialog__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .prompt-history-request-dialog__grid {
    grid-template-columns: 1fr;
  }
}
</style>
