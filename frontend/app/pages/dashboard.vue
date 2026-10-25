<script setup lang="ts">
definePageMeta({
  icon: 'mdi-monitor-dashboard',
  title: 'Dashboard',
  drawerIndex: 0,
  requiredRoles: ['limited', 'user', 'admin'],
})
</script>

<template>
  <DashboardCockpit scope="self" />
</template>
