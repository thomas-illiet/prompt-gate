<script setup lang="ts">
definePageMeta({
  icon: 'mdi-account-group-outline',
  title: 'Administration',
  drawerIndex: 5,
  drawerGroup: true,
  requiredRoles: ['admin'],
  redirect: '/admin/users',
})
</script>

<template>
  <NuxtPage />
</template>
