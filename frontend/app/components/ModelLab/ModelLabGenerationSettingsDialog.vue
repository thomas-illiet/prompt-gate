<script setup lang="ts">
import type { ModelLabRunSettings } from '~/types/model-lab'

const props = defineProps<{
  maxAllowedOutputTokens: number
  maxOutputTokens: number
  temperature: number
}>()

const emit = defineEmits<{
  apply: [settings: ModelLabRunSettings]
}>()

const isOpen = defineModel<boolean>({ default: false })
const formId = useId()
const temperatureInputId = useId()
const draftMaxOutputTokens = shallowRef(props.maxOutputTokens)
const draftTemperature = shallowRef(props.temperature)

watch(isOpen, (open) => {
  if (!open) return
  draftMaxOutputTokens.value = props.maxOutputTokens
  draftTemperature.value = props.temperature
})

function updateMaxOutputTokens(value: unknown) {
  draftMaxOutputTokens.value = Number(value)
}

function updateTemperature(value: unknown) {
  draftTemperature.value = Number(value)
}

function close() {
  isOpen.value = false
}

function apply() {
  const temperature = Number.isFinite(draftTemperature.value)
    ? Math.min(1, Math.max(0, draftTemperature.value))
    : props.temperature
  const maxOutputTokens = Number.isFinite(draftMaxOutputTokens.value)
    ? Math.round(
        Math.min(
          props.maxAllowedOutputTokens,
          Math.max(1, draftMaxOutputTokens.value),
        ),
      )
    : props.maxOutputTokens

  emit('apply', { maxOutputTokens, temperature })
  isOpen.value = false
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-tune-variant"
    max-width="620"
    subtitle="Adjust how the selected model generates its next response."
    title="Generation settings"
  >
    <form
      :id="formId"
      class="model-lab-generation-settings-dialog"
      @submit.prevent="apply"
    >
      <div class="model-lab-generation-settings-dialog__field">
        <div class="model-lab-generation-settings-dialog__label-row">
          <label class="text-subtitle-2" :for="temperatureInputId">
            Temperature
          </label>
          <span class="model-lab-generation-settings-dialog__value">
            {{ draftTemperature.toFixed(1) }}
          </span>
        </div>
        <v-slider
          :id="temperatureInputId"
          aria-label="Temperature"
          hide-details
          :max="1"
          :min="0"
          :model-value="draftTemperature"
          name="Temperature"
          :step="0.1"
          thumb-label
          @update:model-value="updateTemperature"
        />
        <p class="model-lab-generation-settings-dialog__hint">
          Lower values are more predictable; higher values are more varied.
        </p>
      </div>

      <v-number-input
        control-variant="stacked"
        hide-details="auto"
        label="Maximum output tokens"
        :max="props.maxAllowedOutputTokens"
        :min="1"
        :model-value="draftMaxOutputTokens"
        :step="128"
        variant="outlined"
        @update:model-value="updateMaxOutputTokens"
      />
    </form>

    <template #actions>
      <AppDialogCloseButton label="Cancel" @click="close" />
      <AppDialogActionButton
        :form="formId"
        label="Apply settings"
        prepend-icon="mdi-check"
        type="submit"
      />
    </template>
  </AppDialogCard>
</template>

<style scoped>
.model-lab-generation-settings-dialog {
  display: grid;
  gap: 24px;
  padding-top: 4px;
}

.model-lab-generation-settings-dialog__field {
  display: grid;
  gap: 6px;
}

.model-lab-generation-settings-dialog__label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.model-lab-generation-settings-dialog__value {
  min-width: 42px;
  padding: 3px 8px;
  border-radius: 999px;
  background: rgba(var(--v-theme-primary), 0.1);
  color: rgb(var(--v-theme-primary));
  font-size: 0.8rem;
  font-weight: 700;
  text-align: center;
}

.model-lab-generation-settings-dialog__hint {
  margin: 0;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
  line-height: 1.45;
}
</style>
