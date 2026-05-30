<script setup lang="ts">
import type { AppRole } from '~/types/auth'
import type { AdminUser, UpdateUserPayload } from '~/types/users'
import { APP_ROLES, appRoleColor, appRoleLabel } from '~/utils/auth'
import { formatDateTime } from '~/utils/formatters'

const props = defineProps<{
  loading: boolean
  user: AdminUser | null
}>()

const emit = defineEmits<{
  save: [payload: UpdateUserPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const selectedRole = shallowRef<AppRole>('none')
const isActive = shallowRef(true)
const expiresAt = shallowRef('')
const accessForm = shallowRef<HTMLFormElement | null>(null)

const roleOptions = computed(() =>
  APP_ROLES.map((role) => ({
    title: appRoleLabel(role),
    value: role,
  })),
)
const displayName = computed(
  () =>
    props.user?.name ||
    props.user?.preferredUsername ||
    props.user?.email ||
    'User',
)
const displayEmail = computed(() => props.user?.email || 'No email')
const displayUsername = computed(
  () => props.user?.preferredUsername || 'No username',
)
const displaySubject = computed(() => props.user?.sub || 'No OIDC subject')

watch(
  () => props.user,
  (user) => {
    selectedRole.value = user?.role ?? 'none'
    isActive.value = user?.isActive ?? true
    expiresAt.value = toDateTimeLocalValue(user?.expiresAt)
  },
  { immediate: true },
)

watch(selectedRole, (role) => {
  if (role === 'none') {
    expiresAt.value = ''
  }
})

const isExpirationDisabled = computed(() => selectedRole.value === 'none')

// save emits the edited role, status, and expiration payload.
function save() {
  const formExpiresAt = getFormExpiresAt()
  emit('save', {
    role: selectedRole.value,
    isActive: isActive.value,
    expiresAt: toISOStringOrNull(formExpiresAt),
  })
}

// getFormExpiresAt converts disabled or empty expiration inputs to null.
function getFormExpiresAt() {
  if (!accessForm.value) {
    return expiresAt.value
  }

  const value = new FormData(accessForm.value).get('expiresAt')
  return typeof value === 'string' ? value : ''
}

// toDateTimeLocalValue adapts an ISO timestamp for datetime-local inputs.
function toDateTimeLocalValue(value: string | null | undefined) {
  if (!value) {
    return ''
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }

  const offsetMs = date.getTimezoneOffset() * 60 * 1000
  return new Date(date.getTime() - offsetMs).toISOString().slice(0, 16)
}

// toISOStringOrNull converts a datetime-local value back to ISO format.
function toISOStringOrNull(value: string) {
  if (!value) {
    return null
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return null
  }

  return date.toISOString()
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="620" :persistent="props.loading">
    <v-card rounded="xl" class="admin-users-dialog">
      <v-card-item class="px-6 pt-6 pb-2">
        <template #prepend>
          <v-avatar color="primary" variant="tonal" size="44">
            <v-icon icon="mdi-account-cog-outline" />
          </v-avatar>
        </template>

        <v-card-title class="text-h6">Update user access</v-card-title>
        <v-card-subtitle>
          Review identity and adjust application authorization.
        </v-card-subtitle>
      </v-card-item>

      <form
        ref="accessForm"
        class="admin-users-dialog__form"
        @submit.prevent="save"
      >
        <v-card-text
          v-if="props.user"
          class="admin-users-dialog__body px-6 pb-2"
        >
          <v-sheet rounded="lg" border class="admin-users-dialog__identity">
            <v-list bg-color="transparent" density="comfortable" lines="two">
              <v-list-item
                prepend-icon="mdi-account-outline"
                title="Name"
                :subtitle="displayName"
              />
              <v-list-item
                prepend-icon="mdi-email-outline"
                title="Email"
                :subtitle="displayEmail"
              />
              <v-list-item
                prepend-icon="mdi-at"
                title="Username"
                :subtitle="displayUsername"
              >
                <template #append>
                  <v-chip
                    size="small"
                    label
                    variant="tonal"
                    :color="appRoleColor(props.user.role)"
                  >
                    {{ appRoleLabel(props.user.role) }}
                  </v-chip>
                </template>
              </v-list-item>
              <v-list-item
                prepend-icon="mdi-fingerprint"
                title="OIDC subject"
                :subtitle="displaySubject"
              />
            </v-list>
          </v-sheet>

          <v-row>
            <v-col cols="12" md="7">
              <v-select
                v-model="selectedRole"
                :items="roleOptions"
                label="Application role"
                variant="outlined"
                density="comfortable"
              />
            </v-col>

            <v-col cols="12" md="5">
              <v-switch
                v-model="isActive"
                color="success"
                inset
                label="Active account"
              />
            </v-col>

            <v-col cols="12">
              <label class="admin-users-dialog__field">
                <span class="admin-users-dialog__label">Expires at</span>
                <input
                  v-model="expiresAt"
                  :disabled="isExpirationDisabled"
                  class="admin-users-dialog__datetime"
                  name="expiresAt"
                  type="datetime-local"
                />
              </label>
            </v-col>
          </v-row>

          <div class="admin-users-dialog__meta">
            <span
              >Last login: {{ formatDateTime(props.user.lastLoginAt) }}</span
            >
            <span>Created: {{ formatDateTime(props.user.createdAt) }}</span>
          </div>
        </v-card-text>

        <v-card-actions class="px-6 pb-6">
          <v-spacer />
          <AppDialogCloseButton label="Cancel" @click="isOpen = false" />
          <AppDialogActionButton
            color="primary"
            label="Save access"
            type="submit"
            :loading="props.loading"
          />
        </v-card-actions>
      </form>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.admin-users-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  background: linear-gradient(
    180deg,
    rgb(var(--app-shell-surface)) 0%,
    rgb(var(--app-shell-surface-muted)) 100%
  );
}

.admin-users-dialog__form {
  display: contents;
}

.admin-users-dialog__body {
  display: grid;
  gap: 20px;
}

.admin-users-dialog__identity {
  overflow: hidden;
  background: rgba(var(--app-shell-surface-muted), 0.42);
}

.admin-users-dialog__meta {
  display: grid;
  gap: 6px;
  color: rgb(var(--app-shell-text-muted));
}

.admin-users-dialog__field {
  display: grid;
  gap: 6px;
}

.admin-users-dialog__label {
  font-size: 0.75rem;
  color: rgba(var(--v-theme-on-surface), 0.72);
}

.admin-users-dialog__datetime {
  width: 100%;
  min-height: 48px;
  padding: 0 16px;
  color: rgb(var(--v-theme-on-surface));
  background: transparent;
  border: 1px solid rgba(var(--v-theme-on-surface), 0.38);
  border-radius: 4px;
  font: inherit;
}

.admin-users-dialog__datetime:focus {
  border-color: rgb(var(--v-theme-primary));
  outline: 2px solid rgba(var(--v-theme-primary), 0.16);
}

.admin-users-dialog__datetime:disabled {
  opacity: 0.5;
}

@media (max-width: 720px) {
  .admin-users-dialog__body {
    gap: 16px;
  }
}
</style>
