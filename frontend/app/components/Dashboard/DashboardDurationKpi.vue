<script setup lang="ts">
import type {
  DashboardScope,
  DashboardDurationResponse,
  UsageWindow,
} from '~/types/user-service'
import { formatDurationMs } from '~/utils/formatters'

const props = defineProps<{
  scope: DashboardScope
  window: UsageWindow
}>()

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/duration'
    : '/api/v1/me/dashboard/duration',
)
const widget = useDashboardWidget<DashboardDurationResponse>(
  endpoint,
  () => props.window,
)

const totalDurationMs = computed(
  () => widget.data.value?.totalDurationMs ?? null,
)
</script>

<template>
  <DashboardKpiCard
    icon="mdi-timer-outline"
    color="success"
    title="Total duration"
    :value="totalDurationMs"
    :formatter="formatDurationMs"
    caption="Completed requests"
    :loading="widget.loading.value"
    :error="widget.error.value"
    @retry="widget.reload"
  />
</template>
