<script setup lang="ts">
import { storeToRefs } from 'pinia'

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
const visualStatus = computed(() => {
  if (!isReady.value) {
    return { label: 'Checking control room', tone: 'checking' }
  }

  if (!isConfigured.value || initializationError.value) {
    return { label: 'Control room unavailable', tone: 'unavailable' }
  }

  return { label: 'Control room online', tone: 'online' }
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
      <section class="login-visual" aria-hidden="true">
        <div class="login-visual__mark">
          <AppSignalVisual name="gate" />
        </div>
        <div class="login-visual__wash" />
        <div
          class="login-visual__status"
          :class="`login-visual__status--${visualStatus.tone}`"
          data-test="login-visual-status"
          :data-tone="visualStatus.tone"
        >
          <span class="login-visual__status-ping" />
          <span>{{ visualStatus.label }}</span>
        </div>
        <div class="login-visual__caption">
          <span class="login-visual__eyebrow">PromptGate control room</span>
          <strong>One gate. Every model route.</strong>
        </div>
      </section>

      <v-card class="login-card" rounded="xl">
        <v-card-text class="login-card__content pa-7 pa-sm-9 pa-md-10">
          <div class="login-brand">
            <v-avatar size="58" class="login-mark-avatar">
              <v-icon icon="custom:promptgate-mark" size="34" color="primary" />
            </v-avatar>
            <div class="login-brand__copy">
              <span>Prompt<span class="text-primary">Gate</span></span>
              <small>Secure model access</small>
            </div>
          </div>

          <div class="login-copy">
            <p class="login-kicker">Your shift starts here</p>
            <h1>The gate is awake.</h1>
            <p class="login-description">
              Enter the control room for a direct, governed lane to every model
              your team is allowed to use.
            </p>
          </div>

          <v-alert
            v-if="displayedError"
            type="warning"
            variant="tonal"
            rounded="lg"
            class="login-alert"
          >
            {{ displayedError }}
            <template #append>
              <v-btn
                variant="text"
                size="small"
                :disabled="!canRetry"
                @click="retrySession"
              >
                Retry
              </v-btn>
            </template>
          </v-alert>

          <div v-if="isAuthenticated && user" class="login-session rounded-lg">
            <v-icon icon="mdi-account-check-outline" size="22" />
            <div>
              <span class="login-session-label">Connected as</span>
              <strong class="login-session-value">
                {{ user.name || user.preferredUsername }}
              </strong>
            </div>
          </div>

          <v-btn
            block
            size="x-large"
            color="primary"
            rounded="xl"
            class="login-action text-none"
            append-icon="mdi-arrow-right"
            :loading="loginPending || !isReady"
            :disabled="!canLogin"
            @click="signIn"
          >
            Enter the control room
          </v-btn>

          <div class="login-assurance">
            <v-icon icon="mdi-shield-lock-outline" size="17" />
            <span>Authentication stays with your organization.</span>
          </div>
        </v-card-text>
      </v-card>
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
  min-height: min(640px, calc(100vh - clamp(32px, 8vw, 104px)));
  grid-template-columns: minmax(0, 1.2fr) minmax(390px, 0.8fr);
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.54);
  border-radius: clamp(22px, 3vw, 34px);
  background: rgb(var(--app-shell-surface));
  box-shadow:
    0 40px 90px -48px
      rgba(var(--app-shell-shadow), var(--app-shell-shadow-strong-opacity)),
    0 12px 32px -24px rgba(var(--v-theme-primary), 0.3);
}

.login-visual {
  position: relative;
  display: grid;
  min-height: 520px;
  place-items: center;
  overflow: hidden;
  background:
    radial-gradient(
      circle at 24% 18%,
      rgba(118, 236, 174, 0.22),
      transparent 25%
    ),
    radial-gradient(
      circle at 82% 76%,
      rgba(255, 130, 111, 0.18),
      transparent 24%
    ),
    linear-gradient(145deg, rgb(16, 65, 47), rgb(7, 31, 24));
}

.login-visual::before {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.035) 1px, transparent 1px);
  background-size: 36px 36px;
  content: '';
  mask-image: radial-gradient(circle at center, #000, transparent 78%);
}

.login-visual__mark {
  position: relative;
  z-index: 1;
  width: clamp(220px, 32vw, 350px);
  height: clamp(220px, 32vw, 350px);
  --app-accent-coral: 255, 130, 111;
  --app-shell-surface-strong: 12, 52, 39;
  --v-theme-primary: 118, 236, 174;
  filter: drop-shadow(0 30px 42px rgba(0, 0, 0, 0.28));
  animation: login-visual-arrive 720ms var(--app-motion-emphasized, ease-out)
    both;
}

.login-visual__wash {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, transparent 48%, rgba(7, 31, 24, 0.62)),
    linear-gradient(90deg, rgba(0, 150, 94, 0.08), transparent 56%);
}

.login-visual__status,
.login-visual__caption {
  position: absolute;
  z-index: 1;
  color: white;
}

.login-visual__status {
  top: 22px;
  left: 22px;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 13px;
  border: 1px solid rgba(255, 255, 255, 0.26);
  border-radius: 999px;
  background: rgba(7, 31, 24, 0.52);
  box-shadow: 0 16px 32px -22px rgba(0, 0, 0, 0.7);
  font-size: 0.76rem;
  font-weight: 750;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  backdrop-filter: blur(12px);
}

.login-visual__status-ping {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgb(118, 236, 174);
  box-shadow: 0 0 0 0 rgba(118, 236, 174, 0.55);
  animation: login-status-ping 2.4s ease-out infinite;
}

.login-visual__status--checking .login-visual__status-ping {
  background: rgb(var(--v-theme-warning));
  box-shadow: 0 0 0 0 rgba(var(--v-theme-warning), 0.48);
}

.login-visual__status--unavailable .login-visual__status-ping {
  background: rgb(var(--v-theme-error));
  box-shadow: none;
  animation: none;
}

.login-visual__caption {
  right: clamp(22px, 4vw, 44px);
  bottom: clamp(24px, 4vw, 42px);
  left: clamp(22px, 4vw, 44px);
  display: grid;
  max-width: 29rem;
  gap: 7px;
  text-shadow: 0 2px 20px rgba(0, 0, 0, 0.28);
}

.login-visual__caption strong {
  font-size: clamp(1.65rem, 3vw, 2.55rem);
  line-height: 1.03;
  letter-spacing: -0.035em;
}

.login-visual__eyebrow {
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.login-card {
  display: grid;
  align-content: center;
  border: 0;
  border-radius: 0 !important;
  background: linear-gradient(
    160deg,
    rgba(var(--app-shell-surface-strong), 0.99),
    rgba(var(--app-shell-surface), 0.98) 62%,
    rgba(var(--v-theme-primary), 0.045)
  );
  box-shadow: none;
}

.login-card__content {
  display: grid;
  align-content: center;
  gap: 22px;
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 13px;
}

.login-brand__copy {
  display: grid;
  gap: 1px;
}

.login-brand__copy > span {
  color: rgb(var(--app-shell-text-primary));
  font-size: 1.35rem;
  font-weight: 850;
  letter-spacing: -0.03em;
}

.login-brand__copy small {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.login-copy {
  display: grid;
  gap: 12px;
}

.login-mark-avatar {
  background: radial-gradient(
    circle at 30% 30%,
    rgba(var(--v-theme-primary), 0.2),
    rgba(var(--v-theme-primary), 0.1) 58%,
    rgba(255, 255, 255, 0.86) 100%
  );
  border: 1px solid rgba(var(--app-shell-border), 0.18);
  box-shadow: 0 18px 34px -25px rgba(var(--app-shell-shadow), 0.42);
}

.login-kicker {
  margin: 0;
  font-size: 0.76rem;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgb(var(--v-theme-primary));
}

.login-copy h1 {
  margin: 0;
  color: rgb(var(--app-shell-text-primary));
  font-size: clamp(2rem, 3vw, 3rem);
  font-weight: 850;
  line-height: 1.02;
  letter-spacing: -0.045em;
}

.login-description {
  margin: 0;
  max-width: 28rem;
  line-height: 1.65;
  font-size: 1.02rem;
  color: rgb(var(--app-shell-text-secondary));
}

.login-alert {
  margin: 0;
}

.login-session {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid rgba(var(--v-theme-primary), 0.16);
  border-radius: var(--app-card-radius);
  background: rgba(var(--v-theme-primary), 0.08);
  color: rgb(var(--v-theme-primary));
}

.login-session > div {
  display: grid;
  gap: 3px;
}

.login-session-label {
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: rgb(var(--app-shell-text-muted));
}

.login-session-value {
  color: rgb(var(--app-shell-text-primary));
  font-size: 1rem;
}

.login-action {
  min-height: 56px;
  box-shadow: 0 20px 38px -24px rgba(var(--v-theme-primary), 0.82) !important;
}

.login-assurance {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 7px;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
  text-align: center;
}

@keyframes login-visual-arrive {
  from {
    opacity: 0;
    transform: scale(1.035);
  }
}

@keyframes login-status-ping {
  55%,
  100% {
    box-shadow: 0 0 0 9px rgba(118, 236, 174, 0);
  }
}

@media (max-width: 960px) {
  .login-composition {
    min-height: 0;
    grid-template-columns: 1fr;
  }

  .login-visual {
    min-height: clamp(250px, 40vw, 360px);
  }

  .login-card__content {
    gap: 24px;
  }

  .login-copy h1 {
    font-size: clamp(2rem, 7vw, 2.8rem);
  }
}

@media (max-width: 600px) {
  .login-page {
    align-items: start;
    padding: 10px;
  }

  .login-composition {
    border-radius: 22px;
  }

  .login-visual {
    min-height: 210px;
  }

  .login-visual__status {
    top: 14px;
    left: 14px;
  }

  .login-visual__caption {
    right: 20px;
    bottom: 20px;
    left: 20px;
  }

  .login-visual__caption strong {
    font-size: 1.55rem;
  }

  .login-brand__copy > span {
    font-size: 1.2rem;
  }

  .login-copy h1 {
    font-size: 2rem;
  }

  .login-alert :deep(.v-alert__content) {
    min-width: 0;
  }

  .login-alert {
    grid-template-columns: 32px minmax(0, 1fr);
    grid-template-areas:
      'prepend content'
      '. append';
  }

  .login-alert :deep(.v-alert__prepend) {
    align-self: start;
  }

  .login-alert :deep(.v-alert__append) {
    justify-self: start;
    margin-top: 4px;
    margin-inline-start: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .login-visual__mark,
  .login-visual__status-ping {
    animation: none;
  }
}
</style>
