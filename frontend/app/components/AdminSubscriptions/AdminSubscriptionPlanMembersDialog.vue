<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import { toAdminSubscriptionErrorMessage } from '~/composables/useAdminSubscriptions'
import type {
  SubscriptionPlan,
  SubscriptionPlanMember,
} from '~/types/subscriptions'

const props = defineProps<{ plan: SubscriptionPlan }>()
const isOpen = defineModel<boolean>({ required: true })
const apiFetch = useApiFetch()

const members = useQueryList<SubscriptionPlanMember>({
  fetch: (queryString) => {
    const query = new URLSearchParams(queryString)
    query.delete('sortBy')
    query.delete('sortDir')
    return apiFetch<{ items: SubscriptionPlanMember[]; total: number }>(
      `/api/v1/admin/subscriptions/${encodeURIComponent(props.plan.id)}/members?${query}`,
    )
  },
  initialSortBy: 'name',
  initialSortDir: 'asc',
  toErrorMessage: toAdminSubscriptionErrorMessage,
})

const headers: DataTableHeader[] = [
  { title: 'Member', key: 'name', sortable: false },
  { title: 'Account type', key: 'type', sortable: false },
  { title: 'Assignment', key: 'inherited', sortable: false },
]
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-account-multiple-outline"
    max-width="760"
    :title="`${props.plan.name} members`"
  >
    <v-text-field
      :model-value="members.search.value"
      label="Search members"
      prepend-inner-icon="mdi-magnify"
      variant="outlined"
      density="compact"
      class="mb-4"
      clearable
      hide-details
      @update:model-value="members.setSearch($event ?? '')"
    />

    <v-alert
      v-if="members.listError.value"
      type="warning"
      variant="tonal"
      rounded="lg"
    >
      {{ members.listError.value }}
    </v-alert>

    <AppServerDataTable
      v-else
      default-sort-by="name"
      default-sort-dir="asc"
      empty-icon="mdi-account-search-outline"
      :empty-title="
        members.search.value.trim() ? 'No matching members' : 'No members'
      "
      :empty-text="
        members.search.value.trim()
          ? 'Try another name, email, or identifier.'
          : 'No accounts are assigned to this plan.'
      "
      :headers="headers"
      :items="members.items.value"
      :loading="members.loading.value"
      :must-sort="false"
      :page="members.page.value"
      :page-size="members.pageSize.value"
      :sort-by="members.sortBy.value"
      :sort-dir="members.sortDir.value"
      :total="members.total.value"
      @update:page="members.setPage"
      @update:page-size="members.setPageSize"
    >
      <template #item.name="{ item }">
        <div class="app-table-text app-table-text--strong">
          {{ item.name || item.preferredUsername || item.email || item.id }}
        </div>
        <div class="app-table-text app-table-text--secondary">
          {{
            item.type === 'service'
              ? `service:${item.preferredUsername}`
              : item.email || item.preferredUsername
          }}
        </div>
      </template>

      <template #item.type="{ item }">
        {{ item.type === 'service' ? 'Service account' : 'User' }}
      </template>

      <template #item.inherited="{ item }">
        <v-chip size="small" label variant="tonal">
          {{ item.inherited ? 'Inherited' : 'Direct' }}
        </v-chip>
      </template>
    </AppServerDataTable>

    <template #actions>
      <v-btn
        color="primary"
        variant="tonal"
        rounded="lg"
        prepend-icon="mdi-refresh"
        :loading="members.loading.value"
        @click="members.reload()"
      >
        Refresh
      </v-btn>
      <AppDialogCloseButton label="Close" @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>
