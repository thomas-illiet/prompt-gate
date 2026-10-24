<script setup lang="ts">
import ModelLabTurn from './ModelLabTurn.vue'

import type { ModelLabTurn as ChatTurn } from '~/types/model-lab'

const props = defineProps<{
  selectedTargetCount: number
  turns: ChatTurn[]
}>()

const emit = defineEmits<{
  cancel: [laneId: string]
  copy: [text: string]
  retry: [turnId: string, laneId: string]
  suggest: [prompt: string]
}>()

const suggestions = [
  'Explain this model in one concise paragraph.',
  'Draft a safe API integration checklist.',
  'Summarize the trade-offs of a difficult technical decision.',
]
const logElement = useTemplateRef<HTMLElement>('log')
const shouldAutoScroll = shallowRef(true)
const emptyDescription = computed(() => {
  if (props.selectedTargetCount === 0) {
    return 'Select at least one model above to start a conversation.'
  }
  if (props.selectedTargetCount === 2) {
    return 'Each message will run on both models and the responses will appear side by side.'
  }
  return 'Send a message for one focused response, or add a second model above whenever you want to compare.'
})
const streamSignature = computed(() =>
  props.turns
    .flatMap((turn) => [turn.primary, ...turn.comparisons])
    .map((lane) => `${lane.id}:${lane.status}:${lane.output.length}`)
    .join('|'),
)

function updateScrollIntent() {
  const element = logElement.value
  if (!element) return
  shouldAutoScroll.value =
    element.scrollHeight - element.scrollTop - element.clientHeight < 140
}

function forwardRetry(turnId: string, laneId: string) {
  emit('retry', turnId, laneId)
}

watch(streamSignature, async () => {
  if (!shouldAutoScroll.value) return
  await nextTick()
  const element = logElement.value
  if (element) element.scrollTop = element.scrollHeight
})
</script>

<template>
  <section
    ref="log"
    aria-label="Model Lab conversation"
    class="model-lab-conversation"
    role="log"
    @scroll.passive="updateScrollIntent"
  >
    <div v-if="props.turns.length === 0" class="model-lab-conversation__empty">
      <v-avatar color="primary" size="58" variant="tonal">
        <v-icon icon="mdi-message-processing-outline" size="32" />
      </v-avatar>
      <div>
        <h3>Start a governed conversation</h3>
        <p>{{ emptyDescription }}</p>
      </div>
      <div class="model-lab-conversation__suggestions">
        <v-btn
          v-for="suggestion in suggestions"
          :key="suggestion"
          color="primary"
          rounded="lg"
          size="small"
          variant="tonal"
          @click="emit('suggest', suggestion)"
        >
          {{ suggestion }}
        </v-btn>
      </div>
    </div>

    <div v-else class="model-lab-conversation__turns">
      <ModelLabTurn
        v-for="(turn, index) in props.turns"
        :key="turn.id"
        :turn="turn"
        :turn-number="index + 1"
        @cancel="emit('cancel', $event)"
        @copy="emit('copy', $event)"
        @retry="forwardRetry"
      />
    </div>
  </section>
</template>

<style scoped>
.model-lab-conversation {
  min-height: 0;
  flex: 1 1 auto;
  padding: 28px 24px;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  scroll-behavior: smooth;
}

.model-lab-conversation__empty {
  display: grid;
  min-height: 360px;
  place-items: center;
  align-content: center;
  gap: 18px;
  color: rgb(var(--app-shell-text-secondary));
  text-align: center;
}

.model-lab-conversation__empty h3,
.model-lab-conversation__empty p {
  margin: 0;
}

.model-lab-conversation__empty p {
  max-width: 560px;
  margin-top: 6px;
  color: rgb(var(--app-shell-text-muted));
  line-height: 1.6;
}

.model-lab-conversation__suggestions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  max-width: 760px;
}

.model-lab-conversation__suggestions :deep(.v-btn) {
  height: auto;
  min-height: 36px;
  padding-block: 8px;
  white-space: normal;
}

.model-lab-conversation__turns {
  display: grid;
  max-width: 1080px;
  gap: 34px;
  margin: 0 auto;
}

@media (max-width: 720px) {
  .model-lab-conversation {
    padding: 20px 14px;
  }

  .model-lab-conversation__empty {
    min-height: 300px;
  }

  .model-lab-conversation__suggestions {
    align-items: stretch;
    flex-direction: column;
    width: 100%;
  }

  .model-lab-conversation__suggestions :deep(.v-btn) {
    width: 100%;
    min-width: 0;
  }

  .model-lab-conversation__suggestions :deep(.v-btn__content) {
    min-width: 0;
    overflow-wrap: anywhere;
    white-space: normal;
  }
}
</style>
