<script setup lang="ts">
import type {
  DashboardScope,
  DashboardMessagesResponse,
  UsageWindow,
} from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  scope: DashboardScope
  window: UsageWindow
}>()

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/messages'
    : '/api/v1/me/dashboard/messages',
)
const widget = useDashboardWidget<DashboardMessagesResponse>(
  endpoint,
  () => props.window,
)

const messages = computed(() => widget.data.value?.messages ?? null)
</script>

<template>
  <DashboardKpiCard
    icon="mdi-message-text-outline"
    color="primary"
    title="Messages"
    :value="messages"
    :formatter="formatNumber"
    caption="Requests handled"
    :loading="widget.loading.value"
    :error="widget.error.value"
    @retry="widget.reload"
  />
</template>
