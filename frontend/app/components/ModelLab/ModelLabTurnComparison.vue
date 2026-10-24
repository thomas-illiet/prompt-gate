<script setup lang="ts">
import { useDisplay } from 'vuetify'

import ModelLabChatResponse from './ModelLabChatResponse.vue'

import type { ModelLabLane } from '~/types/model-lab'
import { modelLabTargetLabel } from '~/utils/model-lab'

const props = defineProps<{
  comparisons: ModelLabLane[]
  primary: ModelLabLane
}>()

const emit = defineEmits<{
  cancel: [laneId: string]
  copy: [text: string]
  retry: [laneId: string]
}>()

const { smAndDown } = useDisplay()
const selectedLaneId = shallowRef(props.primary.id)
const responses = computed(() => [props.primary, ...props.comparisons])
const selectedLane = computed(
  () =>
    responses.value.find((lane) => lane.id === selectedLaneId.value) ??
    props.primary,
)

watch(
  () => responses.value.map((lane) => lane.id),
  (ids) => {
    if (!ids.includes(selectedLaneId.value)) selectedLaneId.value = ids[0] ?? ''
  },
)
</script>

<template>
  <section
    v-if="props.comparisons.length > 0"
    class="model-lab-turn-comparison"
    aria-label="Response comparison"
  >
    <div class="model-lab-turn-comparison__heading">
      <div>
        <h4>Response comparison</h4>
        <p>
          Both models received the same conversation context. Each response
          consumes its own quota.
        </p>
      </div>
      <v-chip size="small" variant="tonal">
        {{ responses.length }} models
      </v-chip>
    </div>

    <template v-if="smAndDown">
      <v-tabs
        v-model="selectedLaneId"
        class="model-lab-turn-comparison__tabs"
        density="compact"
        show-arrows
      >
        <v-tab v-for="lane in responses" :key="lane.id" :value="lane.id">
          {{ modelLabTargetLabel(lane.target) }}
        </v-tab>
      </v-tabs>
      <ModelLabChatResponse
        :key="selectedLane.id"
        compact
        :lane="selectedLane"
        @cancel="emit('cancel', selectedLane.id)"
        @copy="emit('copy', $event)"
        @retry="emit('retry', selectedLane.id)"
      />
    </template>

    <div v-else class="model-lab-turn-comparison__grid">
      <ModelLabChatResponse
        v-for="lane in responses"
        :key="lane.id"
        compact
        :lane="lane"
        @cancel="emit('cancel', lane.id)"
        @copy="emit('copy', $event)"
        @retry="emit('retry', lane.id)"
      />
    </div>
  </section>
</template>

<style scoped>
.model-lab-turn-comparison {
  display: grid;
  gap: 16px;
  padding: 18px;
  border: 1px solid rgba(var(--app-shell-border), 0.72);
  border-radius: var(--app-card-radius);
  background: rgb(var(--v-theme-background));
}

.model-lab-turn-comparison__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.model-lab-turn-comparison__heading h4,
.model-lab-turn-comparison__heading p {
  margin: 0;
}

.model-lab-turn-comparison__heading p {
  margin-top: 3px;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
}

.model-lab-turn-comparison__heading :deep(.v-chip) {
  flex: 0 0 auto;
}

.model-lab-turn-comparison__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 280px), 1fr));
  gap: 14px;
}

.model-lab-turn-comparison__tabs {
  max-width: 100%;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.5);
}

@media (max-width: 520px) {
  .model-lab-turn-comparison {
    padding: 12px;
  }

  .model-lab-turn-comparison__heading {
    align-items: flex-start;
  }
}
</style>
