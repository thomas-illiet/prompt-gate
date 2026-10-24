<script setup lang="ts">
import ModelLabChatResponse from './ModelLabChatResponse.vue'
import ModelLabTurnComparison from './ModelLabTurnComparison.vue'

import type { ModelLabTurn } from '~/types/model-lab'

const props = defineProps<{
  turn: ModelLabTurn
  turnNumber: number
}>()

const emit = defineEmits<{
  cancel: [laneId: string]
  copy: [text: string]
  retry: [turnId: string, laneId: string]
}>()
</script>

<template>
  <article
    class="model-lab-turn"
    :aria-labelledby="`model-lab-turn-${props.turn.id}`"
  >
    <h3 :id="`model-lab-turn-${props.turn.id}`" class="model-lab-turn__sr-only">
      Conversation turn {{ props.turnNumber }}
    </h3>

    <div class="model-lab-turn__user">
      <div class="model-lab-turn__user-label">
        <span>You</span>
        <v-avatar color="secondary" size="30" variant="tonal">
          <v-icon icon="mdi-account-outline" size="18" />
        </v-avatar>
      </div>
      <p>{{ props.turn.prompt }}</p>
    </div>

    <v-alert
      v-if="props.turn.omittedTurns > 0"
      class="model-lab-turn__context-notice"
      density="compact"
      icon="mdi-history"
      type="info"
      variant="tonal"
    >
      {{ props.turn.omittedTurns }} older
      {{ props.turn.omittedTurns === 1 ? 'turn was' : 'turns were' }} omitted
      from this request to stay within the context limit.
    </v-alert>

    <ModelLabChatResponse
      v-if="props.turn.comparisons.length === 0"
      :lane="props.turn.primary"
      @cancel="emit('cancel', props.turn.primary.id)"
      @copy="emit('copy', $event)"
      @retry="emit('retry', props.turn.id, props.turn.primary.id)"
    />

    <ModelLabTurnComparison
      :comparisons="props.turn.comparisons"
      :primary="props.turn.primary"
      @cancel="emit('cancel', $event)"
      @copy="emit('copy', $event)"
      @retry="emit('retry', props.turn.id, $event)"
    />
  </article>
</template>

<style scoped>
.model-lab-turn {
  display: grid;
  gap: 16px;
}

.model-lab-turn__sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.model-lab-turn__user {
  justify-self: end;
  width: min(82%, 760px);
  padding: 14px 16px;
  border-radius: 20px 20px 5px 20px;
  background: rgba(var(--v-theme-primary), 0.13);
}

.model-lab-turn__user-label {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.78rem;
  font-weight: 700;
}

.model-lab-turn__user p {
  margin: 8px 0 0;
  overflow-wrap: anywhere;
  line-height: 1.6;
  white-space: pre-wrap;
}

.model-lab-turn__context-notice {
  justify-self: center;
  width: min(100%, 760px);
}

@media (max-width: 720px) {
  .model-lab-turn__user {
    width: 92%;
  }
}
</style>
