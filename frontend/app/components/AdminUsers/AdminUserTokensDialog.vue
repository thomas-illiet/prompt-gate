<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { UserToken } from '~/types/user-service'
import type { AdminUser } from '~/types/users'
import { formatDateTime } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'
import {
  canRevokeUserToken,
  userTokenStatus,
  userTokenStatusColor,
  userTokenStatusLabel,
} from '~/utils/user-tokens'

const props = defineProps<{
  loading: boolean
  page: number
  pageSize: number
  saving: boolean
  sortBy: string
  sortDir: 'asc' | 'desc'
  tokens: UserToken[]
  total: number
  user: AdminUser | null
}>()

const emit = defineEmits<{
  refresh: []
  revoke: [token: UserToken]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const isOpen = defineModel<boolean>({ default: false })

const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  { title: 'Description', key: 'description' },
  appTableCenteredColumn({
    title: 'Status',
    key: 'status',
  }),
  appTableCenteredColumn({
    title: 'Created',
    key: 'createdAt',
  }),
  appTableCenteredColumn({
    title: 'Expires',
    key: 'expiresAt',
  }),
  appTableCenteredColumn({
    title: 'Actions',
    key: 'actions',
    sortable: false,
  }),
]

const title = computed(() =>
  props.user ? `${displayUser(props.user)} virtual keys` : 'User virtual keys',
)

const subtitle = computed(() => {
  if (props.total === 0) {
    return 'No virtual key records for this user.'
  }

  if (props.tokens.length === props.total) {
    return props.total === 1
      ? '1 virtual key record.'
      : `${props.total} virtual key records.`
  }

  return `Showing ${props.tokens.length} of ${props.total} virtual key records.`
})

const emptyState = computed(() => ({
  title: props.total === 0 ? 'No virtual keys' : 'No matching virtual keys',
  text:
    props.total === 0
      ? 'This user has no virtual keys.'
      : 'Adjust pagination or sorting to see more virtual key records.',
}))

// displayUser returns the most readable user identifier.
function displayUser(user: AdminUser) {
  return user.name || user.preferredUsername || user.email
}

// tokenStatusLabel returns display text for the token status chip.
function tokenStatusDisplayLabel(token: UserToken) {
  return userTokenStatusLabel(userTokenStatus(token))
}

// tokenStatusDisplayColor returns the Vuetify color for the token status chip.
function tokenStatusDisplayColor(token: UserToken) {
  return userTokenStatusColor(userTokenStatus(token))
}
</script>

<template>
  <AppDialogCard v-model="isOpen" content-class="admin-user-tokens-dialog__body" icon="mdi-key-chain" :loading="props.saving" max-width="980" :subtitle="subtitle" :title="title">
        <div class="admin-user-tokens-dialog__toolbar">
          <div class="admin-user-tokens-dialog__heading">
            <v-avatar color="primary" variant="tonal" size="36">
              <v-icon icon="mdi-table-key" />
            </v-avatar>
            <span class="admin-user-tokens-dialog__heading-text">
              Virtual key records
            </span>
          </div>

          <v-btn
            color="primary"
            variant="tonal"
            rounded="lg"
            prepend-icon="mdi-refresh"
            :loading="props.loading"
            @click="emit('refresh')"
          >
            Refresh
          </v-btn>
        </div>

        <AppServerDataTable
          default-sort-by="createdAt"
          default-sort-dir="desc"
          :headers="headers"
          :items="props.tokens"
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
          <template #no-data>
            <AppEmptyState
              compact
              icon="mdi-key-outline"
              :title="emptyState.title"
              :text="emptyState.text"
            />
          </template>

          <template #item.name="{ item }">
            <span class="app-table-text app-table-text--strong">
              {{ item.name }}
            </span>
          </template>

          <template #item.description="{ item }">
            <span class="app-table-text">
              {{ item.description || 'No description' }}
            </span>
          </template>

          <template #item.status="{ item }">
            <div class="app-table-center">
              <v-chip
                size="small"
                label
                variant="tonal"
                :color="tokenStatusDisplayColor(item)"
              >
                {{ tokenStatusDisplayLabel(item) }}
              </v-chip>
            </div>
          </template>

          <template #item.createdAt="{ item }">
            <span class="app-table-text">
              {{ formatDateTime(item.createdAt) }}
            </span>
          </template>

          <template #item.expiresAt="{ item }">
            <span class="app-table-text">
              {{ formatDateTime(item.expiresAt) }}
            </span>
          </template>

          <template #item.actions="{ item }">
            <div class="app-table-center">
              <v-btn
                size="small"
                color="error"
                variant="tonal"
                rounded="lg"
                prepend-icon="mdi-key-remove"
                :disabled="!canRevokeUserToken(item) || props.saving"
                :loading="props.saving"
                @click="emit('revoke', item)"
              >
                Revoke
              </v-btn>
            </div>
          </template>
        </AppServerDataTable>
      

      <template #actions>
        <AppDialogCloseButton
          :disabled="props.saving"
          @click="isOpen = false"
        />
      </template>
  </AppDialogCard>
</template>

<style scoped>
.admin-user-tokens-dialog__body {
  display: grid;
  gap: 16px;
}

.admin-user-tokens-dialog__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.admin-user-tokens-dialog__heading {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 12px;
}

.admin-user-tokens-dialog__heading-text {
  overflow: hidden;
  color: rgb(var(--app-shell-text));
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 640px) {
  .admin-user-tokens-dialog__toolbar {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
