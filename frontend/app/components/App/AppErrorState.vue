<script setup lang="ts">
import type { AppErrorVariant } from '~/utils/error-page'

const props = withDefaults(
  defineProps<{
    code: number
    eyebrow: string
    text: string
    title: string
    variant?: AppErrorVariant
  }>(),
  {
    variant: 'generic',
  },
)

const root = useTemplateRef<HTMLElement>('root')

onMounted(() => {
  root.value?.focus()
})
</script>

<template>
  <article
    ref="root"
    class="app-error-state"
    :class="`app-error-state--${props.variant}`"
    :aria-labelledby="`app-error-title-${props.code}`"
    aria-atomic="true"
    aria-live="assertive"
    role="alert"
    tabindex="-1"
  >
    <div class="app-error-state__visual">
      <svg
        class="app-error-state__icon"
        viewBox="0 0 160 160"
        fill="none"
        aria-hidden="true"
        focusable="false"
      >
        <circle class="app-error-state__orbit" cx="80" cy="80" r="61" />
        <path
          class="app-error-state__gate"
          d="M52 119V61c0-17 12-30 28-30s28 13 28 30v58"
        />
        <path class="app-error-state__floor" d="M38 119h84" />

        <g v-if="props.variant === 'not-found'">
          <path class="app-error-state__signal" d="M62 75h13l8-12 12 23h12" />
          <path class="app-error-state__accent" d="m68 98 24-24m0 24L68 74" />
        </g>

        <g v-else-if="props.variant === 'forbidden'">
          <path
            class="app-error-state__signal"
            d="M80 55 99 63v14c0 15-8 25-19 30-11-5-19-15-19-30V63l19-8Z"
          />
          <path
            class="app-error-state__accent"
            d="M72 80h16m-12 0v-6a4 4 0 0 1 8 0v6"
          />
        </g>

        <g v-else-if="props.variant === 'rate-limit'">
          <path
            class="app-error-state__signal"
            d="M56 83h48M62 68h36M68 98h24"
          />
          <circle class="app-error-state__accent" cx="80" cy="83" r="29" />
        </g>

        <g v-else-if="props.variant === 'server'">
          <rect
            class="app-error-state__signal"
            x="57"
            y="58"
            width="46"
            height="18"
            rx="5"
          />
          <rect
            class="app-error-state__signal"
            x="57"
            y="86"
            width="46"
            height="18"
            rx="5"
          />
          <path class="app-error-state__accent" d="M66 67h14m-14 28h20" />
        </g>

        <g v-else>
          <circle class="app-error-state__signal" cx="80" cy="80" r="27" />
          <path class="app-error-state__accent" d="M80 64v21m0 10h.1" />
        </g>
      </svg>
      <span class="app-error-state__code">HTTP {{ props.code }}</span>
    </div>

    <div class="app-error-state__copy">
      <p class="app-error-state__eyebrow">{{ props.eyebrow }}</p>
      <h1 :id="`app-error-title-${props.code}`" class="app-error-state__title">
        {{ props.title }}
      </h1>
      <p class="app-error-state__text">{{ props.text }}</p>
    </div>

    <div v-if="$slots.actions" class="app-error-state__actions">
      <slot name="actions" />
    </div>
  </article>
</template>

<style scoped>
.app-error-state {
  --app-error-color: var(--v-theme-primary);
  display: grid;
  width: min(100%, 740px);
  grid-template-columns: 172px minmax(0, 1fr);
  gap: 26px 32px;
  align-items: center;
  padding: clamp(24px, 5vw, 42px);
  border: 1px solid rgba(var(--app-shell-border), 0.58);
  border-radius: var(--app-card-radius);
  background:
    radial-gradient(
      circle at 10% 14%,
      rgba(var(--app-error-color), 0.12),
      transparent 32%
    ),
    rgb(var(--app-shell-surface-strong));
  box-shadow: var(--app-card-shadow);
}

.app-error-state:focus {
  outline: none;
}

.app-error-state--forbidden,
.app-error-state--rate-limit {
  --app-error-color: var(--v-theme-warning);
}

.app-error-state--server {
  --app-error-color: var(--v-theme-error);
}

.app-error-state__visual {
  display: grid;
  justify-items: center;
  gap: 8px;
  color: rgb(var(--app-error-color));
}

.app-error-state__icon {
  width: 152px;
  max-width: 100%;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.app-error-state__orbit {
  stroke-dasharray: 7 10;
  stroke-width: 2;
  opacity: 0.28;
}

.app-error-state__gate,
.app-error-state__floor {
  stroke-width: 5;
  opacity: 0.36;
}

.app-error-state__signal {
  stroke-width: 5;
}

.app-error-state__accent {
  stroke-width: 4;
}

.app-error-state__code,
.app-error-state__eyebrow {
  font-family: var(--app-font-mono);
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.app-error-state__code {
  color: rgb(var(--app-error-color));
}

.app-error-state__copy {
  display: grid;
  gap: 8px;
}

.app-error-state__eyebrow,
.app-error-state__title,
.app-error-state__text {
  margin: 0;
}

.app-error-state__eyebrow {
  color: rgb(var(--app-shell-text-muted));
}

.app-error-state__title {
  color: rgb(var(--app-shell-text-primary));
  font-family: var(--app-font-display);
  font-size: clamp(1.75rem, 4vw, 2.35rem);
  line-height: 1.08;
  letter-spacing: -0.025em;
}

.app-error-state__text {
  max-width: 34rem;
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.6;
}

.app-error-state__actions {
  display: flex;
  grid-column: 2;
  flex-wrap: wrap;
  gap: 10px;
}

@media (max-width: 600px) {
  .app-error-state {
    grid-template-columns: 1fr;
    gap: 18px;
    text-align: center;
  }

  .app-error-state__icon {
    width: 118px;
  }

  .app-error-state__copy {
    justify-items: center;
  }

  .app-error-state__actions {
    grid-column: 1;
    justify-content: center;
  }
}
</style>
