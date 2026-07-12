<script setup lang="ts">
const props = defineProps<{
  copy: string
  icon?: string
  kicker: string
  statLabel: string
  statValue: string
  title: string
}>()
</script>

<template>
  <v-card rounded="lg" class="app-page-hero">
    <v-card-text class="pa-6 pa-md-8">
      <p class="app-page-hero__kicker">{{ props.kicker }}</p>
      <div class="app-page-hero__header">
        <div class="app-page-hero__title">
          <v-avatar
            v-if="props.icon"
            color="primary"
            variant="tonal"
            size="48"
            class="app-page-hero__icon"
          >
            <v-icon :icon="props.icon" size="28" />
          </v-avatar>

          <div class="app-page-hero__content">
            <h1 class="app-page-hero__headline text-h4 font-weight-bold">
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
    </v-card-text>
  </v-card>
</template>

<style scoped>
.app-page-hero {
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    linear-gradient(
      90deg,
      rgba(var(--v-theme-primary), 0.08) 0,
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

.app-page-hero__kicker {
  margin: 0 0 12px;
  text-transform: uppercase;
  letter-spacing: 0.14em;
  font-size: 0.8rem;
  color: rgb(var(--app-shell-text-muted));
}

.app-page-hero__header {
  display: flex;
  justify-content: space-between;
  gap: 24px;
}

.app-page-hero__title {
  min-width: 0;
  display: flex;
  gap: 16px;
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
  line-height: 1.2;
}

.app-page-hero__copy {
  margin: 10px 0 0;
  max-width: 42rem;
  line-height: 1.7;
  color: rgb(var(--app-shell-text-secondary));
}

.app-page-hero__stat {
  min-width: 180px;
  display: grid;
  gap: 6px;
  align-content: start;
  padding: 18px 20px;
  border: 1px solid rgba(var(--v-theme-primary), 0.16);
  border-radius: var(--app-card-radius);
  background: rgba(var(--v-theme-primary), 0.07);
}

.app-page-hero__stat span {
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(var(--app-shell-text-muted));
}

.app-page-hero__stat strong {
  font-size: 1.4rem;
}

@media (max-width: 960px) {
  .app-page-hero__header {
    flex-direction: column;
  }
}

@media (max-width: 720px) {
  .app-page-hero :deep(.v-card-text) {
    padding: 18px !important;
  }

  .app-page-hero__kicker {
    margin-bottom: 8px;
  }

  .app-page-hero__title {
    gap: 12px;
  }

  .app-page-hero__icon {
    width: 40px !important;
    height: 40px !important;
  }

  .app-page-hero__headline {
    font-size: 1.45rem !important;
  }

  .app-page-hero__copy {
    margin-top: 6px;
    line-height: 1.5;
  }

  .app-page-hero__stat {
    min-width: 0;
    padding: 12px 14px;
  }
}
</style>
