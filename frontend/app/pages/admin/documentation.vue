<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'API documentation',
  icon: 'mdi-api',
  drawerIndex: 11,
  drawerSection: 'Developer tools',
})
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-api"
          kicker="Developer tools"
          title="API documentation"
          copy="Explore the PromptGate API and execute requests with your current administrator session."
          visual="cables"
          stat-label="Authentication"
          stat-value="Current session"
        />
      </v-col>

      <v-col cols="12">
        <ClientOnly>
          <AdminDocumentationSwaggerDocumentation />

          <template #fallback>
            <AppSectionCard
              icon="mdi-api"
              title="Interactive API explorer"
              subtitle="Loading the OpenAPI document."
            >
              <v-progress-linear indeterminate />
            </AppSectionCard>
          </template>
        </ClientOnly>
      </v-col>
    </v-row>
  </v-container>
</template>
