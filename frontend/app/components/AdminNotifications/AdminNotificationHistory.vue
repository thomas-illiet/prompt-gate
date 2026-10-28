<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type {
  NotificationDeliveryStatus,
  NotificationKind,
  NotificationMessageSummary,
} from '~/types/notifications'
import { formatDateTime } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  items: NotificationMessageSummary[]
  kind: NotificationKind | ''
  loading: boolean
  page: number
  pageSize: number
  search: string
  status: NotificationDeliveryStatus | ''
  total: number
}>()

const emit = defineEmits<{
  details: [message: NotificationMessageSummary]
  refresh: []
  'update:kind': [value: NotificationKind | '']
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:search': [value: string]
  'update:status': [value: NotificationDeliveryStatus | '']
}>()

const headers: DataTableHeader[] = [
  { title: 'Created', key: 'createdAt' },
  { title: 'Type', key: 'kind' },
  { title: 'Subject', key: 'subject' },
  appTableCenteredColumn({ title: 'Recipients', key: 'recipients' }),
  appTableCenteredColumn({ title: 'Sent', key: 'sent' }),
  appTableCenteredColumn({ title: 'Pending', key: 'pending' }),
  appTableCenteredColumn({ title: 'Failed', key: 'failed' }),
  appTableCenteredColumn({ title: 'Actions', key: 'actions', sortable: false }),
]

const kinds = [
  { title: 'All types', value: '' },
  { title: 'Manual', value: 'manual' },
  { title: 'Welcome', value: 'welcome' },
  { title: 'Quota exhausted', value: 'quota_exhausted' },
  { title: 'Quota reset', value: 'quota_reset_granted' },
  { title: 'Group changed', value: 'group_changed' },
  { title: 'Token created', value: 'token_created' },
  { title: 'Token expired', value: 'token_expired' },
  { title: 'Token revoked', value: 'token_revoked' },
  { title: 'Monthly report', value: 'monthly_usage_report' },
]

const statuses = [
  { title: 'All statuses', value: '' },
  { title: 'Pending', value: 'pending' },
  { title: 'Processing', value: 'processing' },
  { title: 'Sent', value: 'sent' },
  { title: 'Failed', value: 'failed' },
]
</script>

<template>
  <AppSectionCard
    class="admin-notification-history"
    icon="mdi-email-fast-outline"
    title="Delivery history"
    subtitle="Inspect automatic and manually composed messages, delivery attempts, and SMTP failures."
  >
    <template #actions>
      <v-btn
        color="primary"
        size="small"
        variant="tonal"
        prepend-icon="mdi-refresh"
        :loading="props.loading"
        @click="emit('refresh')"
      >
        Refresh
      </v-btn>
    </template>

    <div class="admin-notification-history__filters">
      <div
        class="admin-notification-history__filter-column admin-notification-history__filter-column--search"
      >
        <v-text-field
          :model-value="props.search"
          density="compact"
          label="Search subject"
          prepend-inner-icon="mdi-magnify"
          clearable
          hide-details
          @update:model-value="emit('update:search', $event ?? '')"
        />
      </div>
      <div class="admin-notification-history__filter-column">
        <v-select
          :model-value="props.kind"
          :items="kinds"
          density="compact"
          label="Type"
          hide-details
          @update:model-value="emit('update:kind', $event)"
        />
      </div>
      <div class="admin-notification-history__filter-column">
        <v-select
          :model-value="props.status"
          :items="statuses"
          density="compact"
          label="Status"
          hide-details
          @update:model-value="emit('update:status', $event)"
        />
      </div>
    </div>

    <AppServerDataTable
      default-sort-by="createdAt"
      default-sort-dir="desc"
      empty-icon="mdi-email-off-outline"
      empty-title="No email history"
      empty-text="Notification messages will appear after they are queued."
      :headers="headers"
      :items="props.items"
      :loading="props.loading"
      :page="props.page"
      :page-size="props.pageSize"
      sort-by="createdAt"
      sort-dir="desc"
      :total="props.total"
      @update:page="emit('update:page', $event)"
      @update:page-size="emit('update:page-size', $event)"
    >
      <template #item.createdAt="{ item }">{{
        formatDateTime(item.createdAt)
      }}</template>
      <template #item.kind="{ item }">
        <v-chip size="small" variant="tonal">{{
          item.kind.replaceAll('_', ' ')
        }}</v-chip>
      </template>
      <template #item.subject="{ item }">
        <span class="app-table-text app-table-text--strong">{{
          item.subject
        }}</span>
      </template>
      <template #item.recipients="{ item }"
        ><div class="app-table-center">{{ item.recipients }}</div></template
      >
      <template #item.sent="{ item }"
        ><div class="app-table-center text-success">
          {{ item.sent }}
        </div></template
      >
      <template #item.pending="{ item }"
        ><div class="app-table-center">
          {{ item.pending + item.processing }}
        </div></template
      >
      <template #item.failed="{ item }"
        ><div class="app-table-center text-error">
          {{ item.failed }}
        </div></template
      >
      <template #item.actions="{ item }">
        <div class="app-table-center">
          <v-btn
            icon="mdi-eye-outline"
            variant="text"
            aria-label="Open notification details"
            @click="emit('details', item)"
          />
        </div>
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.admin-notification-history :deep(.app-section-card__header) {
  gap: 12px;
  padding: 16px 20px 12px;
}

.admin-notification-history :deep(.app-section-card__heading) {
  gap: 10px;
}

.admin-notification-history :deep(.app-section-card__icon) {
  width: 32px;
  height: 32px;
}

.admin-notification-history__filters {
  display: grid;
  grid-template-columns: minmax(0, 2fr) repeat(2, minmax(0, 1fr));
  gap: 12px;
  padding: 16px 18px 14px;
}

.admin-notification-history__filter-column {
  min-width: 0;
}

.admin-notification-history :deep(.v-data-table__th),
.admin-notification-history :deep(.v-data-table__td) {
  height: 48px !important;
}

@media (max-width: 720px) {
  .admin-notification-history :deep(.app-section-card__header) {
    padding: 14px 16px 12px;
  }

  .admin-notification-history__filters {
    grid-template-columns: 1fr;
    padding: 12px;
  }
}

@media (min-width: 721px) and (max-width: 959px) {
  .admin-notification-history__filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .admin-notification-history__filter-column--search {
    grid-column: 1 / -1;
  }
}
</style>
