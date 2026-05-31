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
    subtitle: 'Open usage overview',
    to: '/dashboard',
  },
  {
    icon: 'mdi-key-outline',
    title: 'Virtual keys',
    subtitle: 'Manage personal access',
    to: '/tokens',
  },
  {
    icon: 'mdi-help-circle-outline',
    title: 'Setup guide',
    subtitle: 'Configure clients',
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
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-account-circle-outline"
          kicker="Account"
          title="Profile"
          copy="Review your identity, access level, and current session details."
          stat-label="Status"
          :stat-value="statusLabel"
        />
      </v-col>

      <v-col cols="12" lg="5">
        <ProfileIdentityCard
          :display-email="displayEmail"
          :display-name="displayName"
          :display-username="displayUsername"
          :initials="initials"
        />
      </v-col>

      <v-col cols="12" md="6" lg="3">
        <ProfileInfoCard
          icon="mdi-shield-account-outline"
          title="Access"
          subtitle="Role and account state"
        >
          <v-list density="comfortable" class="profile-list">
            <v-list-item title="Role">
              <template #prepend>
                <v-icon icon="mdi-account-key-outline" />
              </template>
              <template #append>
                <v-chip label variant="tonal" :color="displayRoleColor">
                  {{ displayRole }}
                </v-chip>
              </template>
            </v-list-item>

            <v-list-item title="Status">
              <template #prepend>
                <v-icon icon="mdi-check-decagram-outline" />
              </template>
              <template #append>
                <v-chip label variant="tonal" :color="statusColor">
                  {{ statusLabel }}
                </v-chip>
              </template>
            </v-list-item>
          </v-list>
        </ProfileInfoCard>
      </v-col>

      <v-col cols="12" md="6" lg="4">
        <ProfileInfoCard
          icon="mdi-clock-outline"
          title="Session"
          subtitle="Latest known login"
        >
          <div class="profile-session">
            <v-icon icon="mdi-login-variant" color="primary" size="40" />
            <div>
              <p>Last login</p>
              <strong>{{ lastLoginLabel }}</strong>
            </div>
          </div>
        </ProfileInfoCard>
      </v-col>

      <v-col cols="12" lg="7">
        <ProfileGroupsCard
          :error="profileGroups.error.value"
          :groups="profileGroups.groups.value"
          :loading="profileGroups.loading.value"
        />
      </v-col>

      <v-col cols="12" lg="5">
        <ProfileQuickActions :actions="quickActions" @logout="logout" />
      </v-col>

      <v-col cols="12">
        <ProfileInfoCard
          icon="mdi-card-account-details-outline"
          title="Technical details"
          subtitle="Identifiers used by authentication services"
        >
          <v-list density="comfortable" class="profile-list">
            <v-list-item v-for="detail in technicalDetails" :key="detail.label">
              <template #prepend>
                <v-icon :icon="detail.icon" />
              </template>
              <v-list-item-title>{{ detail.label }}</v-list-item-title>
              <v-list-item-subtitle class="profile-list__value">
                {{ detail.value }}
              </v-list-item-subtitle>
            </v-list-item>
          </v-list>
        </ProfileInfoCard>
      </v-col>
    </v-row>
  </v-container>
</template>

<style scoped>
.profile-session {
  min-height: 160px;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px;
}

.profile-session p {
  margin: 0 0 6px;
  color: rgb(var(--app-shell-text-secondary));
}

.profile-session strong {
  display: block;
  overflow-wrap: anywhere;
  font-size: 1.2rem;
}

.profile-list {
  padding: 8px;
}

.profile-list__value {
  overflow-wrap: anywhere;
  white-space: normal;
}
</style>
