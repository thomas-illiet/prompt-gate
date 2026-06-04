<script setup lang="ts">
import type {
  DashboardScope,
  DashboardTokensResponse,
  UsageWindow,
} from '~/types/user-service'
import { formatCurrencyUsd } from '~/utils/formatters'

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

const estimatedCost = computed(
  () => widget.data.value?.estimatedCost?.totalUsd ?? null,
)
const caption = computed(() => {
  if (widget.data.value && !widget.data.value.estimatedCost) {
    return 'Cost estimate unavailable'
  }

  return props.scope === 'global'
    ? 'Global usage estimate'
    : 'User usage estimate'
})
</script>

<template>
  <DashboardKpiCard
    icon="mdi-currency-usd"
    color="info"
    title="Estimated cost"
    :value="estimatedCost"
    :formatter="formatCurrencyUsd"
    :caption="caption"
    :animate="false"
    :loading="widget.loading.value"
    :error="widget.error.value"
    @retry="widget.reload"
  />
</template>
