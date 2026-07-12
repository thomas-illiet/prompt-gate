<script setup lang="ts">
defineProps<{ html: string; loading: boolean }>()
</script>

<template>
  <div class="admin-faq-preview">
    <v-skeleton-loader v-if="loading" type="article" />
    <AppEmptyState v-else-if="!html" icon="mdi-eye-off-outline" title="Nothing to preview" text="Add Markdown content in the editor first." />
    <!-- eslint-disable-next-line vue/no-v-html -- backend-sanitized HTML -->
    <div v-else class="app-markdown" v-html="html" />
  </div>
</template>

<style scoped>
.admin-faq-preview { min-height: 280px; padding: 16px; border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); border-radius: 8px; }
</style>
