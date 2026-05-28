<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    cancelLabel?: string
    confirmColor?: string
    confirmLabel?: string
    icon?: string
    loading?: boolean
    maxWidth?: number | string
    message?: string
    persistent?: boolean
    title: string
  }>(),
  {
    cancelLabel: 'Cancel',
    confirmColor: 'primary',
    confirmLabel: 'Confirm',
    icon: 'mdi-alert-outline',
    loading: false,
    maxWidth: 420,
    message: '',
    persistent: false,
  },
)

const emit = defineEmits<{
  cancel: []
  confirm: []
}>()

const isOpen = defineModel<boolean>({ default: false })

// close hides the dialog without emitting an action.
function close() {
  if (props.loading) {
    return
  }

  isOpen.value = false
}

// cancel notifies the parent that the dialog was dismissed.
function cancel() {
  close()
  emit('cancel')
}

// confirm notifies the parent that the primary action was accepted.
function confirm() {
  emit('confirm')
}
</script>

<template>
  <v-dialog
    v-model="isOpen"
    :max-width="props.maxWidth"
    :persistent="props.persistent || props.loading"
  >
    <v-card rounded="lg" class="app-confirm-dialog app-surface-gradient">
      <v-card-title class="app-confirm-dialog__header">
        <v-icon
          :icon="props.icon"
          :color="props.confirmColor"
          class="app-confirm-dialog__icon"
        />
        <span>{{ props.title }}</span>
      </v-card-title>

      <v-card-text class="app-confirm-dialog__body">
        <slot>
          {{ props.message }}
        </slot>
      </v-card-text>

      <v-card-actions class="app-confirm-dialog__actions">
        <v-spacer />
        <AppDialogCloseButton
          :disabled="props.loading"
          :label="props.cancelLabel"
          @click="cancel"
        />
        <AppDialogActionButton
          :color="props.confirmColor"
          :label="props.confirmLabel"
          :loading="props.loading"
          @click="confirm"
        />
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.app-confirm-dialog__header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 24px 24px 8px;
  white-space: normal;
}

.app-confirm-dialog__icon {
  flex: 0 0 auto;
}

.app-confirm-dialog__body {
  padding: 0 24px 8px;
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.6;
}

.app-confirm-dialog__actions {
  padding: 8px 24px 24px;
}
</style>
