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
        <v-avatar v-if="props.icon" color="primary" variant="tonal" size="36">
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
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background: linear-gradient(
    180deg,
    rgba(var(--app-shell-surface-strong), 0.98) 0%,
    rgba(var(--app-shell-surface), 0.98) 100%
  );
  box-shadow: var(--app-card-shadow-soft);
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

.app-filter-card__heading-copy h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
}

.app-filter-card__heading-copy p {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
}
</style>
