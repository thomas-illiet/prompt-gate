<script setup lang="ts">
import { storeToRefs } from 'pinia'

import { appRoleColor, appRoleLabel, isBlockedUser } from '~/utils/auth'
import { formatDateTime } from '~/utils/formatters'

definePageMeta({
  title: 'Profile',
  requiredRoles: ['limited', 'user', 'admin'],
})

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
const profileGroups = useProfileGroups()
const profileQuotaResets = useProfileQuotaResets()
const useQuotaResetDialogOpen = shallowRef(false)

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

const quotaResetConfirmationMessage = computed(() => {
  const available = profileQuotaResets.summary.value?.available ?? 0
  const remaining = Math.max(0, available - 1)

  return `Use one quota reset? This clears the live 5-hour and 7-day quota windows while preserving usage analytics. ${remaining} reset${remaining === 1 ? '' : 's'} will remain.`
})

async function confirmQuotaReset() {
  try {
    const response = await profileQuotaResets.useReset()
    if (response) {
      useQuotaResetDialogOpen.value = false
    }
  } catch {
    // The composable reports mutation failures through the notification store.
    if (profileQuotaResets.summary.value?.pendingReset) {
      useQuotaResetDialogOpen.value = false
    }
  }
}
</script>

<template>
  <v-container fluid class="app-page profile-page">
    <v-row>
      <v-col cols="12">
        <AppPageHeader
          icon="mdi-account-circle-outline"
          kicker="Access badge"
          title="Profile"
          copy="Your identity, access groups, and quota safety controls — all in one place."
          visual="identity"
          stat-label="Account status"
          :stat-value="statusLabel"
        />
      </v-col>

      <v-col cols="12">
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

      <v-col cols="12">
        <ProfileQuotaResetCard
          :error="profileQuotaResets.error.value"
          :loading="profileQuotaResets.loading.value"
          :summary="profileQuotaResets.summary.value"
          :using="profileQuotaResets.using.value"
          @retry="profileQuotaResets.reload"
          @show-history="profileQuotaResets.openHistory"
          @use="useQuotaResetDialogOpen = true"
        />
      </v-col>

      <v-col cols="12" lg="7">
        <ProfileGroupsCard
          :error="profileGroups.error.value"
          :groups="profileGroups.groups.value"
          :loading="profileGroups.loading.value"
          @retry="profileGroups.reload"
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

    <ProfileQuotaResetHistoryDialog
      v-model="profileQuotaResets.historyOpen.value"
      :error="profileQuotaResets.historyError.value"
      :items="profileQuotaResets.history.value"
      :loading="profileQuotaResets.historyLoading.value"
      :maximum="profileQuotaResets.summary.value?.maximum ?? 2"
      :page="profileQuotaResets.historyPage.value"
      :page-size="profileQuotaResets.historyPageSize.value"
      :total="profileQuotaResets.historyTotal.value"
      @refresh="profileQuotaResets.loadHistory"
      @update:page="profileQuotaResets.setHistoryPage"
      @update:page-size="profileQuotaResets.setHistoryPageSize"
    />

    <AppConfirmDialog
      v-model="useQuotaResetDialogOpen"
      confirm-label="Use reset"
      icon="mdi-refresh"
      :loading="profileQuotaResets.using.value"
      :message="quotaResetConfirmationMessage"
      title="Use a quota reset"
      @cancel="useQuotaResetDialogOpen = false"
      @confirm="confirmQuotaReset"
    />
  </v-container>
</template>

<style scoped>
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
</style>
