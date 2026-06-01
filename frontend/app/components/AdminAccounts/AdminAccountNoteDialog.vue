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
  <v-dialog v-model="isOpen" max-width="640" :persistent="props.loading">
    <v-card rounded="xl" class="admin-account-note-dialog">
      <v-card-item class="px-6 pt-6 pb-2">
        <template #prepend>
          <v-avatar color="primary" variant="tonal" size="44">
            <v-icon icon="mdi-note-edit-outline" />
          </v-avatar>
        </template>

        <v-card-title class="text-h6">Account notes</v-card-title>
        <v-card-subtitle>{{ displayName }}</v-card-subtitle>
      </v-card-item>

      <form class="admin-account-note-dialog__form" @submit.prevent="save">
        <v-card-text
          v-if="props.account"
          class="admin-account-note-dialog__body px-6 pb-2"
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
        </v-card-text>

        <v-card-actions class="px-6 pb-6">
          <v-spacer />
          <AppDialogCloseButton label="Cancel" @click="isOpen = false" />
          <AppDialogActionButton
            color="primary"
            label="Save note"
            type="submit"
            :disabled="!props.account || isNoteTooLong"
            :loading="props.loading"
          />
        </v-card-actions>
      </form>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.admin-account-note-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  background: linear-gradient(
    180deg,
    rgb(var(--app-shell-surface)) 0%,
    rgb(var(--app-shell-surface-muted)) 100%
  );
}

.admin-account-note-dialog__form {
  display: contents;
}

.admin-account-note-dialog__body {
  display: grid;
  gap: 20px;
}

.admin-account-note-dialog__identity {
  overflow: hidden;
  background: rgba(var(--app-shell-surface-muted), 0.42);
}
</style>
