<script setup lang="ts">
import type { AdminUser } from '~/types/users'

type FocusableInput = {
  focus: () => void
}

const props = defineProps<{
  loading: boolean
  user: AdminUser | null
}>()

const emit = defineEmits<{
  cancel: []
  confirm: []
}>()

const isOpen = defineModel<boolean>({ default: false })
const confirmationValue = shallowRef('')
const hasInteracted = shallowRef(false)
const confirmationField = useTemplateRef<FocusableInput>('confirmationField')

const expectedUsername = computed(() => props.user?.preferredUsername ?? '')
const displayName = computed(
  () =>
    props.user?.name ||
    props.user?.preferredUsername ||
    props.user?.email ||
    'User',
)
const canConfirm = computed(
  () =>
    Boolean(expectedUsername.value) &&
    confirmationValue.value.trim() === expectedUsername.value,
)
const confirmationError = computed(() => {
  if (!hasInteracted.value || !confirmationValue.value || canConfirm.value) {
    return ''
  }

  return 'Type the username exactly as shown to enable deletion.'
})

watch(isOpen, async (open) => {
  confirmationValue.value = ''
  hasInteracted.value = false

  if (!open) {
    return
  }

  await nextTick()
  confirmationField.value?.focus()
})

// cancel emits the delete dialog cancel action.
function cancel() {
  isOpen.value = false
  emit('cancel')
}

// confirm emits the delete dialog confirmation action.
function confirm() {
  hasInteracted.value = true

  if (!canConfirm.value) {
    return
  }

  emit('confirm')
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="560" :persistent="props.loading">
    <v-card rounded="xl" class="admin-users-delete-dialog">
      <v-card-item class="px-6 pt-6 pb-2">
        <template #prepend>
          <v-avatar color="error" variant="tonal" size="44">
            <v-icon icon="mdi-delete-alert-outline" />
          </v-avatar>
        </template>

        <v-card-title class="text-h6">Delete user</v-card-title>
        <v-card-subtitle>
          Confirm the local account removal for {{ displayName }}.
        </v-card-subtitle>
      </v-card-item>

      <v-card-text
        v-if="props.user"
        class="admin-users-delete-dialog__body px-6"
      >
        <v-sheet rounded="lg" border class="admin-users-delete-dialog__identity">
          <v-list bg-color="transparent" density="comfortable" lines="two">
            <v-list-item
              prepend-icon="mdi-account-outline"
              title="Name"
              :subtitle="displayName"
            />
            <v-list-item
              prepend-icon="mdi-email-outline"
              title="Email"
              :subtitle="props.user.email"
            />
            <v-list-item
              prepend-icon="mdi-at"
              title="Username"
              :subtitle="props.user.preferredUsername"
            >
              <template #append>
                <v-chip color="error" variant="tonal" label size="small">
                  Required
                </v-chip>
              </template>
            </v-list-item>
          </v-list>
        </v-sheet>

        <v-text-field
          ref="confirmationField"
          v-model="confirmationValue"
          label="Type the username to confirm"
          :placeholder="expectedUsername"
          :error="Boolean(confirmationError)"
          :error-messages="confirmationError ? [confirmationError] : []"
          hide-details="auto"
          variant="outlined"
          density="comfortable"
          autocomplete="off"
          autocapitalize="off"
          spellcheck="false"
          @update:model-value="hasInteracted = true"
          @keydown.enter.prevent="confirm"
        />
      </v-card-text>

      <v-card-actions class="px-6 pb-6 pt-2">
        <v-spacer />
        <AppDialogCloseButton
          :disabled="props.loading"
          label="Cancel"
          @click="cancel"
        />
        <AppDialogActionButton
          color="error"
          label="Delete account"
          :disabled="!canConfirm"
          :loading="props.loading"
          @click="confirm"
        />
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.admin-users-delete-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  background: linear-gradient(
    180deg,
    rgb(var(--app-shell-surface)) 0%,
    rgb(var(--app-shell-surface-muted)) 100%
  );
}

.admin-users-delete-dialog__body {
  display: grid;
  gap: 20px;
}

.admin-users-delete-dialog__identity {
  overflow: hidden;
  background: rgba(var(--app-shell-surface-muted), 0.42);
}
</style>
