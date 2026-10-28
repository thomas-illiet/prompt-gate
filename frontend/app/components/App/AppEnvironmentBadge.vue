<script setup lang="ts">
import type { InstanceEnvironmentState } from '~/types/instance'
import { instancePresentation } from '~/utils/instance'

const props = defineProps<{
  environment: InstanceEnvironmentState
  subtle?: boolean
}>()

const presentation = computed(() => instancePresentation(props.environment))
const badgeStyle = computed(() => ({
  '--environment-accent': presentation.value.accent,
  '--environment-surface': presentation.value.surface,
  '--environment-text': presentation.value.text,
}))
</script>

<template>
  <span
    class="app-environment-badge"
    :class="{ 'app-environment-badge--subtle': props.subtle }"
    :data-environment="props.environment"
    :style="badgeStyle"
  >
    <span class="app-environment-badge__dot" aria-hidden="true" />
    {{ presentation.label }}
  </span>
</template>

<style scoped>
.app-environment-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 9px;
  border-radius: 999px;
  background: var(--environment-surface);
  color: var(--environment-text);
  font-size: 0.68rem;
  font-weight: 850;
  letter-spacing: 0.08em;
  line-height: 1.2;
  text-transform: uppercase;
  white-space: nowrap;
}

.app-environment-badge__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--environment-accent);
}

.app-environment-badge--subtle {
  gap: 5px;
  padding: 0;
  background: transparent;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.58rem;
  font-weight: 780;
  letter-spacing: 0.1em;
}

.app-environment-badge--subtle .app-environment-badge__dot {
  width: 6px;
  height: 6px;
}
</style>
