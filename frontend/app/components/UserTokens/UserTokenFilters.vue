<script setup lang="ts">
import type { UserTokenStatusFilter } from '~/types/user-service'
import { userTokenStatusColor } from '~/utils/user-tokens'

const props = defineProps<{
  search: string
  statusFilter: UserTokenStatusFilter
}>()

const emit = defineEmits<{
  'update:search': [value: string]
  'update:status-filter': [value: UserTokenStatusFilter]
}>()

const statusItems = [
  { label: 'All', value: 'all' as const },
  { label: 'Active', value: 'active' as const },
  { label: 'Expired', value: 'expired' as const },
  { label: 'Revoked', value: 'revoked' as const },
].map((item) => ({
  ...item,
  color: statusFilterColor(item.value),
}))

// statusFilterColor maps filter values to the same Vuetify colors as token chips.
function statusFilterColor(status: UserTokenStatusFilter) {
  if (status === 'all') {
    return 'primary'
  }

  return userTokenStatusColor(status)
}

// updateStatus emits a valid token status filter.
function updateStatus(value: unknown) {
  if (
    value === 'all' ||
    value === 'active' ||
    value === 'expired' ||
    value === 'revoked'
  ) {
    emit('update:status-filter', value)
  }
}
</script>

<template>
  <AppFilterCard
    title="Manage virtual keys"
    subtitle="Search, filter, and review your personal virtual key records."
  >
    <div class="user-token-filters">
      <v-text-field
        :model-value="props.search"
        label="Search"
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        density="comfortable"
        hide-details
        clearable
        @update:model-value="emit('update:search', String($event ?? ''))"
      />

      <v-btn-toggle
        :model-value="props.statusFilter"
        density="comfortable"
        divided
        mandatory
        variant="outlined"
        class="user-token-filters__statuses"
        @update:model-value="updateStatus"
      >
        <v-btn
          v-for="item in statusItems"
          :key="item.value"
          :value="item.value"
          :color="item.color"
          class="user-token-filters__status"
        >
          <span
            class="user-token-filters__status-dot"
            :class="`user-token-filters__status-dot--${item.value}`"
            aria-hidden="true"
          />
          <span>{{ item.label }}</span>
        </v-btn>
      </v-btn-toggle>
    </div>
  </AppFilterCard>
</template>

<style scoped>
.user-token-filters {
  display: grid;
  gap: 14px;
}

.user-token-filters__statuses {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  height: auto;
}

.user-token-filters__status {
  min-height: 44px;
  min-width: 0;
}

.user-token-filters__status :deep(.v-btn__content) {
  display: flex;
  gap: 8px;
  min-width: 0;
}

.user-token-filters__status-dot {
  width: 8px;
  height: 8px;
  flex: 0 0 auto;
  border-radius: 999px;
  background: rgb(var(--v-theme-primary));
}

.user-token-filters__status-dot--active {
  background: rgb(var(--v-theme-success));
}

.user-token-filters__status-dot--expired {
  background: rgb(var(--v-theme-warning));
}

.user-token-filters__status-dot--revoked {
  background: rgba(var(--v-theme-on-surface), 0.56);
}

@media (min-width: 960px) {
  .user-token-filters {
    grid-template-columns: minmax(240px, 1fr) auto;
    align-items: start;
  }

  .user-token-filters__statuses {
    grid-template-columns: repeat(4, minmax(88px, auto));
  }
}
</style>
