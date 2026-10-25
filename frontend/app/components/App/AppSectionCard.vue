<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    headingTag?: 'h1' | 'h2' | 'h3'
    icon?: string
    subtitle?: string
    title?: string
  }>(),
  {
    headingTag: 'h2',
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
        <v-avatar
          v-if="props.icon"
          color="primary"
          variant="tonal"
          size="36"
          class="app-section-card__icon"
        >
          <v-icon :icon="props.icon" />
        </v-avatar>

        <div class="app-section-card__heading-copy">
          <component :is="props.headingTag" v-if="props.title">
            {{ props.title }}
          </component>
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
  position: relative;
  isolation: isolate;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    radial-gradient(
      circle at 96% 0,
      rgba(var(--app-accent-coral), 0.055),
      transparent 22rem
    ),
    linear-gradient(
      180deg,
      rgba(var(--app-shell-surface-strong), 0.98) 0%,
      rgba(var(--app-shell-surface), 0.98) 100%
    );
  box-shadow: var(--app-card-shadow-soft);
  overflow: hidden;
}

.app-section-card::before {
  position: absolute;
  z-index: 1;
  top: 0;
  left: 24px;
  width: 88px;
  height: 4px;
  border-radius: 0 0 999px 999px;
  background: linear-gradient(
    90deg,
    rgb(var(--v-theme-primary)),
    rgb(var(--app-accent-butter)) 62%,
    rgb(var(--app-accent-coral))
  );
  content: '';
  pointer-events: none;
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

.app-section-card__icon {
  border: 1px solid rgba(var(--v-theme-primary), 0.16);
  border-radius: 13px !important;
  background: rgba(var(--v-theme-primary), 0.09) !important;
  box-shadow: 3px 4px 0 rgba(var(--app-accent-butter), 0.12);
}

.app-section-card__heading-copy :is(h1, h2, h3) {
  margin: 0;
  font-size: 1rem;
  color: rgb(var(--app-shell-text-primary));
  font-weight: 800;
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
  overflow-y: hidden;
}

.app-section-card__header + .app-section-card__scroll {
  border-top: 1px solid rgba(var(--app-shell-border), 0.28);
  background: rgba(var(--app-shell-surface-strong), 0.2);
}

@media (max-width: 720px) {
  .app-section-card__header {
    align-items: stretch;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
  }

  .app-section-card::before {
    left: 16px;
    width: 72px;
  }

  .app-section-card__actions {
    justify-content: flex-start;
  }
}
</style>
