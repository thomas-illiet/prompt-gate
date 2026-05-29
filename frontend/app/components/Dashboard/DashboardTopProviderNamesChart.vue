<script setup lang="ts">
import type { DashboardScope, UsageWindow } from '~/types/user-service'

const props = defineProps<{
  scope: DashboardScope
  window: UsageWindow
}>()

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/top-provider-names'
    : '/api/v1/me/dashboard/top-provider-names',
)
</script>

<template>
  <DashboardBreakdownWidget
    :window="window"
    :endpoint="endpoint"
    chart-type="pie"
    icon="mdi-cloud-outline"
    title="Top provider names"
    subtitle="By token volume"
    empty-label="No provider usage yet"
  />
</template>
