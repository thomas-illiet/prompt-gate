<script setup lang="ts">
import type {
  DashboardScope,
  DashboardTokensResponse,
  UsageWindow,
} from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  scope: DashboardScope
  window: UsageWindow
}>()

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/tokens'
    : '/api/v1/me/dashboard/tokens',
)
const widget = useDashboardWidget<DashboardTokensResponse>(
  endpoint,
  () => props.window,
)

const totalTokens = computed(() => widget.data.value?.totalTokens ?? null)
</script>

<template>
  <DashboardKpiCard
    icon="mdi-counter"
    color="warning"
    title="Total tokens"
    :value="totalTokens"
    :formatter="formatNumber"
    :loading="widget.loading.value"
    :error="widget.error.value"
    @retry="widget.reload"
  />
</template>
