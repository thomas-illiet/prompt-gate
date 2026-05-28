<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    compact?: boolean
    icon?: string
    text: string
    title: string
    tone?: 'primary' | 'success' | 'warning' | 'error' | 'info'
  }>(),
  {
    compact: false,
    icon: 'mdi-radar',
    tone: 'primary',
  },
)
</script>

<template>
  <section
    class="app-empty-state"
    :class="{ 'app-empty-state--compact': props.compact }"
  >
    <div class="app-empty-state__visual" aria-hidden="true">
      <v-avatar
        class="app-empty-state__avatar"
        :color="props.tone"
        variant="tonal"
        size="56"
      >
        <v-icon :icon="props.icon" size="30" />
      </v-avatar>
      <div class="app-empty-state__signal">
        <span />
        <span />
        <span />
      </div>
    </div>

    <div class="app-empty-state__copy">
      <h3>{{ props.title }}</h3>
      <p>{{ props.text }}</p>
    </div>

    <div v-if="$slots.actions" class="app-empty-state__actions">
      <slot name="actions" />
    </div>
  </section>
</template>

<style scoped>
.app-empty-state {
  display: grid;
  min-height: 220px;
  place-items: center;
  align-content: center;
  gap: 14px;
  padding: 28px 18px;
  color: rgb(var(--app-shell-text-secondary));
  text-align: center;
}

.app-empty-state--compact {
  min-height: 160px;
  padding: 20px 14px;
}

.app-empty-state__visual {
  position: relative;
  display: grid;
  place-items: center;
  width: 112px;
  height: 84px;
}

.app-empty-state__avatar {
  position: relative;
  z-index: 1;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  box-shadow: 0 18px 34px -28px rgba(var(--app-shell-shadow), 0.55);
}

.app-empty-state__signal {
  position: absolute;
  right: 0;
  bottom: 8px;
  left: 0;
  display: grid;
  gap: 5px;
  justify-items: center;
}

.app-empty-state__signal span {
  display: block;
  height: 4px;
  border-radius: 999px;
  background: rgba(var(--v-theme-primary), 0.28);
  animation: app-empty-state-scan 1.7s ease-in-out infinite;
}

.app-empty-state__signal span:nth-child(1) {
  width: 72px;
}

.app-empty-state__signal span:nth-child(2) {
  width: 52px;
  animation-delay: 0.16s;
}

.app-empty-state__signal span:nth-child(3) {
  width: 34px;
  animation-delay: 0.32s;
}

.app-empty-state__copy {
  display: grid;
  max-width: 32rem;
  gap: 6px;
}

.app-empty-state__copy h3 {
  margin: 0;
  color: rgb(var(--app-shell-text-primary));
  font-size: 1rem;
  font-weight: 800;
}

.app-empty-state__copy p {
  margin: 0;
  line-height: 1.55;
}

.app-empty-state__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
}

@keyframes app-empty-state-scan {
  50% {
    opacity: 0.4;
    transform: translateY(-3px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-empty-state__signal span {
    animation: none;
  }
}
</style>
