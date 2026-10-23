<script setup lang="ts">
import type { QuotaResetSummary } from '~/types/quota-resets'

const props = defineProps<{
  error: string | null
  loading: boolean
  summary: QuotaResetSummary | null
  using: boolean
}>()

const emit = defineEmits<{
  retry: []
  showHistory: []
  use: []
}>()

const available = computed(() => props.summary?.available ?? 0)
const maximum = computed(() => props.summary?.maximum ?? 2)
const canUse = computed(
  () =>
    Boolean(props.summary) &&
    available.value > 0 &&
    Boolean(props.summary?.hasResettableQuota) &&
    !props.summary?.pendingReset,
)

const guidance = computed(() => {
  if (props.summary?.pendingReset) {
    return 'Your reset is pending and will be applied automatically.'
  }
  if (!props.summary?.hasResettableQuota) {
    return 'Your current plan has no finite quota that needs resetting.'
  }
  if (available.value === 0) {
    return 'No reset is available right now. Monthly credits are automatic.'
  }
  return 'Using one reset clears both live quota windows without changing usage analytics.'
})

const useLabel = computed(() =>
  props.summary?.pendingReset ? 'Reset pending' : 'Use a quota reset',
)
</script>

<template>
  <section class="profile-quota-reset-card" data-test="profile-quota-resets">
    <ProfileInfoCard
      icon="mdi-refresh"
      title="Quota resets"
      subtitle="Emergency credits for finite subscription quotas"
    >
      <template #actions>
        <v-chip
          v-if="props.summary"
          color="primary"
          label
          size="small"
          variant="tonal"
          data-test="quota-reset-balance-chip"
        >
          {{ available }}/{{ maximum }} available
        </v-chip>
      </template>

      <div
        v-if="props.loading"
        class="profile-quota-reset-card__loading"
        aria-label="Loading quota resets"
      >
        <v-progress-circular color="primary" indeterminate size="34" />
      </div>

      <v-alert
        v-else-if="props.error"
        class="profile-quota-reset-card__error"
        rounded="lg"
        type="warning"
        variant="tonal"
      >
        <div class="profile-quota-reset-card__alert-content">
          <span>{{ props.error }}</span>
          <v-btn
            prepend-icon="mdi-refresh"
            size="small"
            variant="text"
            @click="emit('retry')"
          >
            Retry
          </v-btn>
        </div>
      </v-alert>

      <div v-else-if="props.summary" class="profile-quota-reset-card__body">
        <div class="profile-quota-reset-card__balance">
          <v-avatar color="primary" size="54" variant="tonal">
            <strong data-test="quota-reset-available">{{ available }}</strong>
          </v-avatar>
          <div>
            <strong>Resets available</strong>
            <span>{{ guidance }}</span>
          </div>
        </div>

        <v-alert
          v-if="props.summary?.pendingReset"
          data-test="quota-reset-pending"
          rounded="lg"
          type="info"
          variant="tonal"
        >
          A reset is pending. This page will update automatically.
        </v-alert>

        <div class="profile-quota-reset-card__actions">
          <v-btn
            color="primary"
            prepend-icon="mdi-refresh"
            variant="flat"
            :disabled="!canUse"
            :loading="props.using"
            data-test="use-quota-reset"
            @click="emit('use')"
          >
            {{ useLabel }}
          </v-btn>
          <v-btn
            prepend-icon="mdi-history"
            variant="tonal"
            data-test="show-quota-reset-history"
            @click="emit('showHistory')"
          >
            View history
          </v-btn>
        </div>
      </div>

      <v-alert
        v-else
        class="profile-quota-reset-card__error"
        data-test="quota-reset-unavailable"
        rounded="lg"
        type="info"
        variant="tonal"
      >
        <div class="profile-quota-reset-card__alert-content">
          <span>Quota reset information is not available yet.</span>
          <v-btn
            prepend-icon="mdi-refresh"
            size="small"
            variant="text"
            @click="emit('retry')"
          >
            Refresh
          </v-btn>
        </div>
      </v-alert>
    </ProfileInfoCard>
  </section>
</template>

<style scoped>
.profile-quota-reset-card__loading {
  display: grid;
  min-height: 150px;
  place-items: center;
}

.profile-quota-reset-card__error {
  margin: 0 24px 24px;
}

.profile-quota-reset-card__alert-content,
.profile-quota-reset-card__actions,
.profile-quota-reset-card__balance {
  display: flex;
  align-items: center;
  gap: 12px;
}

.profile-quota-reset-card__alert-content {
  justify-content: space-between;
}

.profile-quota-reset-card__body {
  display: grid;
  gap: 18px;
  padding: 0 24px 24px;
}

.profile-quota-reset-card__balance > div {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.profile-quota-reset-card__balance span {
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.5;
}

.profile-quota-reset-card__actions {
  justify-content: flex-end;
}

@media (max-width: 600px) {
  .profile-quota-reset-card__body {
    padding: 0 16px 20px;
  }

  .profile-quota-reset-card__alert-content,
  .profile-quota-reset-card__actions {
    align-items: stretch;
    flex-direction: column;
  }

  .profile-quota-reset-card__actions :deep(.v-btn) {
    width: 100%;
  }
}
</style>
