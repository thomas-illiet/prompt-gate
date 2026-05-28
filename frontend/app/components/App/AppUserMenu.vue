<script setup lang="ts">
import { storeToRefs } from 'pinia'

import { Notify } from '~/stores/notification'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const displayName = computed(
  () =>
    user.value?.name || user.value?.preferredUsername || 'Authenticated user',
)
const displayEmail = computed(
  () => user.value?.email || 'No email available',
)
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

// openProfile navigates to the authenticated user's profile.
function openProfile() {
  void navigateTo('/profile')
}

// logout signs out through the auth store.
async function logout() {
  try {
    await authStore.logout('/login')
  } catch (error) {
    Notify.error(error)
  }
}
</script>

<template>
  <v-menu location="bottom end" offset="10">
    <template #activator="{ props }">
      <v-btn
        v-bind="props"
        variant="text"
        rounded="xl"
        class="toolbar-user-button text-none me-1 px-3"
        height="48"
      >
        <template #prepend>
          <v-avatar size="32" class="toolbar-user-avatar">
            <span class="toolbar-user-initials">{{ initials }}</span>
          </v-avatar>
        </template>
        <span class="d-none d-md-block">{{ displayName }}</span>
        <template #append>
          <v-icon
            icon="mdi-chevron-down"
            size="18"
            class="text-medium-emphasis"
          />
        </template>
      </v-btn>
    </template>

    <v-card min-width="280" rounded="lg" class="toolbar-panel-card">
      <v-list class="py-1">
        <v-list-item class="toolbar-user-summary">
          <template #prepend>
            <v-avatar size="40" class="toolbar-user-avatar mr-3">
              <span class="toolbar-user-initials toolbar-user-initials--large">
                {{ initials }}
              </span>
            </v-avatar>
          </template>
          <v-list-item-title class="font-weight-bold">
            {{ displayName }}
          </v-list-item-title>
          <v-list-item-subtitle>
            {{ displayEmail }}
          </v-list-item-subtitle>
        </v-list-item>

        <v-divider class="my-2" />

        <v-list-item
          prepend-icon="mdi-account-outline"
          title="Profile"
          rounded="lg"
          class="toolbar-user-action"
          @click="openProfile"
        />
        <v-list-item
          prepend-icon="mdi-logout"
          title="Logout"
          rounded="lg"
          class="toolbar-user-action"
          @click="logout"
        />
      </v-list>
    </v-card>
  </v-menu>
</template>

<style scoped>
.toolbar-user-button {
  min-width: 0;
  color: inherit;
}

.toolbar-user-button:hover {
  background-color: rgba(var(--app-shell-border), 0.08);
}

.toolbar-user-avatar {
  background-color: rgba(var(--app-shell-border), 0.12);
}

.toolbar-user-initials {
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.toolbar-user-initials--large {
  font-size: 0.95rem;
}

.toolbar-panel-card {
  background: linear-gradient(
    180deg,
    rgba(var(--app-shell-surface), 0.98) 0%,
    rgba(var(--app-shell-surface-muted), 0.98) 100%
  );
  border: 1px solid rgba(var(--app-shell-border), 0.42);
  box-shadow: 0 18px 34px -28px
    rgba(var(--app-shell-shadow), var(--app-shell-shadow-strong-opacity));
  backdrop-filter: blur(16px);
}

.toolbar-user-summary {
  min-height: 72px;
  padding-inline: 10px;
}

.toolbar-user-action {
  min-height: 44px;
}

:deep(.toolbar-user-button .v-btn__prepend) {
  margin-inline-end: 12px;
}

:deep(.toolbar-user-button .v-btn__append) {
  margin-inline-start: 8px;
}
</style>
