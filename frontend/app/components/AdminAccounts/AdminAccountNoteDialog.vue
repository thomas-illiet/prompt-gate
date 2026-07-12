<script setup lang="ts">
interface AccountNoteTarget {
  email?: string
  identifier?: string
  name: string
  note: string
  preferredUsername?: string
}

const maxNoteLength = 2000

const props = defineProps<{
  account: AccountNoteTarget | null
  loading: boolean
}>()

const emit = defineEmits<{
  save: [note: string]
}>()

const isOpen = defineModel<boolean>({ default: false })
const draftNote = shallowRef('')

const displayName = computed(() => props.account?.name || 'Account')
const displayIdentifier = computed(() => {
  if (!props.account) {
    return 'No identifier'
  }

  return (
    props.account.identifier ||
    props.account.preferredUsername ||
    props.account.email ||
    'No identifier'
  )
})
const noteLength = computed(() => Array.from(draftNote.value).length)
const isNoteTooLong = computed(() => noteLength.value > maxNoteLength)
const formId = useId()
const noteErrors = computed(() =>
  isNoteTooLong.value ? ['Notes must be 2,000 characters or fewer.'] : [],
)

watch(
  () => props.account,
  (account) => {
    draftNote.value = account?.note ?? ''
  },
  { immediate: true },
)

// save emits the current note draft when it passes client-side validation.
function save() {
  if (!props.account || isNoteTooLong.value) {
    return
  }

  emit('save', draftNote.value)
}
</script>

<template>
  <AppDialogCard v-model="isOpen" icon="mdi-note-edit-outline" :loading="props.loading" max-width="640" :subtitle="`Keep internal context about ${displayName}.`" title="Account notes">
      <form :id="formId" @submit.prevent="save">
        <div
          v-if="props.account"
          class="admin-account-note-dialog__body"
        >
          <v-sheet
            rounded="lg"
            border
            class="admin-account-note-dialog__identity"
          >
            <v-list bg-color="transparent" density="comfortable" lines="two">
              <v-list-item
                prepend-icon="mdi-account-outline"
                title="Name"
                :subtitle="displayName"
              />
              <v-list-item
                prepend-icon="mdi-tag-outline"
                title="Identifier"
                :subtitle="displayIdentifier"
              />
            </v-list>
          </v-sheet>

          <v-textarea
            v-model="draftNote"
            auto-grow
            counter="2000"
            label="Note"
            rows="7"
            variant="outlined"
            :error-messages="noteErrors"
          />
        </div>
      </form>

      <template #actions>
          <AppDialogCloseButton :disabled="props.loading" label="Cancel" @click="isOpen = false" />
          <AppDialogActionButton
            color="primary"
            :form="formId"
            label="Save note"
            type="submit"
            :disabled="!props.account || isNoteTooLong"
            :loading="props.loading"
          />
      </template>
  </AppDialogCard>
</template>

<style scoped>
.admin-account-note-dialog__body {
  display: grid;
  gap: 20px;
}

.admin-account-note-dialog__identity {
  overflow: hidden;
  background: rgba(var(--app-shell-surface-muted), 0.42);
}
</style>
