<script setup lang="ts">
import type { MissingModelPrice, PricingCheckResponse } from '~/types/pricing'

const props = defineProps<{
  check: PricingCheckResponse | null
  loading: boolean
}>()

const emit = defineEmits<{
  addMissing: [missing: MissingModelPrice]
  refresh: []
}>()

const visibleMissing = computed(
  () => props.check?.missingPrices.slice(0, 8) ?? [],
)
const hiddenMissingCount = computed(() =>
  Math.max(
    (props.check?.missingPrices.length ?? 0) - visibleMissing.value.length,
    0,
  ),
)
const hasIssues = computed(
  () =>
    Boolean(props.check) &&
    ((props.check?.missingPrices.length ?? 0) > 0 ||
      (props.check?.providerErrors.length ?? 0) > 0),
)
</script>

<template>
  <section
    v-if="hasIssues"
    class="admin-pricing-alert"
    role="status"
    aria-live="polite"
  >
    <div class="admin-pricing-alert__marker" aria-hidden="true" />

    <div class="admin-pricing-alert__layout">
      <v-avatar
        class="admin-pricing-alert__icon"
        color="warning"
        variant="tonal"
        size="40"
      >
        <v-icon icon="mdi-alert-circle" size="24" />
      </v-avatar>

      <div class="admin-pricing-alert__body">
        <div class="admin-pricing-alert__header">
          <div class="admin-pricing-alert__copy">
            <h2>Pricing configuration is incomplete</h2>
            <p v-if="props.check?.missingPrices.length">
              {{ props.check.missingPrices.length }} provider/model pair<span
                v-if="props.check.missingPrices.length > 1"
                >s</span
              >
              missing model-specific pricing.
            </p>
          </div>

          <v-btn
            class="admin-pricing-alert__refresh"
            color="warning"
            variant="flat"
            rounded="lg"
            prepend-icon="mdi-refresh"
            :loading="props.loading"
            @click="emit('refresh')"
          >
            Recheck pricing
          </v-btn>
        </div>

        <div v-if="visibleMissing.length" class="admin-pricing-alert__chips">
          <button
            v-for="missing in visibleMissing"
            :key="`${missing.providerName}:${missing.model}`"
            type="button"
            class="admin-pricing-alert__chip"
            :aria-label="`Add pricing for ${missing.providerName} / ${missing.model}`"
            @click="emit('addMissing', missing)"
          >
            <span>{{ missing.providerName }} / {{ missing.model }}</span>
            <v-icon icon="mdi-plus" size="16" />
          </button>

          <span
            v-if="hiddenMissingCount"
            class="admin-pricing-alert__more"
            aria-label="More missing pricing entries"
          >
            +{{ hiddenMissingCount }} more
          </span>
        </div>

        <div
          v-if="props.check?.providerErrors.length"
          class="admin-pricing-alert__errors"
        >
          <p>Some providers could not return their model catalog:</p>
          <v-chip
            v-for="error in props.check.providerErrors"
            :key="error.providerName"
            size="small"
            label
            variant="tonal"
            color="error"
          >
            {{ error.providerName }}: {{ error.message }}
          </v-chip>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.admin-pricing-alert {
  position: relative;
  margin-bottom: 16px;
  overflow: hidden;
  border: 1px solid rgba(var(--v-theme-warning), 0.32);
  border-radius: var(--app-card-radius);
  background:
    linear-gradient(
      90deg,
      rgba(var(--v-theme-warning), 0.1) 0%,
      rgba(var(--app-shell-surface-strong), 0.98) 24%,
      rgba(var(--app-shell-surface), 0.98) 100%
    ),
    rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
  color: rgb(var(--app-shell-text-primary));
}

.admin-pricing-alert__marker {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 4px;
  background: rgb(var(--v-theme-warning));
}

.admin-pricing-alert__layout {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 16px;
  padding: 20px 24px 22px;
}

.admin-pricing-alert__icon {
  border: 1px solid rgba(var(--v-theme-warning), 0.28);
  background: rgba(var(--v-theme-warning), 0.12) !important;
  color: rgb(var(--v-theme-warning));
}

.admin-pricing-alert__body {
  display: grid;
  gap: 16px;
  min-width: 0;
}

.admin-pricing-alert__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

.admin-pricing-alert__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.admin-pricing-alert__copy h2 {
  margin: 0;
  color: rgb(var(--app-shell-text-primary));
  font-size: 1.08rem;
  font-weight: 800;
  line-height: 1.25;
}

.admin-pricing-alert__copy p {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.45;
}

.admin-pricing-alert__chips,
.admin-pricing-alert__errors {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.admin-pricing-alert__chip {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  min-height: 32px;
  gap: 8px;
  padding: 5px 10px;
  border: 1px solid rgba(var(--v-theme-warning), 0.38);
  border-radius: var(--app-chip-radius);
  background: rgba(var(--app-shell-surface-strong), 0.9);
  color: rgb(var(--app-shell-text-secondary));
  cursor: pointer;
  font: inherit;
  line-height: 1.2;
  transition:
    border-color 0.16s ease,
    background-color 0.16s ease,
    color 0.16s ease;
}

.admin-pricing-alert__chip span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-pricing-alert__chip .v-icon {
  flex: 0 0 auto;
  color: rgb(var(--v-theme-warning));
}

.admin-pricing-alert__chip:hover,
.admin-pricing-alert__chip:focus-visible {
  border-color: rgba(var(--v-theme-warning), 0.72);
  background: rgba(var(--v-theme-warning), 0.08);
  color: rgb(var(--app-shell-text-primary));
  outline: none;
}

.admin-pricing-alert__more {
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  padding: 5px 10px;
  border-radius: var(--app-chip-radius);
  background: rgba(var(--app-shell-border), 0.28);
  color: rgb(var(--app-shell-text-secondary));
  font-weight: 650;
  line-height: 1.2;
}

.admin-pricing-alert__errors {
  display: grid;
}

.admin-pricing-alert__errors p {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
}

.admin-pricing-alert__refresh {
  flex: 0 0 auto;
  color: rgb(var(--v-theme-on-warning));
  box-shadow: none;
}

@media (max-width: 720px) {
  .admin-pricing-alert__layout {
    grid-template-columns: 1fr;
    padding: 18px;
  }

  .admin-pricing-alert__header {
    align-items: stretch;
    flex-direction: column;
  }

  .admin-pricing-alert__refresh {
    width: 100%;
  }
}
</style>
