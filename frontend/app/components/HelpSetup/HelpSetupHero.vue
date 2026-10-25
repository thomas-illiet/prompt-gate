<script setup lang="ts">
const props = defineProps<{
  loading: boolean
  providerCount: number
}>()

const providerCountLabel = computed(() =>
  props.loading ? '…' : String(props.providerCount),
)
const providerStatus = computed(() => {
  if (props.loading) return 'Checking your routes'
  if (props.providerCount === 0) return 'Waiting for access'
  if (props.providerCount === 1) return '1 route ready'
  return `${props.providerCount} routes ready`
})
</script>

<template>
  <AppPageHeader
    copy="Choose a route and we’ll wire the right snippet to your PromptGate endpoint."
    icon="mdi-help-circle-outline"
    kicker="Launch sequence"
    :stat-busy="props.loading"
    :stat-detail="providerStatus"
    stat-label="Providers"
    :stat-value="providerCountLabel"
    title="Setup guide"
    visual="cables"
  >
    <template #meta>
      <div class="help-setup-hero__formats" aria-label="Supported setups">
        <span><v-icon icon="mdi-console-line" size="16" />CLI & scripts</span>
        <span><v-icon icon="mdi-code-braces" size="16" />SDKs</span>
        <span><v-icon icon="mdi-robot-outline" size="16" />Coding agents</span>
      </div>
    </template>

    <template #footer>
      <ol class="help-setup-hero__flow" aria-label="Setup flow">
        <li>
          <span>1</span>
          <v-icon icon="mdi-server-network" size="17" />
          <strong>Provider</strong>
        </li>
        <li>
          <span>2</span>
          <v-icon icon="mdi-cube-outline" size="17" />
          <strong>Model</strong>
        </li>
        <li>
          <span>3</span>
          <v-icon icon="mdi-rocket-launch-outline" size="17" />
          <strong>Client</strong>
        </li>
      </ol>
    </template>
  </AppPageHeader>
</template>

<style scoped>
.help-setup-hero__formats {
  display: flex;
  width: 100%;
  flex-wrap: wrap;
  gap: 8px;
}

.help-setup-hero__formats span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 9px;
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  border-radius: 999px;
  background: rgba(var(--app-shell-surface-strong), 0.58);
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.75rem;
  font-weight: 700;
}

.help-setup-hero__flow {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0;
  padding: 0 24px;
  list-style: none;
}

.help-setup-hero__flow li {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-width: 0;
  padding: 11px 14px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.76rem;
}

.help-setup-hero__flow li:not(:last-child)::after {
  position: absolute;
  top: 50%;
  right: -5px;
  width: 10px;
  height: 10px;
  border-top: 1px solid rgba(var(--v-theme-primary), 0.4);
  border-right: 1px solid rgba(var(--v-theme-primary), 0.4);
  content: '';
  transform: translateY(-50%) rotate(45deg);
}

.help-setup-hero__flow li > span {
  display: grid;
  width: 20px;
  height: 20px;
  place-items: center;
  border-radius: 7px;
  background: rgba(var(--v-theme-primary), 0.11);
  color: rgb(var(--v-theme-primary));
  font-family: var(--app-font-mono);
  font-size: 0.66rem;
  font-weight: 800;
}

.help-setup-hero__flow strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@container (max-width: 560px) {
  .help-setup-hero__flow {
    padding: 0 8px;
  }
}

@container (max-width: 380px) {
  .help-setup-hero__formats {
    gap: 6px;
  }

  .help-setup-hero__formats span {
    padding: 4px 7px;
    font-size: 0.7rem;
  }

  .help-setup-hero__flow {
    padding: 0;
  }

  .help-setup-hero__flow li {
    gap: 5px;
    padding: 10px 6px;
  }

  .help-setup-hero__flow li > span {
    width: 18px;
    height: 18px;
  }

  .help-setup-hero__flow :deep(.v-icon) {
    display: none;
  }
}
</style>
