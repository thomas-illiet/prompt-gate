<script setup lang="ts">
import type { UsageWindow } from '~/types/user-service'

defineProps<{
  window: UsageWindow
}>()
</script>

<template>
  <DashboardBreakdownWidget
    :window="window"
    endpoint="/api/v1/admin/dashboard/top-identities"
    icon="mdi-account-multiple-outline"
    title="Top identities"
    subtitle="Users and service accounts"
    empty-label="No identity usage yet"
    :show-setup-action="false"
  />
</template>
