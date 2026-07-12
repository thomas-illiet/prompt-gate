<script setup lang="ts">
definePageMeta({
  icon: 'mdi-account-group-outline',
  title: 'Administration',
  drawerIndex: 6,
  drawerGroup: true,
  requiredRoles: ['admin'],
  redirect: '/admin/users',
})
</script>

<template>
  <NuxtPage />
</template>
