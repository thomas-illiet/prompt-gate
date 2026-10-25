<script setup lang="ts">
const props = defineProps<{
  canLogin: boolean
  canRetry: boolean
  displayedError?: string | null
  isAuthenticated: boolean
  isReady: boolean
  loginPending: boolean
  statusLabel: string
  userLabel: string
}>()

const emit = defineEmits<{
  retry: []
  signIn: []
}>()
</script>

<template>
  <v-card class="login-card" rounded="xl">
    <v-card-text class="login-card__content pa-7 pa-sm-9 pa-md-10">
      <span class="login-card__live-status" role="status" aria-live="polite">
        {{ props.statusLabel }}
      </span>

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
        <p class="login-kicker">Welcome to the green room</p>
        <h1>Your models are ready.</h1>
        <p class="login-description">
          Sign in once and get building. Everything between you and the right
          model is already in place.
        </p>
      </div>

      <div
        class="login-card__reserved-space"
        aria-hidden="true"
        data-test="login-content-reserved-space"
      />

      <v-alert
        v-if="props.displayedError"
        type="warning"
        variant="tonal"
        rounded="lg"
        class="login-alert"
      >
        {{ props.displayedError }}
        <template #append>
          <v-btn
            variant="text"
            size="small"
            :disabled="!props.canRetry"
            @click="emit('retry')"
          >
            Retry
          </v-btn>
        </template>
      </v-alert>

      <div
        v-if="props.isAuthenticated && props.userLabel"
        class="login-session rounded-lg"
      >
        <v-icon icon="mdi-account-check-outline" size="22" />
        <div>
          <span class="login-session-label">Connected as</span>
          <strong class="login-session-value">{{ props.userLabel }}</strong>
        </div>
      </div>

      <v-btn
        block
        size="x-large"
        color="primary"
        rounded="xl"
        class="login-action text-none"
        append-icon="mdi-arrow-right"
        :loading="props.loginPending || !props.isReady"
        :disabled="!props.canLogin"
        @click="emit('signIn')"
      >
        Open PromptGate
      </v-btn>

      <div class="login-assurance">
        <v-icon icon="mdi-shield-lock-outline" size="17" />
        <span>Authentication stays with your organization.</span>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.login-card {
  position: relative;
  display: grid;
  height: 100%;
  min-height: 100%;
  align-self: stretch;
  align-content: stretch;
  border: 0;
  border-radius: 0 !important;
  background:
    radial-gradient(
      circle at 92% 8%,
      rgba(var(--app-accent-butter), 0.18),
      transparent 24%
    ),
    linear-gradient(
      160deg,
      rgba(var(--app-shell-surface-strong), 0.99),
      rgba(var(--app-shell-surface), 0.98) 62%,
      rgba(var(--v-theme-primary), 0.055)
    );
  box-shadow: none;
}

.login-card::after {
  position: absolute;
  top: 24px;
  right: 26px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: rgb(var(--app-accent-coral));
  box-shadow:
    20px 12px 0 -2px rgb(var(--app-accent-butter)),
    5px 28px 0 -4px rgba(var(--v-theme-primary), 0.55);
  content: '';
  opacity: 0.72;
}

.login-card__content {
  display: flex;
  height: 100%;
  min-height: 100%;
  flex-direction: column;
  gap: 22px;
}

.login-card__live-status {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  white-space: nowrap;
}

.login-card__reserved-space {
  width: 100%;
  max-width: 420px;
  height: 122px;
  flex: 0 0 122px;
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
  font-weight: 750;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.login-copy {
  display: grid;
  gap: 12px;
  margin-top: auto;
}

.login-mark-avatar {
  transform: rotate(-4deg);
  border: 1px solid rgba(var(--v-theme-primary), 0.18);
  background: radial-gradient(
    circle at 30% 30%,
    rgba(var(--app-accent-butter), 0.55),
    rgba(var(--v-theme-primary), 0.13) 60%,
    rgba(255, 255, 255, 0.9) 100%
  );
  box-shadow:
    4px 5px 0 rgba(var(--app-accent-coral), 0.16),
    0 18px 34px -25px rgba(var(--app-shell-shadow), 0.42);
}

.login-kicker {
  margin: 0;
  color: rgb(var(--v-theme-primary));
  font-family: var(--app-font-mono);
  font-size: 0.74rem;
  font-weight: 850;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}

.login-copy h1 {
  margin: 0;
  color: rgb(var(--app-shell-text-primary));
  font-size: clamp(2.2rem, 3.4vw, 3.35rem);
  font-weight: 900;
  line-height: 0.98;
  letter-spacing: -0.055em;
}

.login-description {
  margin: 0;
  max-width: 29rem;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 1.02rem;
  line-height: 1.65;
}

.login-alert {
  margin: 0;
}

.login-session {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid rgba(var(--v-theme-primary), 0.18);
  border-radius: var(--app-card-radius);
  background: rgba(var(--v-theme-primary), 0.08);
  color: rgb(var(--v-theme-primary));
}

.login-session > div {
  display: grid;
  gap: 3px;
}

.login-session-label {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.login-session-value {
  color: rgb(var(--app-shell-text-primary));
  font-size: 1rem;
}

.login-action {
  flex: 0 0 auto;
  min-height: 58px;
  margin-top: auto;
  box-shadow:
    0 5px 0 rgba(8, 85, 58, 0.24),
    0 22px 38px -24px rgba(var(--v-theme-primary), 0.78) !important;
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

@media (max-width: 960px) {
  .login-card__content {
    gap: 24px;
  }

  .login-copy,
  .login-action {
    margin-top: 0;
  }

  .login-copy h1 {
    font-size: clamp(2.1rem, 7vw, 3rem);
  }
}

@media (max-width: 600px) {
  .login-card::after {
    top: 20px;
    right: 22px;
    transform: scale(0.8);
  }

  .login-brand__copy > span {
    font-size: 1.2rem;
  }

  .login-copy h1 {
    font-size: 2.2rem;
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
</style>
