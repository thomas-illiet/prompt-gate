<script setup lang="ts">
definePageMeta({
  icon: 'mdi-history',
  title: 'Prompt history',
  drawerIndex: 3,
  requiredRoles: ['user', 'manager', 'admin'],
})

const promptHistory = usePromptHistory()

const totalLabel = computed(() =>
  promptHistory.total.value === 1
    ? '1 prompt'
    : `${promptHistory.total.value} prompts`,
)
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-history"
          kicker="Prompt audit"
          title="Prompt history"
          copy="Review the prompts recorded from your own proxy usage."
          stat-label="Total"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <PromptHistoryFilters
          :search="promptHistory.search.value"
          @update:search="promptHistory.setSearch"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="promptHistory.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ promptHistory.listError.value }}
        </v-alert>

        <PromptHistoryTable
          column-preferences-key="promptgate.promptHistory.columns.v1"
          enable-column-picker
          :items="promptHistory.prompts.value"
          :loading="promptHistory.loading.value"
          :page="promptHistory.page.value"
          :page-size="promptHistory.pageSize.value"
          :sort-by="promptHistory.sortBy.value"
          :sort-dir="promptHistory.sortDir.value"
          :total="promptHistory.total.value"
          @refresh="promptHistory.reload"
          @update:page="promptHistory.setPage"
          @update:page-size="promptHistory.setPageSize"
          @update:sort="promptHistory.setSort"
        />
      </v-col>
    </v-row>
  </v-container>
</template>
