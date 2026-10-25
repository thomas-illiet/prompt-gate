<script setup lang="ts">
import AdminActivityDashboard from '~/components/AdminDashboards/AdminActivityDashboard.vue'

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
