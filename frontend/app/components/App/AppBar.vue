<script setup lang="ts">
const { contextLabel = 'Green room' } = defineProps<{
  contextLabel?: string
}>()

const drawer = useState('drawer')
const route = useRoute()
const breadcrumbs = computed(() => {
  return route!.matched
    .filter((item) => item.meta && item.meta.title)
    .map((r) => ({
      title: r.meta.title!,
      disabled: r.path === route.path || false,
      to: r.path,
    }))
})
const currentTitle = computed(() =>
  String(breadcrumbs.value.at(-1)?.title ?? 'PromptGate'),
)
</script>

<template>
  <v-app-bar flat height="64" class="app-shell-bar">
    <div class="app-shell-bar__inner">
      <v-app-bar-nav-icon
        aria-label="Toggle navigation"
        rounded="lg"
        class="app-shell-bar__nav"
        data-test="navigation-toggle"
        @click="drawer = !drawer"
      />

      <div class="app-shell-bar__context">
        <div class="app-shell-bar__identity">
          <span class="app-shell-bar__eyebrow">
            <span class="app-shell-bar__beacon" aria-hidden="true" />
            {{ contextLabel }}
          </span>
        </div>
        <v-breadcrumbs :items="breadcrumbs" class="app-shell-breadcrumbs">
          <template #divider>
            <v-icon icon="mdi-chevron-right" size="14" />
          </template>
        </v-breadcrumbs>
        <span class="app-shell-bar__mobile-title">{{ currentTitle }}</span>
      </div>

      <v-spacer />
      <div id="app-bar" class="app-shell-bar__actions" />
      <AppUserMenu />
    </div>
  </v-app-bar>
</template>

<style scoped>
.app-shell-bar__inner {
  display: flex;
  width: 100%;
  height: 100%;
  align-items: center;
  padding-inline: 10px 8px;
}

.app-shell-bar__nav {
  margin-inline-end: 10px;
  border: 1px solid rgba(var(--app-accent-butter), 0.44);
  background:
    radial-gradient(
      circle at 28% 24%,
      rgba(var(--app-accent-butter), 0.28),
      transparent 55%
    ),
    rgba(var(--app-shell-surface-strong), 0.78);
  color: rgb(var(--app-shell-text-secondary));
  box-shadow: 3px 4px 0 rgba(var(--app-accent-coral), 0.08);
}

.app-shell-bar__context {
  display: grid;
  align-content: center;
  gap: 1px;
  min-width: 0;
}

.app-shell-bar__identity {
  display: flex;
  align-items: center;
}

.app-shell-bar__eyebrow {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-inline-start: 12px;
  color: rgb(var(--app-shell-text-muted));
  font-family: var(--app-font-mono);
  font-size: 0.58rem;
  font-weight: 800;
  letter-spacing: 0.1em;
  line-height: 1;
  text-transform: uppercase;
}

.app-shell-bar__beacon {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgb(var(--v-theme-primary));
  box-shadow: 0 0 0 3px rgba(var(--v-theme-primary), 0.1);
}

.app-shell-breadcrumbs {
  min-height: 25px;
  margin: 0;
  padding: 0 10px;
  color: rgb(var(--app-shell-text-secondary));
}

:deep(.app-shell-breadcrumbs .v-breadcrumbs-item) {
  padding: 0 2px;
  font-size: 0.84rem;
  font-weight: 680;
}

:deep(.app-shell-breadcrumbs .v-breadcrumbs-item--disabled) {
  color: rgb(var(--v-theme-primary));
  opacity: 1;
}

:deep(.app-shell-breadcrumbs .v-breadcrumbs-divider) {
  padding: 0 2px;
  color: rgb(var(--app-shell-text-muted));
  opacity: 0.7;
}

.app-shell-bar__mobile-title {
  display: none;
  padding-inline: 12px;
  overflow: hidden;
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.88rem;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-shell-bar__actions {
  display: flex;
  align-items: center;
}

@media (max-width: 600px) {
  .app-shell-bar__inner {
    padding-inline: 7px 4px;
  }

  .app-shell-bar__nav {
    margin-inline-end: 5px;
  }

  .app-shell-bar__eyebrow,
  .app-shell-breadcrumbs {
    display: none;
  }

  .app-shell-bar__mobile-title {
    display: block;
  }
}
</style>
