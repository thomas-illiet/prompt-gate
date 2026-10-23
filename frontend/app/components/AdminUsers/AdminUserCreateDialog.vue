<script setup lang="ts">
import type { CreateUserPayload } from '~/types/users'

const props = defineProps<{
  loading: boolean
}>()

const emit = defineEmits<{
  create: [payload: CreateUserPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const preferredUsername = shallowRef('')
const hasSubmitted = shallowRef(false)
const formId = useId()

const normalizedUID = computed(() => preferredUsername.value.trim())
const uidError = computed(() => {
  if (!hasSubmitted.value || normalizedUID.value) {
    return ''
  }
  return 'User UID is required.'
})

watch(isOpen, (open) => {
  if (!open) {
    return
  }
  preferredUsername.value = ''
  hasSubmitted.value = false
})

// create validates and emits the exact, whitespace-trimmed SSO UID.
function create() {
  if (props.loading) {
    return
  }

  hasSubmitted.value = true
  if (!normalizedUID.value) {
    return
  }

  emit('create', { preferredUsername: normalizedUID.value })
}
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-account-plus-outline"
    :loading="props.loading"
    max-width="560"
    subtitle="Create an active limited account now. SSO will complete its profile on first sign-in."
    title="Pre-create user"
  >
    <form :id="formId" @submit.prevent="create">
      <v-text-field
        v-model="preferredUsername"
        autocomplete="off"
        :disabled="props.loading"
        :error="Boolean(uidError)"
        :error-messages="uidError ? [uidError] : []"
        hint="Must exactly match the case-sensitive OIDC preferred_username claim."
        label="User UID"
        persistent-hint
        placeholder="alice"
        prepend-inner-icon="mdi-at"
        required
        variant="outlined"
      />
    </form>

    <template #actions>
      <AppDialogCloseButton
        :disabled="props.loading"
        label="Cancel"
        @click="isOpen = false"
      />
      <AppDialogActionButton
        color="primary"
        :form="formId"
        label="Pre-create user"
        :loading="props.loading"
        prepend-icon="mdi-account-plus-outline"
        type="submit"
      />
    </template>
  </AppDialogCard>
</template>
