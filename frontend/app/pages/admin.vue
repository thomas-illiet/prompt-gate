<script setup lang="ts">
definePageMeta({
  layout: 'admin',
  title: 'Administration',
  requiredRoles: ['admin'],
  redirect: '/admin/dashboard',
})
</script>

<template>
  <NuxtPage />
</template>
