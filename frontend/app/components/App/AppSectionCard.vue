<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    icon?: string
    subtitle?: string
    title?: string
  }>(),
  {
    icon: '',
    subtitle: '',
    title: '',
  },
)
</script>

<template>
  <v-card rounded="lg" class="app-section-card">
    <div
      v-if="props.icon || props.title || props.subtitle || $slots.actions"
      class="app-section-card__header"
    >
      <div class="app-section-card__heading">
        <v-avatar v-if="props.icon" color="primary" variant="tonal" size="36">
          <v-icon :icon="props.icon" />
        </v-avatar>

        <div class="app-section-card__heading-copy">
          <h2 v-if="props.title">{{ props.title }}</h2>
          <p v-if="props.subtitle">{{ props.subtitle }}</p>
        </div>
      </div>

      <div v-if="$slots.actions" class="app-section-card__actions">
        <slot name="actions" />
      </div>
    </div>

    <div class="app-section-card__scroll">
      <slot />
    </div>
  </v-card>
</template>

<style scoped>
.app-section-card {
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background: linear-gradient(
    180deg,
    rgba(var(--app-shell-surface-strong), 0.98) 0%,
    rgba(var(--app-shell-surface), 0.98) 100%
  );
  box-shadow: var(--app-card-shadow-soft);
  overflow: hidden;
}

.app-section-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 24px 16px;
}

.app-section-card__heading {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.app-section-card__heading-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.app-section-card__heading-copy h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
}

.app-section-card__heading-copy p {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
}

.app-section-card__actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
}

.app-section-card__scroll {
  overflow-x: auto;
}

@media (max-width: 720px) {
  .app-section-card__header {
    align-items: stretch;
    flex-direction: column;
  }

  .app-section-card__actions {
    justify-content: flex-start;
  }
}
</style>
