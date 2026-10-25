<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'

interface ThemeCardStat {
  label: string
  value: string
}

const props = defineProps<{
  copy: string
  error?: string | null
  icon: string
  loading: boolean
  stats: ThemeCardStat[]
  title: string
  to: RouteLocationRaw
}>()
</script>

<template>
  <v-card
    class="admin-dashboard-theme-card"
    :to="props.to"
    rounded="lg"
    :aria-busy="props.loading"
  >
    <v-card-text>
      <div class="admin-dashboard-theme-card__heading">
        <v-avatar color="primary" size="42" variant="tonal">
          <v-icon :icon="props.icon" size="23" />
        </v-avatar>
        <div>
          <h2>{{ props.title }}</h2>
          <p>{{ props.copy }}</p>
        </div>
        <v-icon icon="mdi-arrow-right" size="22" />
      </div>

      <v-alert
        v-if="props.error"
        class="admin-dashboard-theme-card__error"
        type="warning"
        variant="tonal"
        density="compact"
      >
        {{ props.error }}
      </v-alert>

      <div v-else class="admin-dashboard-theme-card__stats">
        <div v-for="stat in props.stats" :key="stat.label">
          <strong v-if="!props.loading">{{ stat.value }}</strong>
          <span v-else class="admin-dashboard-theme-card__skeleton" />
          <span>{{ stat.label }}</span>
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.admin-dashboard-theme-card {
  height: 100%;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
  transition:
    border-color var(--app-motion-fast) var(--app-ease-standard),
    transform var(--app-motion-fast) var(--app-ease-out);
}

.admin-dashboard-theme-card:hover,
.admin-dashboard-theme-card:focus-visible {
  border-color: rgba(var(--v-theme-primary), 0.42);
  transform: translateY(-2px);
}

.admin-dashboard-theme-card :deep(.v-card-text) {
  display: grid;
  gap: 20px;
  padding: 20px;
}

.admin-dashboard-theme-card__heading {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  gap: 13px;
}

.admin-dashboard-theme-card__heading h2,
.admin-dashboard-theme-card__heading p {
  margin: 0;
}

.admin-dashboard-theme-card__heading h2 {
  font-size: 1.05rem;
}

.admin-dashboard-theme-card__heading p {
  margin-top: 4px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.87rem;
  line-height: 1.45;
}

.admin-dashboard-theme-card__stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.admin-dashboard-theme-card__stats > div {
  display: grid;
  gap: 3px;
  min-width: 0;
  padding: 10px;
  border-radius: 10px;
  background: rgba(var(--app-shell-surface-muted), 0.68);
}

.admin-dashboard-theme-card__stats strong {
  overflow-wrap: anywhere;
  font-size: 1.15rem;
}

.admin-dashboard-theme-card__stats span:last-child {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.72rem;
}

.admin-dashboard-theme-card__skeleton {
  width: 72%;
  height: 20px;
  border-radius: 5px;
  background: rgba(var(--app-shell-border), 0.4);
}

.admin-dashboard-theme-card__error {
  margin: 0;
}

@media (max-width: 420px) {
  .admin-dashboard-theme-card__stats {
    grid-template-columns: 1fr;
  }
}
</style>
