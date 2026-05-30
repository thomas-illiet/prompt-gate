<script setup lang="ts">
import { storeToRefs } from 'pinia'

import { Notify } from '~/stores/notification'
import { DEFAULT_AUTH_REDIRECT_PATH, normalizeRedirectPath } from '~/utils/auth'

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
const authError = computed(() => {
  if (typeof route.query.authErrorDescription === 'string') {
    return route.query.authErrorDescription
  }

  if (typeof route.query.authError === 'string') {
    return route.query.authError
  }

  return null
})
const displayedError = computed(
  () => authError.value || initializationError.value,
)

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
</script>

<template>
  <v-container class="login-page fill-height">
    <v-row justify="center" align="center" class="fill-height">
      <v-col cols="12" sm="10" md="7" lg="5" xl="4">
        <v-card class="login-card" rounded="xl">
          <v-card-text class="pa-8 pa-md-10">
            <div class="login-copy">
              <div class="login-mark">
                <v-avatar size="78" class="login-mark-avatar">
                  <v-icon icon="custom:vitify-nuxt" size="42" color="primary" />
                </v-avatar>
              </div>
              <p class="login-kicker">Built for developers</p>
              <p class="login-description">
                Give your team a direct lane to
                <span class="login-highlight">LLM access</span> so they can
                prototype faster, ship smarter, and build what comes next.
              </p>
            </div>

            <v-alert
              v-if="displayedError"
              type="warning"
              variant="tonal"
              rounded="lg"
              class="mt-6"
            >
              {{ displayedError }}
            </v-alert>

            <div
              v-if="isAuthenticated && user"
              class="login-session mt-6 rounded-lg"
            >
              <span class="login-session-label">Connected as</span>
              <strong class="login-session-value">
                {{ user.name || user.preferredUsername }}
              </strong>
            </div>

            <v-btn
              block
              size="x-large"
              color="primary"
              rounded="xl"
              class="mt-8 text-none"
              :loading="loginPending || !isReady"
              :disabled="!canLogin"
              @click="signIn"
            >
              Start building
            </v-btn>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
}

.login-card {
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    linear-gradient(
      180deg,
      rgba(var(--app-shell-surface-strong), 0.98) 0%,
      rgba(var(--app-shell-surface), 0.98) 58%,
      rgba(var(--app-shell-surface-muted), 0.9) 100%
    ),
    rgb(var(--app-shell-surface-strong));
  box-shadow: var(--app-card-shadow);
}

.login-copy {
  display: grid;
  gap: 10px;
  justify-items: center;
  text-align: center;
}

.login-mark {
  margin-bottom: 10px;
}

.login-mark-avatar {
  background: radial-gradient(
    circle at 30% 30%,
    rgba(var(--v-theme-primary), 0.2),
    rgba(var(--v-theme-primary), 0.1) 58%,
    rgba(255, 255, 255, 0.86) 100%
  );
  border: 1px solid rgba(var(--app-shell-border), 0.18);
  box-shadow: 0 20px 40px -26px rgba(var(--app-shell-shadow), 0.38);
}

.login-kicker {
  margin: 0;
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgb(var(--app-shell-text-muted));
}

.login-description {
  margin: 0;
  max-width: 32rem;
  line-height: 1.6;
  font-size: 1rem;
  color: rgb(var(--app-shell-text-secondary));
}

.login-highlight {
  font-weight: 700;
  color: rgb(var(--v-theme-primary));
}

.login-session {
  display: grid;
  gap: 6px;
  padding: 16px 18px;
  border: 1px solid rgba(var(--v-theme-primary), 0.16);
  border-radius: var(--app-card-radius);
  background: rgba(var(--v-theme-primary), 0.08);
}

.login-session-label {
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(var(--app-shell-text-muted));
}

.login-session-value {
  font-size: 1rem;
}
</style>
