<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    caption?: string
    color: string
    error?: string | null
    formatter?: (value: number) => string
    icon: string
    loading?: boolean
    title: string
    value?: number | null
  }>(),
  {
    caption: '',
    error: null,
    formatter: (value: number) => value.toString(),
    loading: false,
    value: null,
  },
)

const emit = defineEmits<{
  retry: []
}>()

const accentStyle = computed(() => ({
  '--dashboard-kpi-accent': `rgb(var(--v-theme-${props.color}))`,
}))

const animatedValue = shallowRef(0)
const animationFrame = shallowRef<number | null>(null)
const displayValue = computed(() =>
  props.value == null ? '0' : props.formatter(animatedValue.value),
)

function cancelValueAnimation() {
  if (animationFrame.value == null || !import.meta.client) {
    return
  }

  window.cancelAnimationFrame(animationFrame.value)
  animationFrame.value = null
}

function shouldAnimateValue() {
  if (
    !import.meta.client ||
    typeof window.requestAnimationFrame !== 'function'
  ) {
    return false
  }

  return !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}

function animateValue(nextValue: number) {
  cancelValueAnimation()

  if (!shouldAnimateValue()) {
    animatedValue.value = nextValue
    return
  }

  const startValue = animatedValue.value
  const delta = nextValue - startValue
  const startedAt = performance.now()
  const duration = 720

  function step(now: number) {
    const progress = Math.min((now - startedAt) / duration, 1)
    const easedProgress = 1 - Math.pow(1 - progress, 3)
    animatedValue.value = Math.round(startValue + delta * easedProgress)

    if (progress < 1) {
      animationFrame.value = window.requestAnimationFrame(step)
      return
    }

    animatedValue.value = nextValue
    animationFrame.value = null
  }

  animationFrame.value = window.requestAnimationFrame(step)
}

watch(
  () => props.value,
  (value) => {
    if (value == null) {
      cancelValueAnimation()
      animatedValue.value = 0
      return
    }

    animateValue(value)
  },
  { immediate: true },
)

onBeforeUnmount(cancelValueAnimation)
</script>

<template>
  <section
    class="dashboard-kpi-card"
    :style="accentStyle"
    :aria-busy="props.loading"
    :aria-label="props.title"
  >
    <span class="dashboard-kpi-card__accent" />
    <span class="dashboard-kpi-card__pulse" aria-hidden="true" />

    <div class="dashboard-kpi-card__header">
      <v-avatar
        class="dashboard-kpi-card__icon"
        :color="props.color"
        variant="tonal"
        size="42"
      >
        <v-icon :icon="props.icon" size="24" />
      </v-avatar>

      <div class="dashboard-kpi-card__copy">
        <span class="dashboard-kpi-card__title">{{ props.title }}</span>
        <strong v-if="!(props.loading && props.value == null)">
          {{ displayValue }}
        </strong>
        <span v-else class="dashboard-kpi-card__value-skeleton" />
      </div>
    </div>

    <div class="dashboard-kpi-card__footer">
      <template v-if="props.error">
        <span class="dashboard-kpi-card__error">{{ props.error }}</span>
        <v-btn
          icon="mdi-refresh"
          size="small"
          variant="text"
          color="primary"
          aria-label="Retry"
          @click="emit('retry')"
        />
      </template>
      <template v-else>
        <span>{{ props.loading ? 'Refreshing' : props.caption }}</span>
        <v-progress-circular
          v-if="props.loading"
          :color="props.color"
          indeterminate
          size="16"
          width="2"
        />
      </template>
    </div>
  </section>
</template>

<style scoped>
.dashboard-kpi-card {
  position: relative;
  display: flex;
  min-height: 116px;
  flex-direction: column;
  justify-content: space-between;
  gap: 16px;
  overflow: hidden;
  padding: 18px 18px 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: 8px;
  background: linear-gradient(
    180deg,
    rgba(var(--app-shell-surface-strong), 0.98) 0%,
    rgba(var(--app-shell-surface), 0.98) 100%
  );
  box-shadow: var(--app-card-shadow-soft);
}

.dashboard-kpi-card__accent {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: var(--dashboard-kpi-accent);
  content: '';
}

.dashboard-kpi-card__pulse {
  position: absolute;
  right: 14px;
  bottom: 12px;
  width: 96px;
  height: 34px;
  opacity: 0.2;
  background: linear-gradient(
    90deg,
    transparent 0,
    transparent 12px,
    var(--dashboard-kpi-accent) 12px,
    var(--dashboard-kpi-accent) 14px,
    transparent 14px,
    transparent 28px
  );
  background-size: 28px 100%;
  mask-image: linear-gradient(90deg, transparent, #000 22%, #000);
  animation: dashboard-kpi-card-signal 1.8s linear infinite;
}

.dashboard-kpi-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.dashboard-kpi-card__icon {
  flex: 0 0 auto;
}

.dashboard-kpi-card__copy {
  display: grid;
  justify-items: end;
  min-width: 0;
  gap: 4px;
  text-align: right;
}

.dashboard-kpi-card__title {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 700;
  text-transform: uppercase;
}

.dashboard-kpi-card__copy strong {
  color: rgb(var(--app-shell-text-primary));
  font-size: 1.55rem;
  font-weight: 750;
  line-height: 1.05;
}

.dashboard-kpi-card__value-skeleton {
  display: block;
  width: 96px;
  height: 28px;
  border-radius: 6px;
  background: rgba(var(--app-shell-border), 0.42);
  animation: dashboard-kpi-card-pulse 1.2s ease-in-out infinite;
}

.dashboard-kpi-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 24px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.86rem;
}

.dashboard-kpi-card__footer span {
  min-width: 0;
}

.dashboard-kpi-card__footer span:not(.dashboard-kpi-card__error) {
  overflow-wrap: anywhere;
}

.dashboard-kpi-card__error {
  overflow: hidden;
  min-width: 0;
  color: rgb(var(--v-theme-error));
  text-overflow: ellipsis;
  white-space: nowrap;
}

@keyframes dashboard-kpi-card-pulse {
  50% {
    opacity: 0.48;
  }
}

@keyframes dashboard-kpi-card-signal {
  to {
    background-position-x: 28px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .dashboard-kpi-card__pulse,
  .dashboard-kpi-card__value-skeleton {
    animation: none;
  }
}

@media (max-width: 720px) {
  .dashboard-kpi-card__copy {
    justify-items: start;
    text-align: left;
  }
}
</style>
