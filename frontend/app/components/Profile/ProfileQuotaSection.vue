<script setup lang="ts">
import type { CurrentQuotaStatus } from '~/types/subscriptions'
import { formatDateTime, formatNumber } from '~/utils/formatters'

const props = defineProps<{
  error: string | null
  loading: boolean
  quota: CurrentQuotaStatus | null
}>()

const emit = defineEmits<{
  retry: []
}>()

const windows = computed(() => {
  const quota = props.quota
  return [
    {
      key: '5h',
      label: '5h window',
      used: quota?.used5hTokens ?? 0,
      limit: quota?.quota5hTokens ?? null,
      remaining: quota?.remaining5hTokens ?? null,
      resetAt: quota?.reset5hAt ?? null,
    },
    {
      key: '7d',
      label: '7d window',
      used: quota?.used7dTokens ?? 0,
      limit: quota?.quota7dTokens ?? null,
      remaining: quota?.remaining7dTokens ?? null,
      resetAt: quota?.reset7dAt ?? null,
    },
  ]
})

function progressValue(used: number, limit: number | null) {
  if (!limit || limit <= 0) {
    return 0
  }
  return Math.min(100, Math.round((used / limit) * 100))
}

function quotaLabel(used: number, limit: number | null) {
  if (limit == null) {
    return `${formatNumber(used)} used`
  }
  return `${formatNumber(used)} / ${formatNumber(limit)}`
}

function remainingLabel(remaining: number | null) {
  return remaining == null ? 'Unlimited' : `${formatNumber(remaining)} left`
}

function resetLabel(resetAt: string | null) {
  return resetAt ? `Resets ${formatDateTime(resetAt)}` : 'No active window'
}
</script>

<template>
  <section class="profile-quota-section">
    <ProfileInfoCard
      icon="mdi-card-account-details-star-outline"
      title="Subscription quota"
      :subtitle="props.quota?.plan?.name ?? 'No subscription plan'"
    >
      <div class="profile-quota-section__body">
        <div
          v-if="props.loading"
          class="profile-quota-section__skeleton"
          aria-label="Loading subscription quota"
        >
          <span />
          <span />
        </div>

        <v-alert
          v-else-if="props.error"
          type="warning"
          variant="tonal"
          rounded="lg"
        >
          <div class="profile-quota-section__alert">
            <span>{{ props.error }}</span>
            <v-btn
              size="small"
              variant="tonal"
              prepend-icon="mdi-refresh"
              @click="emit('retry')"
            >
              Retry
            </v-btn>
          </div>
        </v-alert>

        <v-alert
          v-else-if="props.quota && !props.quota.hasSubscription"
          type="error"
          variant="tonal"
          rounded="lg"
        >
          Subscription required
        </v-alert>

        <div v-else class="profile-quota-section__windows">
          <div
            v-for="window in windows"
            :key="window.key"
            class="profile-quota-section__window"
          >
            <div class="profile-quota-section__window-header">
              <strong>{{ window.label }}</strong>
              <span>{{ quotaLabel(window.used, window.limit) }}</span>
            </div>
            <v-progress-linear
              rounded
              height="8"
              color="primary"
              bg-color="surface-variant"
              :model-value="progressValue(window.used, window.limit)"
            />
            <div class="profile-quota-section__window-footer">
              <span>{{ remainingLabel(window.remaining) }}</span>
              <span>{{ resetLabel(window.resetAt) }}</span>
            </div>
          </div>
        </div>
      </div>
    </ProfileInfoCard>
  </section>
</template>

<style scoped>
.profile-quota-section__body {
  padding: 0 24px 24px;
}

.profile-quota-section__skeleton,
.profile-quota-section__windows {
  display: grid;
  gap: 14px;
}

.profile-quota-section__skeleton span {
  min-height: 76px;
  border-radius: var(--app-card-radius);
  background: linear-gradient(
    90deg,
    rgba(var(--app-shell-border), 0.18),
    rgba(var(--app-shell-border), 0.34),
    rgba(var(--app-shell-border), 0.18)
  );
}

.profile-quota-section__window {
  display: grid;
  gap: 10px;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.42);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.58);
}

.profile-quota-section__window-header,
.profile-quota-section__window-footer,
.profile-quota-section__alert {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.profile-quota-section__window-header span,
.profile-quota-section__window-footer {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.86rem;
}

@media (max-width: 720px) {
  .profile-quota-section__window-header,
  .profile-quota-section__window-footer,
  .profile-quota-section__alert {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
