<script setup lang="ts">
import type { DashboardScope, UsageWindow } from '~/types/user-service'

const props = defineProps<{
  scope: DashboardScope
  window: UsageWindow
}>()

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/top-provider-types'
    : '/api/v1/me/dashboard/top-provider-types',
)
</script>

<template>
  <DashboardBreakdownWidget
    :window="window"
    :endpoint="endpoint"
    chart-type="pie"
    icon="mdi-shape-outline"
    title="Top provider types"
    subtitle="By token volume"
    empty-label="No provider type usage yet"
  />
</template>
