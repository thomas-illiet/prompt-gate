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
  <AppDialogCard
    v-model="isOpen"
    :closable="false"
    :icon="props.icon"
    :icon-color="props.confirmColor"
    :loading="props.loading"
    :max-width="props.maxWidth"
    :persistent="props.persistent || props.loading"
    :title="props.title"
  >
    <div class="app-confirm-dialog__body">
        <slot>
          {{ props.message }}
        </slot>
    </div>

    <template #actions>
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
    </template>
  </AppDialogCard>
</template>

<style scoped>
.app-confirm-dialog__body {
  color: rgb(var(--app-shell-text-secondary));
  line-height: 1.6;
}
</style>
