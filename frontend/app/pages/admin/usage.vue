<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'Usage & costs',
  icon: 'mdi-chart-line',
  drawerIndex: 1,
  drawerSection: 'Dashboards',
})
</script>

<template>
  <AdminUsageDashboard />
</template>
