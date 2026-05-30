<script setup lang="ts">
import { storeToRefs } from 'pinia'

definePageMeta({
  allowBlocked: true,
  layout: 'public',
  title: 'Access denied',
})

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const message = computed(() => {
  if (!user.value?.isActive) {
    return 'Your account is inactive. An administrator needs to reactivate it before you can continue.'
  }
  if (user.value?.role === 'none') {
    return 'Your account is connected, but it still needs an application role before you can access the platform.'
  }
  return 'You are signed in, but you do not have permission to open this section.'
})

// logout signs the blocked user out through the auth store.
async function logout() {
  await authStore.logout('/login')
}
</script>

<template>
  <v-container class="access-denied-page fill-height">
    <v-row justify="center" align="center" class="fill-height">
      <v-col cols="12" sm="10" md="7" lg="5" xl="4">
        <v-card rounded="xl" class="access-denied-card">
          <v-card-text class="pa-8 pa-md-10">
            <div class="access-denied-copy">
              <div class="access-denied-mark">
                <v-avatar size="78" class="access-denied-avatar">
                  <v-icon
                    icon="mdi-shield-alert-outline"
                    size="42"
                    color="warning"
                  />
                </v-avatar>
              </div>
              <p class="access-denied-kicker">Restricted workspace</p>
              <h1 class="access-denied-title">Access denied</h1>
              <p class="access-denied-description">
                {{ message }}
              </p>
              <div class="access-denied-scan">
                <div class="access-denied-scan__bars" aria-hidden="true">
                  <span />
                  <span />
                  <span />
                  <span />
                </div>
                <div class="access-denied-scan__status">
                  <v-icon icon="mdi-lock-clock-outline" size="18" />
                  <span>Access check paused</span>
                </div>
              </div>
            </div>

            <div class="access-denied-actions mt-8">
              <v-btn
                block
                size="x-large"
                variant="tonal"
                rounded="xl"
                prepend-icon="mdi-logout"
                class="text-none"
                @click="logout"
              >
                Logout
              </v-btn>
            </div>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<style scoped>
.access-denied-page {
  min-height: 100vh;
}

.access-denied-card {
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background: linear-gradient(
    180deg,
    rgba(var(--app-shell-surface-strong), 0.98) 0%,
    rgba(var(--app-shell-surface), 0.98) 58%,
    rgba(var(--app-shell-surface-muted), 0.9) 100%
  );
  box-shadow: var(--app-card-shadow);
}

.access-denied-copy {
  display: grid;
  gap: 10px;
  justify-items: center;
  text-align: center;
}

.access-denied-mark {
  margin-bottom: 10px;
}

.access-denied-avatar {
  background: radial-gradient(
    circle at 30% 30%,
    rgba(var(--v-theme-warning), 0.22),
    rgba(var(--v-theme-warning), 0.1) 58%,
    rgba(255, 255, 255, 0.88) 100%
  );
  border: 1px solid rgba(var(--app-shell-border), 0.18);
  box-shadow: 0 20px 40px -26px rgba(var(--app-shell-shadow), 0.38);
}

.access-denied-kicker {
  margin: 0;
  text-transform: uppercase;
  letter-spacing: 0.14em;
  font-size: 0.82rem;
  font-weight: 700;
  color: rgb(var(--app-shell-text-muted));
}

.access-denied-title {
  margin: 0;
  font-size: clamp(2rem, 4vw, 2.5rem);
  font-weight: 700;
  line-height: 1.1;
  color: rgb(var(--app-shell-text-primary));
}

.access-denied-description {
  margin: 0 auto;
  max-width: 32rem;
  line-height: 1.6;
  font-size: 1rem;
  color: rgb(var(--app-shell-text-secondary));
}

.access-denied-scan {
  display: grid;
  width: min(100%, 320px);
  gap: 10px;
  margin-top: 12px;
  padding: 14px;
  border: 1px solid rgba(var(--v-theme-warning), 0.26);
  border-radius: 8px;
  background: rgba(var(--v-theme-warning), 0.08);
}

.access-denied-scan__bars {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;
  height: 26px;
  align-items: end;
}

.access-denied-scan__bars span {
  display: block;
  border-radius: 4px 4px 0 0;
  background: rgba(var(--v-theme-warning), 0.72);
  animation: access-denied-scan-bar 1.4s ease-in-out infinite;
}

.access-denied-scan__bars span:nth-child(1) {
  height: 12px;
}

.access-denied-scan__bars span:nth-child(2) {
  height: 22px;
  animation-delay: 0.12s;
}

.access-denied-scan__bars span:nth-child(3) {
  height: 16px;
  animation-delay: 0.24s;
}

.access-denied-scan__bars span:nth-child(4) {
  height: 24px;
  animation-delay: 0.36s;
}

.access-denied-scan__status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.86rem;
  font-weight: 700;
}

.access-denied-actions {
  display: grid;
  gap: 12px;
}

@keyframes access-denied-scan-bar {
  50% {
    opacity: 0.42;
    transform: scaleY(0.68);
  }
}

@media (prefers-reduced-motion: reduce) {
  .access-denied-scan__bars span {
    animation: none;
  }
}
</style>
