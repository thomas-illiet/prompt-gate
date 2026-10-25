<script setup lang="ts">
import AdminUsageDashboard from '~/components/AdminDashboards/AdminUsageDashboard.vue'

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
