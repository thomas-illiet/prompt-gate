<script setup lang="ts">
import type { DataTableHeader } from 'vuetify'

import type { AccountIPAddress } from '~/types/account-ips'
import { formatDateTime } from '~/utils/formatters'

const props = defineProps<{
  accountName: string
  items: AccountIPAddress[]
  loading: boolean
  page: number
  pageSize: number
  sortBy: string
  sortDir: 'asc' | 'desc'
  total: number
}>()

const emit = defineEmits<{
  refresh: []
  'update:page': [value: number]
  'update:page-size': [value: number]
  'update:sort': [sortBy: string, sortDir: 'asc' | 'desc']
}>()

const isOpen = defineModel<boolean>({ default: false })

const headers: DataTableHeader[] = [
  { title: 'IP address', key: 'ip' },
  { title: 'Last seen', key: 'lastSeen' },
]

const subtitle = computed(() => {
  if (props.total === 0) return 'No proxy IP addresses recorded.'
  if (props.items.length === props.total) {
    return props.total === 1 ? '1 IP address recorded.' : `${props.total} IP addresses recorded.`
  }
  return `Showing ${props.items.length} of ${props.total} IP addresses.`
})
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    content-class="admin-account-ips-dialog__body"
    icon="mdi-ip-network-outline"
    max-width="820"
    :subtitle="subtitle"
    :title="`${props.accountName} IP addresses`"
  >
    <div class="admin-account-ips-dialog__toolbar">
      <span class="admin-account-ips-dialog__heading">Proxy activity</span>
      <v-btn
        color="primary"
        prepend-icon="mdi-refresh"
        rounded="lg"
        variant="tonal"
        :loading="props.loading"
        @click="emit('refresh')"
      >
        Refresh
      </v-btn>
    </div>

    <AppServerDataTable
      default-sort-by="lastSeen"
      default-sort-dir="desc"
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
      @update:sort="(sortBy, sortDir) => emit('update:sort', sortBy, sortDir)"
    >
      <template #no-data>
        <AppEmptyState
          compact
          icon="mdi-ip-off-outline"
          text="An address will appear after this account completes a proxy request."
          title="No IP addresses"
        />
      </template>
      <template #item.ip="{ item }">
        <code class="admin-account-ips-dialog__ip">{{ item.ip }}</code>
      </template>
      <template #item.lastSeen="{ item }">
        <span class="app-table-text">{{ formatDateTime(item.lastSeen) }}</span>
      </template>
    </AppServerDataTable>

    <template #actions>
      <AppDialogCloseButton @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>

<style scoped>
.admin-account-ips-dialog__body {
  display: grid;
  gap: 16px;
}

.admin-account-ips-dialog__toolbar {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.admin-account-ips-dialog__heading {
  font-weight: 600;
}

.admin-account-ips-dialog__ip {
  color: rgb(var(--v-theme-primary));
  font-size: 0.875rem;
}
</style>
