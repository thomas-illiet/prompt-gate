<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type {
  ServiceAccount,
  TokenResponse,
} from '~/types/service-accounts'
import { formatDateTime } from '~/utils/formatters'
import { appTableCenteredColumn } from '~/utils/table'

const props = defineProps<{
  account: ServiceAccount | null
  loading: boolean
  page: number
  pageSize: number
  saving: boolean
  sortBy: string
  sortDir: 'asc' | 'desc'
  tokens: TokenResponse[]
  total: number
}>()

const emit = defineEmits<{
  create: []
  refresh: []
  revoke: [token: TokenResponse]
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const isOpen = defineModel<boolean>({ default: false })
const showRevoked = defineModel<boolean>('showRevoked', { default: false })

const descriptionColumnProps = {
  class: 'admin-service-account-tokens-dialog__description-column',
}
const headers: DataTableHeader[] = [
  { title: 'Name', key: 'name' },
  {
    title: 'Description',
    key: 'description',
    headerProps: descriptionColumnProps,
    cellProps: descriptionColumnProps,
  },
  appTableCenteredColumn({
    title: 'Status',
    key: 'status',
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
  props.account
    ? `${props.account.name} virtual keys`
    : 'Service account virtual keys',
)

// tokenStatus derives the current display status for a service account token.
function tokenStatus(token: TokenResponse) {
  if (token.revokedAt) {
    return { label: 'Revoked', color: 'grey' }
  }
  if (token.expiredAt || new Date(token.expiresAt).getTime() <= Date.now()) {
    return { label: 'Expired', color: 'warning' }
  }

  return { label: 'Active', color: 'success' }
}

// canRevoke reports whether a service account token can still be revoked.
function canRevoke(token: TokenResponse) {
  return !token.revokedAt && !token.expiredAt
}
</script>

<template>
  <AppDialogCard v-model="isOpen" content-class="admin-service-account-tokens-dialog__body" icon="mdi-key-chain" :loading="props.saving" max-width="980" subtitle="Manage virtual keys for this service account. Raw key values are shown only once." :title="title">
        <div class="admin-service-account-tokens-dialog__table">
          <div class="admin-service-account-tokens-dialog__table-header">
            <div class="admin-service-account-tokens-dialog__table-heading">
              <v-avatar color="primary" variant="tonal" size="36">
                <v-icon icon="mdi-table-key" />
              </v-avatar>
              <div class="admin-service-account-tokens-dialog__heading-copy">
                <h2 class="admin-service-account-tokens-dialog__section-title">
                  Virtual keys
                </h2>
                <p
                  class="admin-service-account-tokens-dialog__section-subtitle"
                >
                  {{ props.total }} virtual key records
                </p>
              </div>
            </div>

            <div class="admin-service-account-tokens-dialog__table-actions">
              <v-btn
                color="primary"
                variant="flat"
                rounded="xl"
                prepend-icon="mdi-key-plus"
                :disabled="props.saving"
                @click="emit('create')"
              >
                New key
              </v-btn>

              <v-switch
                v-model="showRevoked"
                class="admin-service-account-tokens-dialog__revoked-switch"
                color="primary"
                density="compact"
                hide-details
                inset
                label="Show revoked"
              />

              <v-btn
                color="primary"
                variant="tonal"
                rounded="xl"
                prepend-icon="mdi-refresh"
                :loading="props.loading"
                @click="emit('refresh')"
              >
                Refresh
              </v-btn>
            </div>
          </div>

          <div class="admin-service-account-tokens-dialog__table-scroll">
            <AppServerDataTable
              default-sort-by="createdAt"
              default-sort-dir="desc"
              empty-icon="mdi-key-outline"
              empty-title="No virtual keys found"
              empty-text="This account has no virtual keys for the current view."
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
              <template #item.name="{ item }">
                <span class="admin-service-account-tokens-dialog__text">
                  {{ item.name }}
                </span>
              </template>

              <template #item.description="{ item }">
                <span class="admin-service-account-tokens-dialog__text">
                  {{ item.description || 'No description' }}
                </span>
              </template>

              <template #item.status="{ item }">
                <div class="admin-service-account-tokens-dialog__center">
                  <v-chip
                    size="small"
                    label
                    variant="tonal"
                    :color="tokenStatus(item).color"
                  >
                    {{ tokenStatus(item).label }}
                  </v-chip>
                </div>
              </template>

              <template #item.expiresAt="{ item }">
                <div class="admin-service-account-tokens-dialog__center">
                  <span class="admin-service-account-tokens-dialog__text">
                    {{ formatDateTime(item.expiresAt) }}
                  </span>
                </div>
              </template>

              <template #item.actions="{ item }">
                <div class="admin-service-account-tokens-dialog__center">
                  <v-btn
                    size="small"
                    color="error"
                    variant="tonal"
                    rounded="lg"
                    :disabled="!canRevoke(item)"
                    :loading="props.saving"
                    @click="emit('revoke', item)"
                  >
                    Revoke
                  </v-btn>
                </div>
              </template>
            </AppServerDataTable>
          </div>
        </div>
      

      <template #actions>
        <AppDialogCloseButton
          :disabled="props.saving"
          @click="isOpen = false"
        />
      </template>
  </AppDialogCard>
</template>

<style scoped>
.admin-service-account-tokens-dialog__body {
  display: grid;
  gap: 16px;
}

.admin-service-account-tokens-dialog__section-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
}

.admin-service-account-tokens-dialog__section-subtitle {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
  opacity: 1;
}

.admin-service-account-tokens-dialog__text {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}

.admin-service-account-tokens-dialog__center {
  display: flex;
  justify-content: center;
  width: 100%;
}

.admin-service-account-tokens-dialog__table {
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.5);
  border-radius: 8px;
}

.admin-service-account-tokens-dialog__table-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
}

.admin-service-account-tokens-dialog__table-heading {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.admin-service-account-tokens-dialog__heading-copy {
  min-width: 0;
}

.admin-service-account-tokens-dialog__table-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.admin-service-account-tokens-dialog__revoked-switch {
  flex: 0 0 auto;
}

.admin-service-account-tokens-dialog__table-scroll {
  overflow-x: auto;
}

@media (max-width: 720px) {
  .admin-service-account-tokens-dialog__table-header {
    align-items: stretch;
    flex-direction: column;
  }

  .admin-service-account-tokens-dialog__table-actions {
    justify-content: flex-start;
  }

  .admin-service-account-tokens-dialog__table-scroll
    :deep(.admin-service-account-tokens-dialog__description-column) {
    display: none;
  }
}
</style>
