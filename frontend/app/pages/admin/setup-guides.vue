<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'Setup guides',
  icon: 'mdi-book-cog-outline',
  drawerIndex: 11,
  drawerSection: 'Configuration',
})
</script>
<template>
  <v-container fluid class="app-page"><AdminSetupGuidesManager /></v-container>
</template>
