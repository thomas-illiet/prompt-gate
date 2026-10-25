<script setup lang="ts">
import { storeToRefs } from 'pinia'

import LoginPanel from '~/components/Login/LoginPanel.vue'
import LoginVisual from '~/components/Login/LoginVisual.vue'
import { Notify } from '~/stores/notification'
import {
  DEFAULT_AUTH_REDIRECT_PATH,
  normalizeRedirectPath,
  publicLoginErrorMessage,
} from '~/utils/auth'

definePageMeta({
  auth: false,
  layout: 'public',
  title: 'Welcome',
})

const route = useRoute()
const authStore = useAuthStore()
const { initializationError, isAuthenticated, isConfigured, isReady, user } =
  storeToRefs(authStore)
const loginPending = shallowRef(false)

const redirectPath = computed(() =>
  normalizeRedirectPath(route.query.redirect, DEFAULT_AUTH_REDIRECT_PATH),
)
const canLogin = computed(
  () => isConfigured.value && isReady.value && !loginPending.value,
)
const canRetry = computed(
  () => isConfigured.value && isReady.value && !loginPending.value,
)
const displayedError = computed(() =>
  publicLoginErrorMessage(
    route.query.authError,
    Boolean(initializationError.value),
  ),
)
const sessionLabel = computed(
  () => user.value?.name || user.value?.preferredUsername || '',
)
const visualStatus = computed(() => {
  if (!isReady.value) {
    return { label: 'Checking control room', tone: 'checking' } as const
  }

  if (!isConfigured.value || initializationError.value) {
    return { label: 'Control room unavailable', tone: 'unavailable' } as const
  }

  return { label: 'Control room online', tone: 'online' } as const
})

watchEffect(() => {
  if (!isReady.value || !isAuthenticated.value) {
    return
  }

  void navigateTo(redirectPath.value)
})

// signIn starts the backend login flow with the requested redirect path.
async function signIn() {
  loginPending.value = true

  try {
    await authStore.login(redirectPath.value)
  } catch (error) {
    Notify.error(error)
    loginPending.value = false
  }
}

// retrySession rechecks backend availability without leaving the page.
async function retrySession() {
  loginPending.value = true

  try {
    const authenticated = await authStore.refresh()
    if (authenticated) {
      await navigateTo(redirectPath.value)
    }
  } finally {
    loginPending.value = false
  }
}
</script>

<template>
  <v-container fluid class="login-page">
    <div class="login-composition">
      <LoginVisual
        :status-label="visualStatus.label"
        :status-tone="visualStatus.tone"
      />
      <LoginPanel
        :can-login="canLogin"
        :can-retry="canRetry"
        :displayed-error="displayedError"
        :is-authenticated="isAuthenticated"
        :is-ready="isReady"
        :login-pending="loginPending"
        :status-label="visualStatus.label"
        :user-label="sessionLabel"
        @retry="retrySession"
        @sign-in="signIn"
      />
    </div>
  </v-container>
</template>

<style scoped>
.login-page {
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: clamp(16px, 4vw, 52px);
}

.login-composition {
  position: relative;
  display: grid;
  width: min(1180px, 100%);
  min-height: min(650px, calc(100vh - clamp(32px, 8vw, 104px)));
  grid-template-columns: minmax(0, 1.16fr) minmax(390px, 0.84fr);
  overflow: hidden;
  border: 2px solid rgba(var(--app-shell-text-primary), 0.2);
  border-radius: clamp(24px, 3vw, 38px);
  background: rgb(var(--app-shell-surface));
  box-shadow:
    12px 14px 0 rgba(var(--app-accent-coral), 0.13),
    -8px -8px 0 rgba(var(--app-accent-butter), 0.13),
    0 44px 92px -48px
      rgba(var(--app-shell-shadow), var(--app-shell-shadow-strong-opacity));
}

@media (max-width: 960px) {
  .login-composition {
    min-height: 0;
    grid-template-columns: 1fr;
  }
}

@media (max-width: 600px) {
  .login-page {
    align-items: start;
    padding: 10px;
  }

  .login-composition {
    border-width: 1px;
    border-radius: 24px;
    box-shadow:
      5px 7px 0 rgba(var(--app-accent-coral), 0.11),
      0 24px 56px -38px rgba(var(--app-shell-shadow), 0.28);
  }
}
</style>
