<script setup lang="ts">
definePageMeta({
  title: 'Page not found',
})
</script>

<template>
  <v-container fluid class="app-page not-found-page">
    <AppSectionCard
      icon="custom:nustar"
      title="Page not found"
      subtitle="HTTP 404"
    >
      <AppEmptyState
        icon="custom:nustar"
        title="Whoops, 404"
        text="This route has no signal. Head back to a live workspace view."
      >
        <template #actions>
          <v-btn
            to="/dashboard"
            color="primary"
            variant="flat"
            rounded="lg"
            prepend-icon="mdi-monitor-dashboard"
          >
            Back to dashboard
          </v-btn>
        </template>
      </AppEmptyState>
    </AppSectionCard>
  </v-container>
</template>

<style scoped>
.not-found-page {
  display: grid;
  min-height: calc(100vh - 120px);
  align-items: center;
}
</style>
