<script setup lang="ts">
import type { PriceRates } from '~/types/pricing'

const props = defineProps<{
  fallback: PriceRates
  loading: boolean
}>()

const emit = defineEmits<{
  save: [payload: PriceRates]
}>()

const input = shallowRef(0)
const output = shallowRef(0)
const hasSubmitted = shallowRef(false)
const formId = 'admin-pricing-fallback-form'

const inputError = computed(() => priceError(input.value))
const outputError = computed(() => priceError(output.value))
const canSave = computed(() => !inputError.value && !outputError.value)

watch(
  () => props.fallback,
  (fallback) => {
    input.value = fallback.input
    output.value = fallback.output
    hasSubmitted.value = false
  },
  { immediate: true },
)

function priceError(value: number) {
  if (!hasSubmitted.value) {
    return ''
  }
  if (!Number.isFinite(Number(value)) || Number(value) < 0) {
    return 'Use a positive value or zero.'
  }
  return ''
}

function save() {
  hasSubmitted.value = true
  if (!canSave.value) {
    return
  }
  emit('save', { input: Number(input.value), output: Number(output.value) })
}
</script>

<template>
  <AppSectionCard
    icon="mdi-currency-usd"
    title="Fallback pricing"
    subtitle="Default USD rates per 1M tokens used when no model-specific price exists."
  >
    <template #actions>
      <v-btn
        color="primary"
        :form="formId"
        :loading="props.loading"
        prepend-icon="mdi-content-save-outline"
        rounded="lg"
        type="submit"
      >
        Save
      </v-btn>
    </template>

    <form
      :id="formId"
      class="admin-pricing-fallback"
      @submit.prevent="save"
    >
      <div class="admin-pricing-fallback__grid">
        <div class="admin-pricing-fallback__rate-card">
          <label
            class="admin-pricing-fallback__rate-header"
            for="fallback-pricing-input"
          >
            <v-icon
              class="admin-pricing-fallback__rate-icon"
              color="primary"
              icon="mdi-import"
              size="20"
            />

            <span class="admin-pricing-fallback__rate-copy">
              <span class="admin-pricing-fallback__rate-label">
                Input rate
              </span>
              <span class="admin-pricing-fallback__rate-hint">
                USD per 1M input tokens
              </span>
            </span>
          </label>

          <v-text-field
            id="fallback-pricing-input"
            v-model.number="input"
            aria-label="Input USD per 1M tokens"
            class="admin-pricing-fallback__field"
            density="comfortable"
            hide-details="auto"
            min="0"
            prefix="$"
            step="0.000001"
            type="number"
            variant="solo-filled"
            :error="Boolean(inputError)"
            :error-messages="inputError ? [inputError] : []"
          />
        </div>

        <div class="admin-pricing-fallback__rate-card">
          <label
            class="admin-pricing-fallback__rate-header"
            for="fallback-pricing-output"
          >
            <v-icon
              class="admin-pricing-fallback__rate-icon"
              color="primary"
              icon="mdi-export"
              size="20"
            />

            <span class="admin-pricing-fallback__rate-copy">
              <span class="admin-pricing-fallback__rate-label">
                Output rate
              </span>
              <span class="admin-pricing-fallback__rate-hint">
                USD per 1M output tokens
              </span>
            </span>
          </label>

          <v-text-field
            id="fallback-pricing-output"
            v-model.number="output"
            aria-label="Output USD per 1M tokens"
            class="admin-pricing-fallback__field"
            density="comfortable"
            hide-details="auto"
            min="0"
            prefix="$"
            step="0.000001"
            type="number"
            variant="solo-filled"
            :error="Boolean(outputError)"
            :error-messages="outputError ? [outputError] : []"
          />
        </div>
      </div>
    </form>
  </AppSectionCard>
</template>

<style scoped>
.admin-pricing-fallback {
  padding: 4px 24px 24px;
}

.admin-pricing-fallback__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(240px, 1fr));
  gap: 16px;
  max-width: 1120px;
}

.admin-pricing-fallback__rate-card {
  display: grid;
  gap: 14px;
  min-width: 0;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.58);
}

.admin-pricing-fallback__rate-header {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.admin-pricing-fallback__rate-icon {
  flex: 0 0 auto;
  margin-top: 1px;
}

.admin-pricing-fallback__rate-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.admin-pricing-fallback__rate-label {
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.875rem;
  font-weight: 700;
  line-height: 1.25;
}

.admin-pricing-fallback__rate-hint {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.8125rem;
  line-height: 1.3;
}

.admin-pricing-fallback__field {
  min-width: 0;
}

.admin-pricing-fallback__field :deep(.v-field__input) {
  font-weight: 700;
}

@media (max-width: 960px) {
  .admin-pricing-fallback__grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .admin-pricing-fallback {
    padding: 8px 16px 16px;
  }
}
</style>
