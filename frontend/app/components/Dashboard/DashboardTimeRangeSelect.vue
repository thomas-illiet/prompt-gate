<script setup lang="ts">
import type { UsageWindow } from '~/types/user-service'

const model = defineModel<UsageWindow>({ required: true })

const options: { title: string; value: UsageWindow }[] = [
  { title: '7 days', value: '7d' },
  { title: '30 days', value: '30d' },
  { title: 'All time', value: 'all' },
]
</script>

<template>
  <div class="dashboard-time-range-select">
    <span class="dashboard-time-range-select__label">Window</span>
    <v-select
      v-model="model"
      aria-label="Usage window"
      class="dashboard-time-range-select__control"
      density="compact"
      variant="solo"
      flat
      hide-details
      menu-icon="mdi-chevron-down"
      :items="options"
    />
  </div>
</template>

<style scoped>
.dashboard-time-range-select {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 6px 6px 12px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: 8px;
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.dashboard-time-range-select__label {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 700;
  text-transform: uppercase;
}

.dashboard-time-range-select__control {
  width: 128px;
}

.dashboard-time-range-select__control :deep(.v-field) {
  border-radius: var(--app-control-radius);
  background: rgba(var(--app-shell-surface-muted), 0.62);
}

.dashboard-time-range-select__control :deep(.v-field__input) {
  min-height: 32px;
  padding-top: 0;
  padding-bottom: 0;
  color: rgb(var(--app-shell-text-primary));
  font-weight: 650;
}

@media (max-width: 720px) {
  .dashboard-time-range-select {
    justify-content: space-between;
  }

  .dashboard-time-range-select__control {
    flex: 0 0 150px;
  }
}
</style>
