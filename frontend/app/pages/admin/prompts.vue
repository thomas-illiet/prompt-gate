<script setup lang="ts">
definePageMeta({
  requiredRoles: ['admin'],
  title: 'Prompt history',
  icon: 'mdi-history',
  drawerIndex: 9,
  drawerSection: 'Observability',
})

const adminPromptHistory = useAdminPromptHistory()

const totalLabel = computed(() =>
  adminPromptHistory.total.value === 1
    ? '1 prompt'
    : `${adminPromptHistory.total.value} prompts`,
)
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-history"
          kicker="Admin audit"
          title="Prompt history"
          copy="Review prompts recorded across user proxy usage."
          stat-label="Total"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <PromptHistoryFilters
          title="Prompt filters"
          subtitle="Search prompt text and narrow history to a user."
          show-user
          :loading-users="adminPromptHistory.loadingUsers.value"
          :search="adminPromptHistory.search.value"
          :user-id="adminPromptHistory.userId.value"
          :users="adminPromptHistory.users.value"
          @update:search="adminPromptHistory.setSearch"
          @update:user-id="adminPromptHistory.setUserId"
          @update:user-search="adminPromptHistory.setUserSearch"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminPromptHistory.usersError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminPromptHistory.usersError.value }}
        </v-alert>

        <v-alert
          v-if="adminPromptHistory.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminPromptHistory.listError.value }}
        </v-alert>

        <PromptHistoryTable
          column-preferences-key="promptgate.adminPromptHistory.columns.v1"
          enable-column-picker
          title="All prompts"
          scope-label="admin history"
          show-user
          :items="adminPromptHistory.prompts.value"
          :loading="adminPromptHistory.loading.value"
          :page="adminPromptHistory.page.value"
          :page-size="adminPromptHistory.pageSize.value"
          :sort-by="adminPromptHistory.sortBy.value"
          :sort-dir="adminPromptHistory.sortDir.value"
          :total="adminPromptHistory.total.value"
          @refresh="adminPromptHistory.reload"
          @update:page="adminPromptHistory.setPage"
          @update:page-size="adminPromptHistory.setPageSize"
          @update:sort="adminPromptHistory.setSort"
        />
      </v-col>
    </v-row>
  </v-container>
</template>
