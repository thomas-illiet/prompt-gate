<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'Global dashboard',
  icon: 'mdi-monitor-dashboard',
  drawerIndex: 0,
})
</script>

<template>
  <DashboardCockpit scope="global" />
</template>
