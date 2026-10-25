<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    icon?: string
    subtitle?: string
    title?: string
  }>(),
  {
    icon: 'mdi-filter-variant',
    subtitle: '',
    title: '',
  },
)
</script>

<template>
  <v-card rounded="lg" class="app-filter-card">
    <v-card-text class="pa-5">
      <div
        v-if="props.icon || props.title || props.subtitle"
        class="app-filter-card__header"
      >
        <v-avatar
          v-if="props.icon"
          color="primary"
          variant="tonal"
          size="36"
          class="app-filter-card__icon"
        >
          <v-icon :icon="props.icon" />
        </v-avatar>

        <div class="app-filter-card__heading-copy">
          <h2 v-if="props.title">{{ props.title }}</h2>
          <p v-if="props.subtitle">{{ props.subtitle }}</p>
        </div>
      </div>
      <slot />
    </v-card-text>
  </v-card>
</template>

<style scoped>
.app-filter-card {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    radial-gradient(
      circle at 94% 0,
      rgba(var(--app-accent-butter), 0.1),
      transparent 18rem
    ),
    linear-gradient(
      180deg,
      rgba(var(--app-shell-surface-strong), 0.98) 0%,
      rgba(var(--app-shell-surface), 0.98) 100%
    );
  box-shadow: var(--app-card-shadow-soft);
}

.app-filter-card::before {
  position: absolute;
  top: 0;
  left: 24px;
  width: 72px;
  height: 4px;
  border-radius: 0 0 999px 999px;
  background: linear-gradient(
    90deg,
    rgb(var(--v-theme-primary)),
    rgb(var(--app-accent-butter))
  );
  content: '';
  pointer-events: none;
}

.app-filter-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
  min-width: 0;
}

.app-filter-card__heading-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.app-filter-card__icon {
  border: 1px solid rgba(var(--v-theme-primary), 0.16);
  border-radius: 13px !important;
  background: rgba(var(--v-theme-primary), 0.09) !important;
  box-shadow: 3px 4px 0 rgba(var(--app-accent-coral), 0.08);
}

.app-filter-card__heading-copy h2 {
  margin: 0;
  font-size: 1rem;
  color: rgb(var(--app-shell-text-primary));
  font-weight: 800;
}

.app-filter-card__heading-copy p {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
}

@media (max-width: 720px) {
  .app-filter-card::before {
    left: 16px;
  }

  .app-filter-card :deep(.v-card-text) {
    padding: 16px !important;
  }

  .app-filter-card__header {
    margin-bottom: 12px;
  }
}
</style>
