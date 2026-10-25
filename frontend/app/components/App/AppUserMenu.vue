<script setup lang="ts">
import { storeToRefs } from 'pinia'

import { Notify } from '~/stores/notification'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const displayName = computed(
  () =>
    user.value?.name || user.value?.preferredUsername || 'Authenticated user',
)
const displayEmail = computed(() => user.value?.email || 'No email available')
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
        :aria-label="`Open user menu for ${displayName}`"
        variant="text"
        rounded="xl"
        class="toolbar-user-button text-none me-1 px-3"
        height="48"
        data-test="user-menu-trigger"
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

    <v-card min-width="300" rounded="xl" class="toolbar-panel-card">
      <v-list class="py-1">
        <v-list-item class="toolbar-user-summary">
          <template #prepend>
            <v-avatar size="40" class="toolbar-user-avatar mr-3">
              <span class="toolbar-user-initials toolbar-user-initials--large">
                {{ initials }}
              </span>
            </v-avatar>
          </template>
          <div class="toolbar-user-summary__copy">
            <span class="toolbar-user-summary__eyebrow">Signed in</span>
            <strong>{{ displayName }}</strong>
            <span>{{ displayEmail }}</span>
          </div>
          <template #append>
            <span class="toolbar-user-summary__beacon" aria-hidden="true" />
          </template>
        </v-list-item>

        <v-divider class="my-2" />

        <v-list-item
          prepend-icon="mdi-account-outline"
          title="Profile"
          rounded="lg"
          class="toolbar-user-action toolbar-user-action--profile"
          data-test="user-menu-profile"
          @click="openProfile"
        />
        <v-list-item
          prepend-icon="mdi-logout"
          title="Logout"
          rounded="lg"
          base-color="error"
          class="toolbar-user-action toolbar-user-action--logout"
          data-test="user-menu-logout"
          @click="logout"
        />
      </v-list>
    </v-card>
  </v-menu>
</template>

<style scoped>
.toolbar-user-button {
  min-width: 0;
  border: 1px solid rgba(var(--app-shell-border), 0.62);
  background: linear-gradient(
    100deg,
    rgba(var(--app-accent-butter), 0.1),
    rgba(var(--app-shell-surface-strong), 0.78)
  );
  color: rgb(var(--app-shell-text-primary));
  box-shadow: 3px 4px 0 rgba(var(--app-accent-coral), 0.07);
}

.toolbar-user-button:hover {
  border-color: rgba(var(--v-theme-primary), 0.28);
  background-color: rgba(var(--app-accent-butter), 0.13);
}

.toolbar-user-avatar {
  transform: rotate(-3deg);
  border: 1px solid rgba(var(--v-theme-primary), 0.18);
  background:
    radial-gradient(
      circle at 28% 24%,
      rgba(var(--app-accent-butter), 0.56),
      transparent 54%
    ),
    rgba(var(--v-theme-primary), 0.1);
  color: rgb(var(--v-theme-primary));
  box-shadow: 2px 3px 0 rgba(var(--app-accent-coral), 0.12);
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
  min-height: 86px;
  margin: 5px;
  padding-inline: 11px;
  border: 1px solid rgba(var(--v-theme-primary), 0.1);
  border-radius: 17px !important;
  background:
    radial-gradient(
      circle at 92% 8%,
      rgba(var(--app-accent-butter), 0.2),
      transparent 36%
    ),
    rgba(var(--v-theme-primary), 0.045);
}

.toolbar-user-summary__copy {
  display: grid;
  min-width: 0;
  gap: 1px;
}

.toolbar-user-summary__copy strong,
.toolbar-user-summary__copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.toolbar-user-summary__copy strong {
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.94rem;
}

.toolbar-user-summary__copy > span:not(.toolbar-user-summary__eyebrow) {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.72rem;
}

.toolbar-user-summary__eyebrow {
  color: rgb(var(--v-theme-primary));
  font-family: var(--app-font-mono);
  font-size: 0.59rem;
  font-weight: 800;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.toolbar-user-summary__beacon {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgb(var(--v-theme-primary));
  box-shadow: 0 0 0 4px rgba(var(--v-theme-primary), 0.1);
}

.toolbar-user-action {
  min-height: 44px;
  margin: 2px 5px;
  font-weight: 700;
}

.toolbar-user-action--profile {
  color: rgb(var(--app-shell-text-secondary));
}

.toolbar-user-action--logout:hover {
  background: rgba(var(--v-theme-error), 0.08);
}

:deep(.toolbar-user-button .v-btn__prepend) {
  margin-inline-end: 12px;
}

:deep(.toolbar-user-button .v-btn__append) {
  margin-inline-start: 8px;
}
</style>
