<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'Setup guides',
  icon: 'mdi-book-cog-outline',
  drawerIndex: 2,
  drawerSection: 'Content',
})
</script>
<template>
  <v-container fluid class="app-page"><AdminSetupGuidesManager /></v-container>
</template>
