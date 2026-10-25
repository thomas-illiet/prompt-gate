<script setup lang="ts">
import ModelLabGenerationSettingsDialog from './ModelLabGenerationSettingsDialog.vue'
import type { ModelLabRunSettings, ModelLabTarget } from '~/types/model-lab'
import { modelLabTargetKey, modelLabTargetLabel } from '~/utils/model-lab'

const props = defineProps<{
  disabled: boolean
  hasConversation: boolean
  maxAllowedOutputTokens: number
  maxOutputTokens: number
  maxSelectedTargets: number
  modelValue: ModelLabTarget[]
  targets: ModelLabTarget[]
  temperature: number
}>()

const emit = defineEmits<{
  newChat: []
  'update:max-output-tokens': [value: number]
  'update:model-value': [targets: ModelLabTarget[]]
  'update:temperature': [value: number]
}>()

const settingsDialogOpen = shallowRef(false)
const selectedTargetKeys = computed(() =>
  props.modelValue.map(modelLabTargetKey),
)
const selectedTargetKeySet = computed(() => new Set(selectedTargetKeys.value))
const targetItems = computed(() =>
  props.targets.map((target) => {
    const key = modelLabTargetKey(target)
    return {
      props: {
        disabled:
          selectedTargetKeys.value.length >= props.maxSelectedTargets &&
          !selectedTargetKeySet.value.has(key),
      },
      title: modelLabTargetLabel(target),
      value: key,
    }
  }),
)
const selectionHint = computed(() => {
  if (props.maxSelectedTargets === 1) {
    return 'This environment allows one model per message.'
  }
  if (props.modelValue.length === 0) {
    return 'Select at least one model. You can choose up to two.'
  }
  if (props.modelValue.length === 1) {
    return 'Primary model selected. Add one more to compare each new response.'
  }
  return 'The first model is primary. Both models run and consume quota for each new message.'
})

function updateSelection(keys: string[]) {
  const targetsByKey = new Map(
    props.targets.map((target) => [modelLabTargetKey(target), target]),
  )
  const selected: ModelLabTarget[] = []
  const seen = new Set<string>()
  for (const key of keys) {
    const target = targetsByKey.get(key)
    if (target && !seen.has(key)) {
      selected.push(target)
      seen.add(key)
    }
    if (selected.length === props.maxSelectedTargets) break
  }
  emit('update:model-value', selected)
}

function applySettings(settings: ModelLabRunSettings) {
  emit('update:max-output-tokens', settings.maxOutputTokens)
  emit('update:temperature', settings.temperature)
}
</script>

<template>
  <header class="model-lab-chat-header">
    <div class="model-lab-chat-header__main">
      <v-autocomplete
        class="model-lab-chat-header__model"
        chips
        closable-chips
        density="comfortable"
        :disabled="props.disabled"
        :hint="selectionHint"
        :items="targetItems"
        label="Models"
        :model-value="selectedTargetKeys"
        multiple
        no-data-text="No accessible model found"
        persistent-hint
        prepend-inner-icon="mdi-robot-outline"
        variant="outlined"
        @update:model-value="updateSelection"
      />

      <div class="model-lab-chat-header__actions">
        <v-btn
          color="primary"
          prepend-icon="mdi-tune-variant"
          rounded="xl"
          variant="outlined"
          :disabled="props.disabled"
          @click="settingsDialogOpen = true"
        >
          Generation Settings
        </v-btn>

        <v-btn
          color="primary"
          prepend-icon="mdi-message-plus-outline"
          rounded="xl"
          variant="tonal"
          :disabled="props.disabled && !props.hasConversation"
          @click="emit('newChat')"
        >
          New chat
        </v-btn>
      </div>
    </div>
  </header>

  <ModelLabGenerationSettingsDialog
    v-model="settingsDialogOpen"
    :max-allowed-output-tokens="props.maxAllowedOutputTokens"
    :max-output-tokens="props.maxOutputTokens"
    :temperature="props.temperature"
    @apply="applySettings"
  />
</template>

<style scoped>
.model-lab-chat-header {
  position: relative;
  display: grid;
  flex: 0 0 auto;
  gap: 12px;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    radial-gradient(
      circle at 92% 0,
      rgba(var(--app-accent-butter), 0.12),
      transparent 18rem
    ),
    rgba(var(--app-shell-surface-strong), 0.84);
}

.model-lab-chat-header::before {
  position: absolute;
  top: 0;
  left: 20px;
  width: 58px;
  height: 3px;
  border-radius: 0 0 999px 999px;
  background: linear-gradient(
    90deg,
    rgb(var(--v-theme-primary)),
    rgb(var(--app-accent-coral))
  );
  content: '';
}

.model-lab-chat-header__main,
.model-lab-chat-header__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.model-lab-chat-header__main {
  justify-content: space-between;
}

.model-lab-chat-header__model {
  max-width: 680px;
}

.model-lab-chat-header__model :deep(.v-field__input) {
  min-width: 0;
  flex-wrap: wrap;
}

.model-lab-chat-header__model :deep(.v-chip) {
  max-width: 100%;
}

.model-lab-chat-header__model :deep(.v-chip__content) {
  overflow: hidden;
  text-overflow: ellipsis;
}

@media (max-width: 960px) {
  .model-lab-chat-header {
    padding: 14px;
  }

  .model-lab-chat-header__main {
    align-items: stretch;
    flex-direction: column;
  }

  .model-lab-chat-header__model {
    max-width: none;
  }

  .model-lab-chat-header__actions > * {
    flex: 1 1 0;
  }
}

@media (max-width: 440px) {
  .model-lab-chat-header__actions {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
