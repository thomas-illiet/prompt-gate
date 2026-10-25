<script setup lang="ts">
import { useId } from 'vue'

import type { AppSignalVisualName } from '~/types/theme'

const props = withDefaults(
  defineProps<{
    copy: string
    density?: 'compact' | 'standard'
    icon?: string
    kicker: string
    statBusy?: boolean
    statDetail?: string
    statLabel?: string
    statValue?: string
    title: string
    visual?: AppSignalVisualName
  }>(),
  {
    density: 'standard',
    icon: undefined,
    statBusy: false,
    statDetail: undefined,
    statLabel: undefined,
    statValue: undefined,
    visual: undefined,
  },
)

defineSlots<{
  footer?(): unknown
  meta?(): unknown
}>()

const headingId = useId()
const hasStat = computed(() =>
  Boolean(props.statLabel || props.statValue || props.statDetail),
)
const hasInstrument = computed(() => hasStat.value || Boolean(props.visual))
</script>

<template>
  <v-card
    class="app-page-header"
    :class="`app-page-header--${props.density}`"
    rounded="lg"
    tag="header"
    :aria-labelledby="headingId"
  >
    <div
      class="app-page-header__main"
      :class="{ 'app-page-header__main--single': !hasInstrument }"
    >
      <div class="app-page-header__intro">
        <p class="app-page-header__kicker">{{ props.kicker }}</p>

        <div class="app-page-header__heading">
          <v-avatar
            v-if="props.icon"
            class="app-page-header__icon"
            color="primary"
            size="44"
            variant="tonal"
          >
            <v-icon :icon="props.icon" size="25" />
          </v-avatar>

          <div class="app-page-header__content">
            <h1 :id="headingId">{{ props.title }}</h1>
            <p class="app-page-header__copy">{{ props.copy }}</p>
          </div>
        </div>

        <div v-if="$slots.meta" class="app-page-header__meta">
          <slot name="meta" />
        </div>
      </div>

      <div
        v-if="hasInstrument"
        class="app-page-header__instrument"
        :class="{
          'app-page-header__instrument--stat-only': hasStat && !props.visual,
          'app-page-header__instrument--visual-only': !hasStat && props.visual,
        }"
      >
        <div
          v-if="hasStat"
          class="app-page-header__stat"
          data-test="app-page-header-stat"
          role="status"
          aria-live="polite"
          :aria-busy="props.statBusy"
        >
          <span v-if="props.statLabel" class="app-page-header__stat-label">
            {{ props.statLabel }}
          </span>
          <strong v-if="props.statValue">{{ props.statValue }}</strong>
          <span v-if="props.statDetail" class="app-page-header__stat-detail">
            {{ props.statDetail }}
          </span>
        </div>

        <div
          v-if="props.visual"
          class="app-page-header__visual"
          data-test="app-page-header-visual"
          aria-hidden="true"
        >
          <AppSignalVisual :name="props.visual" />
        </div>
      </div>
    </div>

    <div v-if="$slots.footer" class="app-page-header__footer">
      <slot name="footer" />
    </div>
  </v-card>
</template>

<style scoped>
.app-page-header {
  position: relative;
  isolation: isolate;
  container-type: inline-size;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    radial-gradient(
      circle at 91% 0,
      rgba(var(--app-accent-coral), 0.12),
      transparent 30%
    ),
    linear-gradient(
      135deg,
      rgba(var(--app-shell-surface-strong), 0.99),
      rgba(var(--app-shell-surface-muted), 0.9)
    );
  box-shadow: var(--app-card-shadow-soft);
}

.app-page-header::before {
  position: absolute;
  z-index: 1;
  top: 0;
  left: 24px;
  width: 96px;
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

.app-page-header__main {
  position: relative;
  display: grid;
  align-items: center;
  grid-template-columns: minmax(0, 1fr) minmax(250px, 310px);
  gap: clamp(28px, 5vw, 72px);
  padding: 22px 24px 18px;
}

.app-page-header__main::after {
  position: absolute;
  z-index: -1;
  top: -138px;
  right: -72px;
  width: 270px;
  height: 270px;
  border: 1px solid rgba(var(--v-theme-primary), 0.12);
  border-radius: 50%;
  box-shadow:
    0 0 0 32px rgba(var(--v-theme-primary), 0.035),
    0 0 0 66px rgba(var(--app-accent-coral), 0.025);
  content: '';
  pointer-events: none;
}

.app-page-header__main--single {
  grid-template-columns: minmax(0, 1fr);
}

.app-page-header__intro,
.app-page-header__content {
  min-width: 0;
}

.app-page-header__kicker {
  margin: 0 0 12px;
  color: rgb(var(--app-shell-text-muted));
  font-family: var(--app-font-mono);
  font-size: 0.72rem;
  font-weight: 750;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.app-page-header__heading {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.app-page-header__icon {
  flex: 0 0 auto;
  border: 1px solid rgba(var(--v-theme-primary), 0.2);
  border-radius: 15px !important;
  background: rgba(var(--v-theme-primary), 0.1) !important;
  box-shadow: 4px 5px 0 rgba(var(--app-accent-coral), 0.12);
  transform: rotate(-2deg);
}

.app-page-header__icon :deep(.v-icon) {
  transform: rotate(2deg);
}

.app-page-header__content h1 {
  margin: 1px 0 0;
  color: rgb(var(--app-shell-text-primary));
  font-family: var(--app-font-display);
  font-size: clamp(1.55rem, 2.3vw, 2rem);
  font-weight: 800;
  letter-spacing: -0.035em;
  line-height: 1.08;
}

.app-page-header__copy {
  max-width: 46rem;
  margin: 7px 0 0;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.98rem;
  line-height: 1.5;
}

.app-page-header__meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0 0 58px;
}

.app-page-header__instrument {
  display: grid;
  align-items: center;
  grid-template-columns: minmax(0, 1fr) 104px;
  gap: 12px;
  min-width: 0;
  padding: 12px;
  border: 1px solid rgba(var(--v-theme-primary), 0.18);
  border-radius: calc(var(--app-card-radius) - 2px);
  background: linear-gradient(
    145deg,
    rgba(var(--v-theme-primary), 0.09),
    rgba(var(--app-shell-surface-strong), 0.82)
  );
  box-shadow:
    inset 0 1px 0 rgba(var(--app-shell-surface-strong), 0.72),
    5px 6px 0 rgba(var(--app-accent-butter), 0.08);
}

.app-page-header__instrument--stat-only,
.app-page-header__instrument--visual-only {
  grid-template-columns: 1fr;
}

.app-page-header__instrument--visual-only {
  width: 128px;
  justify-self: end;
}

.app-page-header__stat {
  display: grid;
  align-content: center;
  gap: 2px;
  min-width: 0;
  padding-left: 4px;
}

.app-page-header__stat-label {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.72rem;
  font-weight: 750;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.app-page-header__stat strong {
  overflow-wrap: anywhere;
  color: rgb(var(--app-shell-text-primary));
  font-family: var(--app-font-display);
  font-size: clamp(1.25rem, 2.4vw, 1.75rem);
  line-height: 1.15;
}

.app-page-header__stat-detail {
  overflow: hidden;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.72rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-page-header__visual {
  width: 104px;
  height: 104px;
}

.app-page-header__footer {
  border-top: 1px solid rgba(var(--app-shell-border), 0.3);
  background: rgba(var(--app-shell-surface-strong), 0.32);
}

.app-page-header--compact .app-page-header__main {
  grid-template-columns: minmax(0, 1fr) minmax(112px, 180px);
  gap: 20px;
  padding: 16px 18px;
}

.app-page-header--compact .app-page-header__kicker {
  margin-bottom: 8px;
}

.app-page-header--compact .app-page-header__content h1 {
  font-size: clamp(1.35rem, 2vw, 1.65rem);
}

.app-page-header--compact .app-page-header__copy {
  margin-top: 5px;
  font-size: 0.9rem;
}

.app-page-header--compact .app-page-header__visual {
  width: 76px;
  height: 76px;
  margin-inline: auto;
}

@container (max-width: 760px) {
  .app-page-header__main {
    grid-template-columns: minmax(0, 1fr) minmax(220px, 250px);
    gap: 20px;
  }

  .app-page-header__visual {
    width: 84px;
    height: 84px;
  }

  .app-page-header__instrument {
    grid-template-columns: minmax(0, 1fr) 84px;
  }

  .app-page-header__instrument--stat-only,
  .app-page-header__instrument--visual-only {
    grid-template-columns: 1fr;
  }
}

@container (max-width: 560px) {
  .app-page-header__main,
  .app-page-header--compact .app-page-header__main {
    grid-template-columns: 1fr;
    gap: 18px;
    padding: 18px 16px 16px;
  }

  .app-page-header__meta {
    margin-left: 0;
  }

  .app-page-header__instrument,
  .app-page-header__instrument--stat-only,
  .app-page-header__instrument--visual-only {
    width: auto;
    grid-template-columns: minmax(0, 1fr) 76px;
    justify-self: stretch;
    padding: 10px 12px;
  }

  .app-page-header__instrument--stat-only {
    grid-template-columns: 1fr;
  }

  .app-page-header__visual,
  .app-page-header--compact .app-page-header__visual {
    width: 76px;
    height: 76px;
  }
}

@container (max-width: 380px) {
  .app-page-header::before {
    left: 16px;
    width: 72px;
  }

  .app-page-header__heading {
    gap: 11px;
  }

  .app-page-header__icon {
    width: 40px !important;
    height: 40px !important;
  }

  .app-page-header__instrument,
  .app-page-header__instrument--visual-only {
    grid-template-columns: minmax(0, 1fr) 64px;
  }

  .app-page-header__instrument--stat-only {
    grid-template-columns: 1fr;
  }

  .app-page-header__visual,
  .app-page-header--compact .app-page-header__visual {
    width: 64px;
    height: 64px;
  }
}
</style>
