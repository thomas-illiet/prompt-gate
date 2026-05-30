<script setup lang="ts">
import type { UserRoleFilter, UserStatusFilter } from '~/types/users'
import { APP_ROLES, appRoleLabel } from '~/utils/auth'

const props = defineProps<{
  role: UserRoleFilter
  search: string
  status: UserStatusFilter
}>()

const emit = defineEmits<{
  'update:role': [value: UserRoleFilter]
  'update:search': [value: string]
  'update:status': [value: UserStatusFilter]
}>()

const roleOptions = computed(() => [
  { title: 'All roles', value: 'all' as const },
  ...APP_ROLES.map((role) => ({
    title: appRoleLabel(role),
    value: role,
  })),
])
const statusOptions = [
  { title: 'All statuses', value: 'all' as const },
  { title: 'Active', value: 'active' as const },
  { title: 'Inactive', value: 'inactive' as const },
]

// updateSearch emits normalized search text.
function updateSearch(value: string) {
  emit('update:search', value)
}

// updateRole emits the selected role filter with an all fallback.
function updateRole(value: UserRoleFilter | null) {
  emit('update:role', value ?? 'all')
}

// updateStatus emits the selected status filter with an all fallback.
function updateStatus(value: UserStatusFilter | null) {
  emit('update:status', value ?? 'all')
}
</script>

<template>
  <AppFilterCard
    title="User filters"
    subtitle="Search identities, narrow roles, and isolate disabled accounts."
  >
    <div class="admin-users-filters__grid">
      <v-text-field
        :model-value="props.search"
        prepend-inner-icon="mdi-magnify"
        label="Search users"
        placeholder="Name, email, username"
        density="compact"
        variant="outlined"
        flat
        clearable
        hide-details
        @update:model-value="updateSearch(($event as string | null) ?? '')"
      />

      <v-select
        :model-value="props.role"
        :items="roleOptions"
        prepend-inner-icon="mdi-account-key-outline"
        label="Role"
        density="compact"
        variant="outlined"
        flat
        hide-details
        @update:model-value="updateRole($event as UserRoleFilter | null)"
      />

      <v-select
        :model-value="props.status"
        :items="statusOptions"
        prepend-inner-icon="mdi-account-check-outline"
        label="Status"
        density="compact"
        variant="outlined"
        flat
        hide-details
        @update:model-value="updateStatus($event as UserStatusFilter | null)"
      />
    </div>
  </AppFilterCard>
</template>

<style scoped>
.admin-users-filters__grid {
  display: grid;
  gap: 16px;
}

@media (min-width: 960px) {
  .admin-users-filters__grid {
    grid-template-columns: minmax(0, 1.6fr) repeat(2, minmax(180px, 0.8fr));
    align-items: center;
  }
}
</style>
