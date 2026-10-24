<script setup lang="ts">
import AppErrorState from '~/components/App/AppErrorState.vue'

definePageMeta({
  title: 'Page not found',
})
</script>

<template>
  <v-container fluid class="app-page not-found-page">
    <AppErrorState
      :code="404"
      eyebrow="Route signal"
      title="Page not found"
      text="This route is not connected to PromptGate. Check the address or return to a known workspace view."
      variant="not-found"
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
    </AppErrorState>
  </v-container>
</template>

<style scoped>
.not-found-page {
  display: grid;
  min-height: calc(100vh - 120px);
  place-items: center;
}
</style>
