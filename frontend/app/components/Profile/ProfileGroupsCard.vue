<script setup lang="ts">
import type { ProfileGroupSummary } from '~/types/groups'

const props = defineProps<{
  error: string | null
  groups: ProfileGroupSummary[]
  loading: boolean
}>()
</script>

<template>
  <ProfileInfoCard
    icon="mdi-account-multiple-check-outline"
    title="Groups"
    subtitle="Access groups assigned to this account"
  >
    <div v-if="props.loading" class="profile-groups-card__loading">
      <v-progress-circular indeterminate color="primary" size="32" />
    </div>

    <v-alert
      v-else-if="props.error"
      type="warning"
      variant="tonal"
      rounded="lg"
      class="ma-4"
    >
      {{ props.error }}
    </v-alert>

    <v-list
      v-else-if="props.groups.length"
      density="comfortable"
      class="profile-groups-card__list"
    >
      <v-list-item
        v-for="group in props.groups"
        :key="group.id"
        :title="group.name"
        :subtitle="group.description || 'No description'"
      >
        <template #prepend>
          <v-avatar color="primary" variant="tonal" size="36">
            <v-icon icon="mdi-account-group-outline" />
          </v-avatar>
        </template>
      </v-list-item>
    </v-list>

    <AppEmptyState
      v-else
      compact
      icon="mdi-account-multiple-remove-outline"
      title="No groups"
      text="This account is not assigned to an access group."
    />
  </ProfileInfoCard>
</template>

<style scoped>
.profile-groups-card__loading {
  display: grid;
  min-height: 160px;
  place-items: center;
}

.profile-groups-card__list {
  padding: 8px;
}
</style>
