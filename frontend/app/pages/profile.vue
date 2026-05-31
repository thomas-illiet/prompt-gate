<script setup lang="ts">
import { storeToRefs } from 'pinia'

import { Notify } from '~/stores/notification'
import { appRoleColor, appRoleLabel, isBlockedUser } from '~/utils/auth'
import { formatDateTime } from '~/utils/formatters'

definePageMeta({
  title: 'Profile',
  requiredRoles: ['user', 'manager', 'admin'],
})

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
const profileGroups = useProfileGroups()

const displayName = computed(
  () =>
    user.value?.name || user.value?.preferredUsername || 'Authenticated user',
)
const displayUsername = computed(
  () => user.value?.preferredUsername || 'No username available',
)
const displayEmail = computed(() => user.value?.email || 'No email available')
const displayRole = computed(() => appRoleLabel(user.value?.role ?? 'none'))
const displayRoleColor = computed(() =>
  appRoleColor(user.value?.role ?? 'none'),
)
const accountBlocked = computed(() => isBlockedUser(user.value))
const statusLabel = computed(() =>
  accountBlocked.value ? 'Blocked' : 'Active',
)
const statusColor = computed(() => (accountBlocked.value ? 'error' : 'success'))
const lastLoginLabel = computed(() => formatDateTime(user.value?.lastLoginAt))
const initials = computed(() => {
  const parts = displayName.value
    .split(' ')
    .map((part) => part.trim())
    .filter(Boolean)
    .slice(0, 2)

  if (parts.length === 0) {
    return 'AU'
  }

  return parts.map((part) => part[0]?.toUpperCase() ?? '').join('')
})

const technicalDetails = computed(() => [
  {
    icon: 'mdi-identifier',
    label: 'User ID',
    value: user.value?.id || 'Unavailable',
  },
  {
    icon: 'mdi-fingerprint',
    label: 'OIDC subject',
    value: user.value?.sub || 'Unavailable',
  },
])

const quickActions = [
  {
    icon: 'mdi-monitor-dashboard',
    title: 'Dashboard',
    to: '/dashboard',
  },
  {
    icon: 'mdi-key-outline',
    title: 'Virtual keys',
    to: '/tokens',
  },
  {
    icon: 'mdi-help-circle-outline',
    title: 'Setup guide',
    to: '/help',
  },
]

// logout signs the current user out through the auth store.
async function logout() {
  try {
    await authStore.logout('/login')
  } catch (error) {
    Notify.error(error)
  }
}
</script>

<template>
  <v-container fluid class="app-page profile-page">
    <div class="profile-page__header">
      <div>
        <p class="profile-page__kicker">Account</p>
        <h1 class="profile-page__title">Profile</h1>
        <p class="profile-page__subtitle">
          Review your identity, access level, and current session details.
        </p>
      </div>
    </div>

    <v-row>
      <v-col cols="12" lg="8">
        <ProfileIdentityCard
          :display-email="displayEmail"
          :display-name="displayName"
          :display-username="displayUsername"
          :initials="initials"
          :last-login-label="lastLoginLabel"
          :role-color="displayRoleColor"
          :role-label="displayRole"
          :status-color="statusColor"
          :status-label="statusLabel"
        />
      </v-col>

      <v-col cols="12" lg="4">
        <ProfileQuickActions :actions="quickActions" @logout="logout" />
      </v-col>

      <v-col cols="12" lg="7">
        <ProfileGroupsCard
          :error="profileGroups.error.value"
          :groups="profileGroups.groups.value"
          :loading="profileGroups.loading.value"
        />
      </v-col>

      <v-col cols="12" lg="5">
        <ProfileInfoCard
          icon="mdi-card-account-details-outline"
          title="Technical details"
          subtitle="Identifiers used by authentication services"
        >
          <div class="profile-detail-list">
            <div
              v-for="detail in technicalDetails"
              :key="detail.label"
              class="profile-detail-list__item"
            >
              <v-avatar color="primary" variant="tonal" size="34">
                <v-icon :icon="detail.icon" size="19" />
              </v-avatar>
              <div class="profile-detail-list__copy">
                <span class="profile-detail-list__label">{{
                  detail.label
                }}</span>
                <strong class="profile-detail-list__value">
                  {{ detail.value }}
                </strong>
              </div>
            </div>
          </div>
        </ProfileInfoCard>
      </v-col>
    </v-row>
  </v-container>
</template>

<style scoped>
.profile-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}

.profile-page__kicker {
  margin: 0 0 8px;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.profile-page__title {
  margin: 0;
  font-size: 1.55rem;
  font-weight: 800;
  line-height: 1.2;
}

.profile-page__subtitle {
  max-width: 44rem;
  margin: 6px 0 0;
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.6;
}

.profile-detail-list {
  display: grid;
  gap: 10px;
  padding: 0 24px 24px;
}

.profile-detail-list__item {
  min-width: 0;
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.42);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.58);
}

.profile-detail-list__copy {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.profile-detail-list__label {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 650;
}

.profile-detail-list__value {
  overflow-wrap: anywhere;
  font-size: 0.95rem;
  line-height: 1.4;
}

@media (max-width: 720px) {
  .profile-page__header {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
