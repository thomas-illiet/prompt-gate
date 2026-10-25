<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AppRowAction } from '~/types/row-actions'
import { QUOTA_RESET_MAXIMUM } from '~/types/quota-resets'
import type { AdminUser } from '~/types/users'
import { appRoleColor, appRoleLabel } from '~/utils/auth'
import { formatNumber } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  exporting: boolean
  items: AdminUser[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  create: []
  delete: [user: AdminUser]
  edit: [user: AdminUser]
  export: []
  manageIps: [user: AdminUser]
  manageGroups: [user: AdminUser]
  manageTokens: [user: AdminUser]
  notes: [user: AdminUser]
  offerQuotaReset: [user: AdminUser]
  refresh: []
  toggleStatus: [user: AdminUser]
  usageStatistics: [user: AdminUser]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  { title: 'Identity', key: 'email' },
  appTableCenteredColumn({
    title: 'Role',
    key: 'role',
  }),
  appTableCenteredColumn({
    title: 'Access',
    key: 'isActive',
  }),
  appTableCenteredColumn({
    title: 'Platform activity',
    key: 'activeInLast30Days',
    sortable: false,
  }),
  appTableCenteredColumn({
    title: 'Plan',
    key: 'subscriptionPlan',
    sortable: false,
  }),
  appTableCenteredColumn({
    title: 'Input tokens',
    key: 'inputTokens',
  }),
  appTableCenteredColumn({
    title: 'Output tokens',
    key: 'outputTokens',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

const summaryLabel = computed(() => {
  if (props.total === 0) {
    return 'No accounts match the current filters.'
  }

  if (props.items.length === props.total) {
    return props.total === 1
      ? '1 account in the directory.'
      : `${props.total} accounts in the directory.`
  }

  return `Showing ${props.items.length} of ${props.total} accounts.`
})

// displayName returns the preferred user display name.
function displayName(user: AdminUser) {
  return user.name || user.preferredUsername
}

// displayUsername returns a stable username label for disambiguating duplicate names.
function displayUsername(user: AdminUser) {
  return user.preferredUsername ? `@${user.preferredUsername}` : 'No username'
}

// displayEmail returns the user's email or a readable fallback.
function displayEmail(user: AdminUser) {
  return user.email || 'No email'
}

// displaySubjectSuffix keeps OIDC subjects scannable in compact table cells.
function displaySubjectSuffix(user: AdminUser) {
  if (!user.sub) {
    return 'Not linked'
  }

  if (user.sub.length <= 12) {
    return `sub: ${user.sub}`
  }

  return `sub: ...${user.sub.slice(-8)}`
}

function planLabel(user: AdminUser) {
  return user.effectiveSubscriptionPlan?.name ?? 'No subscription'
}

function planCaption(user: AdminUser) {
  if (!user.effectiveSubscriptionPlan) {
    return 'Required'
  }
  return user.subscriptionPlanId ? 'Assigned' : 'Default'
}

function planColor(user: AdminUser) {
  if (!user.effectiveSubscriptionPlan) {
    return 'error'
  }
  return user.subscriptionPlanId ? 'primary' : 'success'
}

const rowActions: AppRowAction<AdminUser>[] = [
  {
    icon: 'mdi-ip-network-outline',
    key: 'manageIps',
    onSelect: (user) => emit('manageIps', user),
    title: 'IP addresses',
  },
  {
    icon: 'mdi-pencil-outline',
    key: 'edit',
    onSelect: (user) => emit('edit', user),
    title: 'Edit access',
  },
  {
    icon: 'mdi-key-chain',
    key: 'tokens',
    onSelect: (user) => emit('manageTokens', user),
    title: 'Manage virtual keys',
  },
  {
    icon: 'mdi-account-multiple-check-outline',
    key: 'groups',
    onSelect: (user) => emit('manageGroups', user),
    title: 'Manage groups',
  },
  {
    icon: 'mdi-chart-box-outline',
    key: 'usageStatistics',
    onSelect: (user) => emit('usageStatistics', user),
    title: 'Usage statistics',
  },
  {
    disabled: (user) => user.quotaResetCredits >= QUOTA_RESET_MAXIMUM,
    icon: 'mdi-refresh',
    key: 'offerQuotaReset',
    onSelect: (user) => emit('offerQuotaReset', user),
    title: (user) =>
      `Offer quota reset (${user.quotaResetCredits}/${QUOTA_RESET_MAXIMUM})`,
  },
  {
    icon: 'mdi-note-edit-outline',
    key: 'notes',
    onSelect: (user) => emit('notes', user),
    title: 'Notes',
  },
  {
    color: 'error',
    icon: 'mdi-delete-outline',
    key: 'delete',
    onSelect: (user) => emit('delete', user),
    title: 'Delete user',
  },
]
</script>

<template>
  <AppSectionCard
    icon="mdi-account-group-outline"
    title="Directory access"
    :subtitle="summaryLabel"
  >
    <template #actions>
      <v-btn
        data-test="pre-create-user"
        color="primary"
        rounded="xl"
        prepend-icon="mdi-account-plus-outline"
        @click.stop="emit('create')"
      >
        Pre-create user
      </v-btn>
      <v-btn
        data-test="export-users"
        color="primary"
        prepend-icon="mdi-download-outline"
        rounded="xl"
        variant="tonal"
        :disabled="props.loading"
        :loading="props.exporting"
        @click.stop="emit('export')"
      >
        Export CSV
      </v-btn>
      <v-btn
        color="primary"
        variant="tonal"
        rounded="xl"
        prepend-icon="mdi-refresh"
        :loading="props.loading"
        @click.stop="emit('refresh')"
      >
        Refresh
      </v-btn>
    </template>

    <AppServerDataTable
      default-sort-by="lastLoginAt"
      default-sort-dir="desc"
      empty-icon="mdi-account-group-outline"
      empty-title="No users found"
      empty-text="Try adjusting the search, role, or access filters."
      :headers="headers"
      :items="props.items"
      :loading="props.loading"
      :page="props.page"
      :page-size="props.pageSize"
      :sort-by="props.sortBy"
      :sort-dir="props.sortDir"
      :total="props.total"
      @update:page="emit('update:page', $event)"
      @update:page-size="emit('update:page-size', $event)"
      @update:sort="
        (nextSortBy, nextSortDir) =>
          emit('update:sort', nextSortBy, nextSortDir)
      "
    >
      <template #item.name="{ item }">
        <div class="admin-users-table__identity-cell">
          <span class="app-table-text app-table-text--strong">
            {{ displayName(item) }}
          </span>
          <span class="app-table-text app-table-text--muted">
            {{ displayUsername(item) }}
          </span>
        </div>
      </template>

      <template #item.email="{ item }">
        <div class="admin-users-table__identity-cell">
          <span class="app-table-text app-table-text--secondary">
            {{ displayEmail(item) }}
          </span>
          <span class="app-table-text app-table-text--muted">
            {{ displaySubjectSuffix(item) }}
          </span>
        </div>
      </template>

      <template #item.role="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="appRoleColor(item.role)"
          >
            {{ appRoleLabel(item.role) }}
          </v-chip>
        </div>
      </template>

      <template #item.isActive="{ item }">
        <div class="app-table-center">
          <AppStatusToggleButton
            :active="item.isActive"
            active-label="Enabled"
            inactive-label="Disabled"
            @click="emit('toggleStatus', item)"
          />
        </div>
      </template>

      <template #item.activeInLast30Days="{ item }">
        <div class="app-table-center">
          <v-chip
            size="small"
            label
            variant="tonal"
            :color="item.activeInLast30Days ? 'success' : undefined"
          >
            {{ item.activeInLast30Days ? 'Active (30d)' : 'Inactive (30d)' }}
          </v-chip>
        </div>
      </template>

      <template #item.subscriptionPlan="{ item }">
        <div class="app-table-center">
          <v-chip size="small" label variant="tonal" :color="planColor(item)">
            {{ planLabel(item) }}
            <span class="admin-users-table__chip-caption">
              {{ planCaption(item) }}
            </span>
          </v-chip>
        </div>
      </template>

      <template #item.inputTokens="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.inputTokens) }}
        </span>
      </template>

      <template #item.outputTokens="{ item }">
        <span class="app-table-text">
          {{ formatNumber(item.outputTokens) }}
        </span>
      </template>

      <template #item.actions="{ item }">
        <AppRowActionMenu :actions="rowActions" :item="item" />
      </template>
    </AppServerDataTable>
  </AppSectionCard>
</template>

<style scoped>
.admin-users-table__identity-cell {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.admin-users-table__chip-caption {
  margin-left: 6px;
  opacity: 0.68;
}
</style>
