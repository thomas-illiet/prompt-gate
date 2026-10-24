<script setup lang="ts">
import ModelLabChatComposer from './ModelLabChatComposer.vue'
import ModelLabChatHeader from './ModelLabChatHeader.vue'
import ModelLabConversation from './ModelLabConversation.vue'

import type { ModelLabTarget, ModelLabTurn } from '~/types/model-lab'

const props = defineProps<{
  draft: string
  draftError: string
  error: string | null
  maxAllowedOutputTokens: number
  maxOutputTokens: number
  maxPromptLength: number
  maxSelectedTargets: number
  modelsError: string | null
  modelsLoading: boolean
  running: boolean
  selectedTargets: ModelLabTarget[]
  targets: ModelLabTarget[]
  temperature: number
  turns: ModelLabTurn[]
}>()

const emit = defineEmits<{
  cancel: [laneId: string]
  copy: [text: string]
  newChat: []
  retry: [turnId: string, laneId: string]
  retryModels: []
  send: []
  stop: []
  'update:draft': [value: string]
  'update:max-output-tokens': [value: number]
  'update:selected-targets': [targets: ModelLabTarget[]]
  'update:temperature': [value: number]
}>()

const confirmNewChatOpen = shallowRef(false)
const composer = useTemplateRef<{ focus: () => void }>('composer')
const canSend = computed(
  () =>
    props.selectedTargets.length > 0 &&
    !props.running &&
    !props.draftError &&
    props.draft.trim().length > 0,
)
const terminalLaneSignature = computed(() =>
  props.turns
    .flatMap((turn) => [turn.primary, ...turn.comparisons])
    .filter((lane) => ['completed', 'error', 'stopped'].includes(lane.status))
    .map((lane) => `${lane.id}:${lane.status}`)
    .join('|'),
)

watch(terminalLaneSignature, async (current, previous) => {
  if (!current || current === previous) return
  const previousLanes = new Set(previous ? previous.split('|') : [])
  if (!current.split('|').some((lane) => !previousLanes.has(lane))) return
  await nextTick()
  composer.value?.focus()
})

function requestNewChat() {
  if (props.turns.length === 0) {
    emit('newChat')
    return
  }
  confirmNewChatOpen.value = true
}

function confirmNewChat() {
  confirmNewChatOpen.value = false
  emit('newChat')
}

async function useSuggestion(prompt: string) {
  emit('update:draft', prompt)
  await nextTick()
  composer.value?.focus()
}

function forwardRetry(turnId: string, laneId: string) {
  emit('retry', turnId, laneId)
}
</script>

<template>
  <AppSectionCard
    class="model-lab-chat-panel__card"
    icon="mdi-message-processing-outline"
    title="Model Lab"
    subtitle="Chat with one model, or select a second for side-by-side responses."
  >
    <div class="model-lab-chat-panel">
      <v-alert v-if="props.error" type="warning" variant="tonal" rounded="lg">
        {{ props.error }}
      </v-alert>

      <div
        v-if="props.modelsLoading"
        class="model-lab-chat-panel__models-state"
        role="status"
      >
        <v-progress-circular indeterminate color="primary" size="28" />
        <span>Loading your available models…</span>
      </div>

      <v-alert
        v-else-if="props.modelsError"
        type="warning"
        variant="tonal"
        rounded="lg"
      >
        <div class="model-lab-chat-panel__models-error">
          <span>{{ props.modelsError }}</span>
          <v-btn
            prepend-icon="mdi-refresh"
            size="small"
            variant="text"
            @click="emit('retryModels')"
          >
            Retry models
          </v-btn>
        </div>
      </v-alert>

      <AppEmptyState
        v-else-if="props.targets.length === 0"
        compact
        icon="mdi-lock-outline"
        title="No runnable model"
        text="Your groups do not currently grant access to a chat-compatible model."
      />

      <template v-else>
        <ModelLabChatHeader
          :disabled="props.running"
          :has-conversation="props.turns.length > 0"
          :max-allowed-output-tokens="props.maxAllowedOutputTokens"
          :max-output-tokens="props.maxOutputTokens"
          :max-selected-targets="props.maxSelectedTargets"
          :model-value="props.selectedTargets"
          :targets="props.targets"
          :temperature="props.temperature"
          @new-chat="requestNewChat"
          @update:max-output-tokens="emit('update:max-output-tokens', $event)"
          @update:model-value="emit('update:selected-targets', $event)"
          @update:temperature="emit('update:temperature', $event)"
        />

        <ModelLabConversation
          :selected-target-count="props.selectedTargets.length"
          :turns="props.turns"
          @cancel="emit('cancel', $event)"
          @copy="emit('copy', $event)"
          @retry="forwardRetry"
          @suggest="useSuggestion"
        />

        <ModelLabChatComposer
          ref="composer"
          :can-send="canSend"
          :error="props.draftError"
          :max-prompt-length="props.maxPromptLength"
          :model-value="props.draft"
          :running="props.running"
          @send="emit('send')"
          @stop="emit('stop')"
          @update:model-value="emit('update:draft', $event)"
        />
      </template>
    </div>

    <AppConfirmDialog
      v-model="confirmNewChatOpen"
      confirm-color="warning"
      confirm-label="Start new chat"
      icon="mdi-message-plus-outline"
      title="Start a new chat?"
      @confirm="confirmNewChat"
    >
      The current conversation and all model responses will be cleared from this
      tab. Your selected models and generation settings will be kept.
    </AppConfirmDialog>
  </AppSectionCard>
</template>

<style scoped>
.model-lab-chat-panel__card {
  display: flex;
  width: 100%;
  height: 100%;
  max-height: 100%;
  min-height: 0;
  flex: 1 1 auto;
  flex-direction: column;
  overflow: clip;
}

.model-lab-chat-panel__card :deep(.app-section-card__scroll) {
  display: flex;
  min-height: 0;
  flex: 1 1 auto;
  overflow: hidden;
}

.model-lab-chat-panel {
  display: flex;
  min-height: 0;
  flex: 1 1 auto;
  flex-direction: column;
  padding: 0 24px 24px;
}

.model-lab-chat-panel__models-state,
.model-lab-chat-panel__models-error {
  display: flex;
  align-items: center;
  gap: 12px;
}

.model-lab-chat-panel__models-state {
  min-height: 240px;
  justify-content: center;
  color: rgb(var(--app-shell-text-secondary));
}

.model-lab-chat-panel__models-error {
  justify-content: space-between;
}

@media (max-width: 720px) {
  .model-lab-chat-panel {
    padding: 0 12px 12px;
  }

  .model-lab-chat-panel__models-error {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
