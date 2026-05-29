<script setup lang="ts">
import type { DashboardScope } from '~/types/user-service'

const model = defineModel<DashboardScope>({ required: true })

const options: { title: string; value: DashboardScope }[] = [
  { title: 'My usage', value: 'self' },
  { title: 'Global', value: 'global' },
]
</script>

<template>
  <div class="dashboard-scope-select">
    <span class="dashboard-scope-select__label">Scope</span>
    <v-select
      v-model="model"
      class="dashboard-scope-select__control"
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
.dashboard-scope-select {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 248px;
  padding: 6px 6px 6px 12px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: 8px;
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.dashboard-scope-select__label {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 700;
  text-transform: uppercase;
  white-space: nowrap;
}

.dashboard-scope-select__control {
  flex: 0 0 168px;
  width: 168px;
}

.dashboard-scope-select__control :deep(.v-field) {
  border-radius: var(--app-control-radius);
  background: rgba(var(--app-shell-surface-muted), 0.62);
}

.dashboard-scope-select__control :deep(.v-field__input) {
  min-height: 32px;
  padding-top: 0;
  padding-bottom: 0;
  color: rgb(var(--app-shell-text-primary));
  font-weight: 650;
}

@media (max-width: 720px) {
  .dashboard-scope-select {
    min-width: 0;
    justify-content: space-between;
  }

  .dashboard-scope-select__control {
    flex: 0 0 168px;
  }
}
</style>
