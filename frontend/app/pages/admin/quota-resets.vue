<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'Quotas & resets',
  icon: 'mdi-timer-refresh-outline',
  drawerIndex: 3,
  drawerSection: 'Dashboards',
})
</script>

<template>
  <AdminQuotaResetsDashboard />
</template>
