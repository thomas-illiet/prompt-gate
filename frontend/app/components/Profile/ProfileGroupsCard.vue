<script setup lang="ts">
import type { ProfileGroupSummary } from '~/types/groups'

const props = defineProps<{
  error: string | null
  groups: ProfileGroupSummary[]
  loading: boolean
}>()

const groupCountLabel = computed(() =>
  props.groups.length === 1 ? '1 group' : `${props.groups.length} groups`,
)
</script>

<template>
  <ProfileInfoCard
    icon="mdi-account-multiple-check-outline"
    title="Groups"
    subtitle="Access groups assigned to this account"
  >
    <template #actions>
      <v-chip label variant="tonal" color="primary" size="small">
        {{ groupCountLabel }}
      </v-chip>
    </template>

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

    <div v-else-if="props.groups.length" class="profile-groups-card__list">
      <article
        v-for="group in props.groups"
        :key="group.id"
        class="profile-groups-card__item"
      >
        <v-avatar color="primary" variant="tonal" size="36">
          <v-icon icon="mdi-shield-check-outline" />
        </v-avatar>

        <div class="profile-groups-card__copy">
          <h3 class="profile-groups-card__name">
            {{ group.displayName || group.name }}
          </h3>
          <p class="profile-groups-card__description">
            {{ group.description || group.name }}
          </p>
        </div>
      </article>
    </div>

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
  display: grid;
  gap: 10px;
  padding: 0 24px 24px;
}

.profile-groups-card__item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.42);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.58);
}

.profile-groups-card__copy {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.profile-groups-card__name {
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 0.95rem;
  font-weight: 750;
}

.profile-groups-card__description {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.45;
  overflow-wrap: anywhere;
}
</style>
