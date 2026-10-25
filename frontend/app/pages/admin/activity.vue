<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'Activity & adoption',
  icon: 'mdi-account-group-outline',
  drawerIndex: 2,
  drawerSection: 'Dashboards',
})
</script>

<template>
  <AdminActivityDashboard />
</template>
