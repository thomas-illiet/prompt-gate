<script setup lang="ts">
import quotaExhaustedImage from '~/assets/images/quota-exhausted.jpg'
import type { ExhaustedQuotaWindow } from '~/composables/useQuotaExhaustionDialog'
import { formatDateTime, formatNumber } from '~/utils/formatters'

const props = defineProps<{
  canReset: boolean
  resetAvailable: number
  resetError: string | null
  resetLoading: boolean
  resetMaximum: number
  resetPending: boolean
  resetUsing: boolean
  windows: ExhaustedQuotaWindow[]
}>()

const emit = defineEmits<{
  reset: []
}>()

const isOpen = defineModel<boolean>({ default: false })
const confirmingReset = shallowRef(false)
const remainingAfterReset = computed(() =>
  Math.max(0, props.resetAvailable - 1),
)

function close() {
  if (props.resetUsing) {
    return
  }

  confirmingReset.value = false
  isOpen.value = false
}

function startResetConfirmation() {
  if (props.canReset && !props.resetUsing) {
    confirmingReset.value = true
  }
}

function confirmReset() {
  if (props.canReset && !props.resetUsing) {
    emit('reset')
  }
}

watch(isOpen, (open) => {
  if (!open) {
    confirmingReset.value = false
  }
})

watch(
  () => props.resetPending,
  (pending) => {
    if (pending) {
      confirmingReset.value = false
    }
  },
)
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    close-label="Close quota notification"
    :loading="props.resetUsing"
    :max-width="620"
    subtitle="Request access is paused until quota becomes available again."
    title="Quota reached"
  >
    <div class="quota-exhausted-dialog" data-test="quota-exhausted-dialog">
      <div class="quota-exhausted-dialog__visual" aria-hidden="true">
        <img :src="quotaExhaustedImage" alt="" />
      </div>

      <template v-if="confirmingReset">
        <div
          class="quota-exhausted-dialog__confirmation"
          aria-live="polite"
          role="status"
        >
          <strong>Use one quota reset?</strong>
          <p>
            This clears both live quota windows without deleting usage
            analytics. {{ remainingAfterReset }}
            {{ remainingAfterReset === 1 ? 'reset' : 'resets' }} will remain.
          </p>
        </div>
      </template>

      <template v-else>
        <p class="quota-exhausted-dialog__intro">
          You've used all available tokens in the following rolling
          {{ props.windows.length === 1 ? 'window' : 'windows' }}.
        </p>

        <div class="quota-exhausted-dialog__windows">
          <article
            v-for="window in props.windows"
            :key="window.key"
            class="quota-exhausted-dialog__window"
            :data-test="'quota-window-' + window.key"
          >
            <div class="quota-exhausted-dialog__window-heading">
              <strong>{{ window.label }}</strong>
              <v-chip color="error" label size="small" variant="tonal">
                Exhausted
              </v-chip>
            </div>
            <span>
              {{ formatNumber(window.used) }} /
              {{ formatNumber(window.limit) }} tokens used
            </span>
            <span class="quota-exhausted-dialog__reset-time">
              {{
                window.resetAt
                  ? 'Usage starts freeing up ' + formatDateTime(window.resetAt)
                  : 'No automatic reset time is available'
              }}
            </span>
          </article>
        </div>

        <v-alert
          v-if="props.resetPending"
          data-test="quota-reset-pending"
          rounded="lg"
          type="info"
          variant="tonal"
        >
          Your reset is pending and will be applied automatically.
        </v-alert>
        <div
          v-else-if="props.resetLoading"
          class="quota-exhausted-dialog__reset-status"
          data-test="quota-reset-loading"
        >
          <v-progress-circular color="primary" indeterminate size="20" />
          <span>Checking reset availability…</span>
        </div>
        <v-alert
          v-else-if="props.resetError"
          data-test="quota-reset-unavailable"
          rounded="lg"
          type="warning"
          variant="tonal"
        >
          Reset availability could not be checked. You can close this message
          and try again when you reopen the site.
        </v-alert>
        <p
          v-else-if="!props.canReset"
          class="quota-exhausted-dialog__reset-status"
          data-test="quota-reset-empty"
        >
          No quota reset is available right now.
        </p>
        <p
          v-else
          class="quota-exhausted-dialog__reset-status"
          data-test="quota-reset-available"
        >
          {{ props.resetAvailable }} of {{ props.resetMaximum }} quota resets
          available.
        </p>
      </template>
    </div>

    <template #actions>
      <template v-if="confirmingReset">
        <AppDialogCloseButton
          :disabled="props.resetUsing"
          label="Back"
          @click="confirmingReset = false"
        />
        <AppDialogActionButton
          label="Confirm reset"
          prepend-icon="mdi-refresh"
          :loading="props.resetUsing"
          @click="confirmReset"
        />
      </template>
      <template v-else>
        <AppDialogCloseButton
          :disabled="props.resetUsing"
          label="Close"
          @click="close"
        />
        <AppDialogActionButton
          v-if="props.canReset"
          label="Use a reset"
          prepend-icon="mdi-refresh"
          :loading="props.resetUsing"
          @click="startResetConfirmation"
        />
      </template>
    </template>
  </AppDialogCard>
</template>

<style scoped>
.quota-exhausted-dialog {
  display: grid;
  gap: 18px;
}

.quota-exhausted-dialog__visual {
  overflow: hidden;
  border-radius: var(--app-card-radius);
  aspect-ratio: 16 / 9;
  background: rgb(var(--app-shell-surface-muted));
}

.quota-exhausted-dialog__visual img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center 42%;
}

.quota-exhausted-dialog__intro,
.quota-exhausted-dialog__confirmation p,
.quota-exhausted-dialog__reset-status {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.55;
}

.quota-exhausted-dialog__windows {
  display: grid;
  gap: 10px;
}

.quota-exhausted-dialog__window {
  display: grid;
  gap: 7px;
  padding: 14px;
  border: 1px solid rgba(var(--v-theme-error), 0.34);
  border-radius: var(--app-card-radius);
  background: rgba(var(--v-theme-error), 0.06);
}

.quota-exhausted-dialog__window-heading,
.quota-exhausted-dialog__reset-status {
  display: flex;
  align-items: center;
  gap: 10px;
}

.quota-exhausted-dialog__window-heading {
  justify-content: space-between;
}

.quota-exhausted-dialog__window > span {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.9rem;
}

.quota-exhausted-dialog__reset-time {
  color: rgb(var(--app-shell-text-muted)) !important;
}

.quota-exhausted-dialog__confirmation {
  display: grid;
  gap: 8px;
  padding: 16px;
  border: 1px solid rgba(var(--v-theme-warning), 0.35);
  border-radius: var(--app-card-radius);
  background: rgba(var(--v-theme-warning), 0.08);
}

@media (max-width: 600px) {
  .quota-exhausted-dialog__visual {
    aspect-ratio: 4 / 3;
  }

  .quota-exhausted-dialog__window-heading {
    align-items: flex-start;
  }
}
</style>
