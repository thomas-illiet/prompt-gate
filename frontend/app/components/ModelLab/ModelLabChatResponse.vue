<script setup lang="ts">
import ModelLabMarkdown from './ModelLabMarkdown.vue'

import type { ModelLabLane } from '~/types/model-lab'
import { modelLabTargetLabel } from '~/utils/model-lab'

const props = withDefaults(
  defineProps<{
    compact?: boolean
    lane: ModelLabLane
  }>(),
  { compact: false },
)

const emit = defineEmits<{
  cancel: []
  copy: [text: string]
  retry: []
}>()

const active = computed(
  () =>
    props.lane.status === 'authorizing' || props.lane.status === 'streaming',
)
const statusColor = computed(() => {
  if (props.lane.status === 'completed') return 'success'
  if (props.lane.status === 'error') return 'error'
  if (props.lane.status === 'stopped') return 'warning'
  if (props.lane.status === 'streaming') return 'primary'
  return 'grey'
})
const statusLabel = computed(() => {
  if (props.lane.status === 'authorizing') return 'Preparing secure ticket'
  if (props.lane.status === 'streaming') return 'Streaming'
  if (props.lane.status === 'completed') return 'Completed'
  if (props.lane.status === 'stopped') return 'Stopped locally'
  if (props.lane.status === 'error') return 'Failed'
  return 'Ready'
})
const metrics = computed(() => {
  const values: string[] = []
  if (props.lane.clientTtftMs !== null) {
    values.push(`TTFT ${props.lane.clientTtftMs} ms`)
  }
  if (props.lane.clientDurationMs !== null) {
    values.push(`${props.lane.clientDurationMs} ms total`)
  }
  if (props.lane.usage.outputTokens !== null) {
    values.push(
      `${props.lane.usage.outputTokens.toLocaleString()} output tokens`,
    )
  }
  return values.join(' · ')
})
</script>

<template>
  <article
    class="model-lab-chat-response"
    :class="{ 'model-lab-chat-response--compact': props.compact }"
    :aria-busy="active"
    :aria-label="`Response from ${modelLabTargetLabel(props.lane.target)}`"
  >
    <div class="model-lab-chat-response__header">
      <div class="model-lab-chat-response__identity">
        <v-avatar color="primary" size="30" variant="tonal">
          <v-icon icon="mdi-robot-outline" size="18" />
        </v-avatar>
        <div>
          <strong>{{ props.lane.target.model }}</strong>
          <span>{{ props.lane.target.providerDisplayName }}</span>
        </div>
      </div>

      <div class="model-lab-chat-response__status">
        <v-chip :color="statusColor" size="x-small" variant="tonal">
          {{ statusLabel }}
        </v-chip>
        <v-btn
          v-if="active"
          icon="mdi-stop"
          size="small"
          variant="text"
          :aria-label="`Stop ${props.lane.target.model}`"
          @click="emit('cancel')"
        />
        <v-btn
          v-if="props.lane.output"
          icon="mdi-content-copy"
          size="small"
          variant="text"
          :aria-label="`Copy output from ${props.lane.target.model}`"
          @click="emit('copy', props.lane.output)"
        />
        <v-btn
          v-if="
            props.lane.status === 'error' || props.lane.status === 'stopped'
          "
          icon="mdi-refresh"
          size="small"
          variant="text"
          :aria-label="`Retry ${props.lane.target.model}`"
          @click="emit('retry')"
        />
      </div>
    </div>

    <p class="model-lab-chat-response__announcement" aria-live="polite">
      {{ props.lane.target.model }}: {{ statusLabel }}
    </p>

    <v-alert
      v-if="props.lane.error"
      density="compact"
      type="error"
      variant="tonal"
    >
      {{ props.lane.error }}
    </v-alert>

    <ModelLabMarkdown
      v-if="props.lane.output"
      class="model-lab-chat-response__content"
      :content="props.lane.output"
    />
    <div v-else class="model-lab-chat-response__placeholder">
      <v-progress-circular
        v-if="active"
        color="primary"
        indeterminate
        size="24"
      />
      <span v-else>The response will appear here.</span>
    </div>

    <p v-if="metrics" class="model-lab-chat-response__metrics">
      Client measurement · {{ metrics }}
    </p>
  </article>
</template>

<style scoped>
.model-lab-chat-response {
  display: grid;
  min-width: 0;
  gap: 14px;
  padding: 18px;
  border: 1px solid rgba(var(--app-shell-border), 0.78);
  border-radius: var(--app-card-radius);
  background: rgb(var(--app-shell-surface-strong));
}

.model-lab-chat-response--compact {
  height: 100%;
  padding: 14px;
  box-shadow: var(--app-card-shadow-soft);
}

.model-lab-chat-response__header,
.model-lab-chat-response__identity,
.model-lab-chat-response__status {
  display: flex;
  align-items: center;
  gap: 9px;
}

.model-lab-chat-response__header {
  justify-content: space-between;
}

.model-lab-chat-response__identity {
  min-width: 0;
}

.model-lab-chat-response__identity > div:last-child {
  display: grid;
  min-width: 0;
}

.model-lab-chat-response__identity strong,
.model-lab-chat-response__identity span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-lab-chat-response__identity span,
.model-lab-chat-response__metrics {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.75rem;
}

.model-lab-chat-response__announcement {
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

.model-lab-chat-response__content {
  min-height: 54px;
  line-height: 1.65;
}

.model-lab-chat-response__placeholder {
  display: grid;
  min-height: 80px;
  place-items: center;
  color: rgb(var(--app-shell-text-muted));
}

.model-lab-chat-response__metrics {
  margin: 0;
}

@media (max-width: 520px) {
  .model-lab-chat-response__header {
    align-items: flex-start;
  }

  .model-lab-chat-response__status {
    justify-content: flex-end;
    flex-wrap: wrap;
  }
}
</style>
