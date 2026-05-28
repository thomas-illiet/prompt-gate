<script setup lang="ts">
import { Notify } from '~/stores/notification'
import { copyTextToClipboard } from '~/utils/clipboard'
import { formatDateTime } from '~/utils/formatters'

interface CreatedTokenInfo {
  expiresAt: string
  name: string
}

interface CreatedTokenValue {
  token: string
  tokenInfo: CreatedTokenInfo
}

const props = defineProps<{
  createdToken: CreatedTokenValue | null
}>()

const isOpen = defineModel<boolean>({ default: false })
const rawToken = computed(() => props.createdToken?.token ?? '')

// copyToken writes the one-time token secret to the clipboard.
async function copyToken() {
  if (!rawToken.value || !import.meta.client) {
    return
  }

  const copied = await copyTextToClipboard(rawToken.value)
  if (copied) {
    Notify.success('Virtual key copied.')
    return
  }

  Notify.error('Unable to copy virtual key.')
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-key-plus"
    icon-color="success"
    max-width="760"
    title="Virtual key generated"
    subtitle="Copy this value now; it will not be shown again."
  >
    <div class="app-created-token-dialog__body">
      <div class="app-created-token-dialog__token">
        {{ rawToken }}
      </div>

      <div v-if="props.createdToken" class="app-created-token-dialog__meta">
        <span>Name: {{ props.createdToken.tokenInfo.name }}</span>
        <span>
          Expires:
          {{ formatDateTime(props.createdToken.tokenInfo.expiresAt) }}
        </span>
      </div>
    </div>

    <template #actions>
      <v-spacer />
      <AppDialogCloseButton @click="isOpen = false" />
      <AppDialogActionButton
        color="primary"
        label="Copy key"
        prepend-icon="mdi-content-copy"
        @click="copyToken"
      />
    </template>
  </AppDialogCard>
</template>

<style scoped>
.app-created-token-dialog__body {
  display: grid;
  gap: 16px;
}

.app-created-token-dialog__token {
  max-height: 180px;
  padding: 16px;
  overflow: auto;
  color: rgb(var(--app-shell-text-primary));
  background: rgba(var(--app-shell-surface-muted), 0.74);
  border: 1px solid rgba(var(--app-shell-border), 0.5);
  border-radius: 8px;
  font-family: monospace;
  font-size: 0.8125rem;
  line-height: 1.6;
  overflow-wrap: anywhere;
}

.app-created-token-dialog__meta {
  display: grid;
  gap: 6px;
  color: rgb(var(--app-shell-text-secondary));
}
</style>
