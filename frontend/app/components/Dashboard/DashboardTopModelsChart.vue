<script setup lang="ts">
import type { DashboardScope, UsageWindow } from '~/types/user-service'

const props = defineProps<{
  scope: DashboardScope
  window: UsageWindow
}>()

const endpoint = computed(() =>
  props.scope === 'global'
    ? '/api/v1/admin/dashboard/top-models'
    : '/api/v1/me/dashboard/top-models',
)
</script>

<template>
  <DashboardBreakdownWidget
    :window="window"
    :endpoint="endpoint"
    icon="mdi-cube-outline"
    title="Top models"
    subtitle="By token volume"
    empty-label="No model usage yet"
  />
</template>
