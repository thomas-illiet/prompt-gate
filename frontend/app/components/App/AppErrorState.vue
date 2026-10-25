<script setup lang="ts">
import forbiddenImage from '~/assets/images/errors/error-forbidden.jpg'
import genericImage from '~/assets/images/errors/error-generic.jpg'
import notFoundImage from '~/assets/images/errors/error-not-found.jpg'
import rateLimitImage from '~/assets/images/errors/error-rate-limit.jpg'
import serverImage from '~/assets/images/errors/error-server.jpg'
import type { AppErrorVariant } from '~/utils/error-page'

const errorImages: Record<AppErrorVariant, string> = {
  forbidden: forbiddenImage,
  generic: genericImage,
  'not-found': notFoundImage,
  'rate-limit': rateLimitImage,
  server: serverImage,
}

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
const errorImage = computed(() => errorImages[props.variant])

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
      <span class="app-error-state__signal" aria-hidden="true">
        Green Room detour
      </span>
      <div class="app-error-state__image-frame" aria-hidden="true">
        <img
          :src="errorImage"
          alt=""
          aria-hidden="true"
          class="app-error-state__image"
          data-test="app-error-image"
          :data-variant="props.variant"
          decoding="async"
          fetchpriority="high"
          height="720"
          width="720"
        />
      </div>
      <span class="app-error-state__code">
        <span>HTTP</span>
        <strong>{{ props.code }}</strong>
      </span>
    </div>

    <div class="app-error-state__content">
      <div class="app-error-state__copy">
        <p class="app-error-state__eyebrow">{{ props.eyebrow }}</p>
        <h1
          :id="`app-error-title-${props.code}`"
          class="app-error-state__title"
        >
          {{ props.title }}
        </h1>
        <p class="app-error-state__text">{{ props.text }}</p>
      </div>

      <div v-if="$slots.actions" class="app-error-state__actions">
        <slot name="actions" />
      </div>

      <p class="app-error-state__recovery-note" aria-hidden="true">
        Friendly recovery route
      </p>
    </div>
  </article>
</template>

<style scoped>
.app-error-state {
  --app-error-color: var(--v-theme-primary);
  position: relative;
  display: grid;
  width: min(100%, 1020px);
  min-height: 480px;
  grid-template-columns: minmax(300px, 0.88fr) minmax(0, 1.22fr);
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.64);
  border-radius: calc(var(--app-card-radius) + 10px);
  background:
    radial-gradient(
      circle at 96% 8%,
      rgba(var(--app-accent-coral), 0.08),
      transparent 28%
    ),
    linear-gradient(
      145deg,
      rgba(var(--app-shell-surface-strong), 0.99),
      rgba(var(--app-shell-surface-muted), 0.92)
    );
  box-shadow:
    12px 14px 0 rgba(var(--app-error-color), 0.07),
    var(--app-card-shadow);
}

.app-error-state::before {
  position: absolute;
  z-index: 4;
  top: 0;
  left: 0;
  width: 100%;
  height: 5px;
  background: linear-gradient(
    90deg,
    rgb(var(--app-error-color)),
    rgb(var(--app-accent-butter)) 58%,
    rgb(var(--app-accent-coral))
  );
  content: '';
  pointer-events: none;
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
  position: relative;
  display: grid;
  min-width: 0;
  align-content: center;
  justify-items: center;
  overflow: hidden;
  padding: 56px 38px 42px;
  border-right: 1px dashed rgba(var(--app-shell-border), 0.72);
  background:
    radial-gradient(
      circle at 18% 16%,
      rgba(var(--app-accent-butter), 0.3),
      transparent 32%
    ),
    radial-gradient(
      circle at 84% 82%,
      rgba(var(--app-error-color), 0.18),
      transparent 36%
    ),
    linear-gradient(
      155deg,
      rgba(var(--app-error-color), 0.09),
      rgba(var(--app-shell-surface-strong), 0.76)
    );
  color: rgb(var(--app-error-color));
}

.app-error-state__visual::before {
  position: absolute;
  width: 330px;
  height: 330px;
  transform: translate(-12%, 2%);
  border: 1px solid rgba(var(--app-error-color), 0.16);
  border-radius: 50%;
  box-shadow:
    0 0 0 30px rgba(var(--app-error-color), 0.035),
    0 0 0 64px rgba(var(--app-accent-coral), 0.025);
  content: '';
  pointer-events: none;
}

.app-error-state__signal {
  position: absolute;
  z-index: 2;
  top: 27px;
  left: 30px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: rgb(var(--app-shell-text-muted));
  font-family: var(--app-font-mono);
  font-size: 0.68rem;
  font-weight: 750;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}

.app-error-state__signal::before {
  width: 24px;
  height: 3px;
  border-radius: 999px;
  background: rgb(var(--app-error-color));
  content: '';
}

.app-error-state__image-frame {
  position: relative;
  z-index: 1;
  width: min(100%, 286px);
  aspect-ratio: 1;
  overflow: hidden;
  transform: rotate(-2deg);
  border: 2px solid rgba(var(--app-shell-text-primary), 0.18);
  border-radius: 48px 32px 52px 36px;
  background: rgb(var(--app-shell-surface-muted));
  box-shadow:
    12px 13px 0 rgba(var(--app-error-color), 0.16),
    0 30px 52px -34px rgba(var(--app-shell-shadow), 0.78);
}

.app-error-state__image-frame::after {
  position: absolute;
  inset: 0;
  border: 1px solid rgba(255, 255, 255, 0.34);
  border-radius: inherit;
  content: '';
  pointer-events: none;
}

.app-error-state__image {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.app-error-state__code,
.app-error-state__eyebrow {
  font-family: var(--app-font-mono);
  font-size: 0.75rem;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.app-error-state__code {
  position: absolute;
  z-index: 3;
  right: 24px;
  bottom: 32px;
  display: grid;
  min-width: 86px;
  transform: rotate(3deg);
  justify-items: center;
  padding: 9px 13px 10px;
  border: 1px solid rgba(var(--app-error-color), 0.3);
  border-radius: 18px 14px 20px 13px;
  background: rgb(var(--app-shell-surface-strong));
  box-shadow: 5px 6px 0 rgba(var(--app-accent-coral), 0.14);
  color: rgb(var(--app-shell-text-primary));
  line-height: 1;
}

.app-error-state__code span {
  color: rgb(var(--app-error-color));
  font-size: 0.62rem;
  letter-spacing: 0.17em;
}

.app-error-state__code strong {
  margin-top: 3px;
  font-family: var(--app-font-display);
  font-size: 1.9rem;
  letter-spacing: -0.04em;
}

.app-error-state__content {
  position: relative;
  z-index: 1;
  display: grid;
  align-content: center;
  gap: 28px;
  min-width: 0;
  padding: clamp(36px, 5vw, 62px);
}

.app-error-state__copy {
  display: grid;
  gap: 12px;
}

.app-error-state__eyebrow,
.app-error-state__title,
.app-error-state__text {
  margin: 0;
}

.app-error-state__eyebrow {
  display: flex;
  align-items: center;
  gap: 10px;
  color: rgb(var(--app-shell-text-muted));
}

.app-error-state__eyebrow::before {
  width: 30px;
  height: 3px;
  border-radius: 999px;
  background: rgb(var(--app-error-color));
  content: '';
}

.app-error-state__title {
  color: rgb(var(--app-shell-text-primary));
  font-family: var(--app-font-display);
  font-size: clamp(2.35rem, 4.5vw, 3.4rem);
  font-weight: 850;
  line-height: 1.04;
  letter-spacing: -0.04em;
}

.app-error-state__text {
  max-width: 34rem;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 1.02rem;
  line-height: 1.65;
}

.app-error-state__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.app-error-state__actions :deep(.v-btn) {
  min-height: 44px;
  padding-inline: 20px;
}

.app-error-state__recovery-note {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 0;
  padding-top: 18px;
  border-top: 1px dashed rgba(var(--app-shell-border), 0.72);
  color: rgb(var(--app-shell-text-muted));
  font-family: var(--app-font-mono);
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.app-error-state__recovery-note::before {
  width: 22px;
  height: 3px;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    rgb(var(--app-error-color)),
    rgb(var(--app-accent-butter))
  );
  content: '';
}

@media (max-width: 780px) {
  .app-error-state {
    grid-template-columns: 1fr;
    min-height: 0;
    max-width: 620px;
  }

  .app-error-state__visual {
    min-height: 310px;
    padding: 52px 32px 34px;
    border-right: 0;
    border-bottom: 1px dashed rgba(var(--app-shell-border), 0.72);
  }

  .app-error-state__image-frame {
    width: min(62vw, 250px);
  }

  .app-error-state__code {
    right: max(24px, calc(50% - 158px));
    bottom: 24px;
  }

  .app-error-state__content {
    gap: 24px;
    padding: 34px 32px 38px;
  }
}

@media (max-width: 480px) {
  .app-error-state__visual {
    min-height: 270px;
    padding: 48px 24px 28px;
  }

  .app-error-state__signal {
    top: 21px;
    left: 22px;
  }

  .app-error-state__image-frame {
    width: min(68vw, 214px);
    border-radius: 38px 26px 42px 30px;
  }

  .app-error-state__code {
    right: 16px;
    bottom: 18px;
  }

  .app-error-state__content {
    padding: 30px 22px 32px;
  }

  .app-error-state__title {
    font-size: clamp(2.15rem, 12vw, 2.7rem);
  }

  .app-error-state__actions {
    display: grid;
  }

  .app-error-state__actions :deep(.v-btn) {
    width: 100%;
  }
}
</style>
