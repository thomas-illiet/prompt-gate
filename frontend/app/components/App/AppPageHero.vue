<script setup lang="ts">
import type { AppSignalVisualName } from '~/types/theme'

const props = withDefaults(
  defineProps<{
    copy: string
    icon?: string
    kicker: string
    statLabel: string
    statValue: string
    title: string
    visual?: AppSignalVisualName
  }>(),
  {
    icon: undefined,
    visual: undefined,
  },
)
</script>

<template>
  <v-card rounded="lg" class="app-page-hero">
    <v-card-text class="pa-4 pa-md-5">
      <p class="app-page-hero__kicker">{{ props.kicker }}</p>
      <div
        class="app-page-hero__layout"
        :class="{ 'app-page-hero__layout--visual': props.visual }"
      >
        <div class="app-page-hero__header">
          <div class="app-page-hero__title">
            <v-avatar
              v-if="props.icon"
              color="primary"
              variant="tonal"
              size="40"
              class="app-page-hero__icon"
            >
              <v-icon :icon="props.icon" size="23" />
            </v-avatar>

            <div class="app-page-hero__content">
              <h1 class="app-page-hero__headline font-weight-bold">
                {{ props.title }}
              </h1>
              <p class="app-page-hero__copy">
                {{ props.copy }}
              </p>
            </div>
          </div>

          <div class="app-page-hero__stat">
            <span>{{ props.statLabel }}</span>
            <strong>{{ props.statValue }}</strong>
          </div>
        </div>

        <div
          v-if="props.visual"
          class="app-page-hero__visual"
          data-test="app-page-hero-visual"
          aria-hidden="true"
        >
          <AppSignalVisual :name="props.visual" />
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.app-page-hero {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    radial-gradient(
      circle at 92% 8%,
      rgba(var(--app-accent-coral), 0.12),
      transparent 28%
    ),
    linear-gradient(
      90deg,
      rgba(var(--v-theme-primary), 0.12) 0,
      rgba(var(--v-theme-primary), 0) 5px
    ),
    linear-gradient(
      180deg,
      rgba(var(--app-shell-surface-strong), 0.98) 0%,
      rgba(var(--app-shell-surface), 0.98) 58%,
      rgba(var(--app-shell-surface-muted), 0.82) 100%
    );
  box-shadow: var(--app-card-shadow-soft);
}

.app-page-hero::before {
  position: absolute;
  z-index: -1;
  top: -96px;
  right: -72px;
  width: 230px;
  height: 230px;
  border: 1px solid rgba(var(--v-theme-primary), 0.12);
  border-radius: 50%;
  box-shadow:
    0 0 0 28px rgba(var(--v-theme-primary), 0.035),
    0 0 0 58px rgba(var(--app-accent-coral), 0.025);
  content: '';
  pointer-events: none;
  animation: app-float 9s ease-in-out infinite;
}

.app-page-hero__kicker {
  margin: 0 0 8px;
  font-family: var(--app-font-mono);
  text-transform: uppercase;
  letter-spacing: 0.14em;
  font-size: 0.72rem;
  color: rgb(var(--app-shell-text-muted));
}

.app-page-hero__layout--visual {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 104px;
  align-items: center;
  gap: 20px;
}

.app-page-hero__header {
  display: flex;
  justify-content: space-between;
  gap: 18px;
}

.app-page-hero__title {
  min-width: 0;
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.app-page-hero__icon {
  flex: 0 0 auto;
}

.app-page-hero__content {
  min-width: 0;
}

.app-page-hero__headline {
  margin: 0;
  font-family: var(--app-font-display);
  font-size: clamp(1.3rem, 2vw, 1.55rem);
  line-height: 1.15;
  letter-spacing: -0.025em;
}

.app-page-hero__copy {
  margin: 6px 0 0;
  max-width: 42rem;
  line-height: 1.5;
  color: rgb(var(--app-shell-text-secondary));
}

.app-page-hero__stat {
  min-width: 150px;
  display: grid;
  gap: 6px;
  align-content: start;
  padding: 12px 14px;
  border: 1px solid rgba(var(--v-theme-primary), 0.16);
  border-radius: var(--app-card-radius);
  background:
    linear-gradient(
      rgba(var(--v-theme-primary), 0.07),
      rgba(var(--v-theme-primary), 0.07)
    ),
    rgb(var(--app-shell-surface));
  box-shadow: inset 0 1px 0 rgba(var(--app-shell-surface-strong), 0.68);
}

.app-page-hero__stat span {
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(var(--app-shell-text-muted));
}

.app-page-hero__stat strong {
  font-family: var(--app-font-display);
  font-size: 1.15rem;
}

.app-page-hero__visual {
  position: relative;
  width: 104px;
  height: 104px;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.6);
  border-radius: calc(var(--app-card-radius) - 2px);
  background:
    radial-gradient(
      circle at 70% 20%,
      rgba(var(--app-accent-coral), 0.12),
      transparent 42%
    ),
    linear-gradient(
      rgba(var(--v-theme-primary), 0.055),
      rgba(var(--v-theme-primary), 0.055)
    ),
    rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
  padding: 10px;
}

@media (max-width: 960px) {
  .app-page-hero__header {
    flex-direction: column;
  }

  .app-page-hero__layout--visual {
    grid-template-columns: minmax(0, 1fr) 92px;
  }

  .app-page-hero__visual {
    width: 92px;
    height: 92px;
  }
}

@media (max-width: 720px) {
  .app-page-hero :deep(.v-card-text) {
    padding: 16px !important;
  }

  .app-page-hero__kicker {
    margin-bottom: 8px;
  }

  .app-page-hero__layout--visual {
    grid-template-columns: minmax(0, 1fr) 72px;
    gap: 12px;
  }

  .app-page-hero__title {
    gap: 12px;
  }

  .app-page-hero__icon {
    width: 40px !important;
    height: 40px !important;
  }

  .app-page-hero__copy {
    margin-top: 6px;
    line-height: 1.5;
  }

  .app-page-hero__stat {
    min-width: 0;
    padding: 12px 14px;
  }

  .app-page-hero__visual {
    width: 72px;
    height: 72px;
    padding: 6px;
  }
}

@media (max-width: 600px) {
  .app-page-hero__layout--visual {
    display: block;
  }

  .app-page-hero__visual {
    display: none;
  }
}
</style>
