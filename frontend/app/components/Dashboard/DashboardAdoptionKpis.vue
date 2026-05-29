<script setup lang="ts">
import type {
  DashboardAdoptionResponse,
  UsageWindow,
} from '~/types/user-service'
import { formatNumber } from '~/utils/formatters'

const props = defineProps<{
  window: UsageWindow
}>()

const widget = useDashboardWidget<DashboardAdoptionResponse>(
  '/api/v1/admin/dashboard/adoption',
  () => props.window,
)

const activeUsers = computed(() => widget.data.value?.activeUsers ?? null)
const activeServiceAccounts = computed(
  () => widget.data.value?.activeServiceAccounts ?? null,
)
const activeVirtualKeys = computed(
  () => widget.data.value?.activeVirtualKeys ?? null,
)
</script>

<template>
  <v-row>
    <v-col cols="12" md="4">
      <DashboardKpiCard
        icon="mdi-account-check-outline"
        color="info"
        title="Active users"
        :value="activeUsers"
        :formatter="formatNumber"
        caption="Used this window"
        :loading="widget.loading.value"
        :error="widget.error.value"
        @retry="widget.reload"
      />
    </v-col>
    <v-col cols="12" md="4">
      <DashboardKpiCard
        icon="mdi-account-cog-outline"
        color="secondary"
        title="Active services"
        :value="activeServiceAccounts"
        :formatter="formatNumber"
        caption="Used this window"
        :loading="widget.loading.value"
        :error="widget.error.value"
        @retry="widget.reload"
      />
    </v-col>
    <v-col cols="12" md="4">
      <DashboardKpiCard
        icon="mdi-key-chain"
        color="primary"
        title="Active keys"
        :value="activeVirtualKeys"
        :formatter="formatNumber"
        caption="Valid now"
        :loading="widget.loading.value"
        :error="widget.error.value"
        @retry="widget.reload"
      />
    </v-col>
  </v-row>
</template>
