<script setup lang="ts">
import type {
  CreateProviderPayload,
  Provider,
  ProviderType,
  UpdateProviderPayload,
} from '~/types/providers'

const DEFAULT_BASE_URLS: Record<ProviderType, string> = {
  openai: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com',
  ollama: 'http://localhost:11434/v1',
}

const props = defineProps<{
  loading: boolean
  provider: Provider | null
}>()

const emit = defineEmits<{
  save: [payload: CreateProviderPayload | UpdateProviderPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const name = shallowRef('')
const displayName = shallowRef('')
const type = shallowRef<ProviderType>('openai')
const baseUrl = shallowRef(DEFAULT_BASE_URLS.openai)
const apiKey = shallowRef('')
const enabled = shallowRef(true)
const clearApiKey = shallowRef(false)
const hasSubmitted = shallowRef(false)

const providerTypeOptions = [
  { title: 'OpenAI', value: 'openai' as const },
  { title: 'Anthropic', value: 'anthropic' as const },
  { title: 'Ollama', value: 'ollama' as const },
]

const title = computed(() =>
  props.provider ? 'Update provider' : 'Create provider',
)
const submitLabel = computed(() =>
  props.provider ? 'Save provider' : 'Create provider',
)
const isEditing = computed(() => Boolean(props.provider))
const normalizedName = computed(() => name.value.trim().toLowerCase())
const trimmedBaseUrl = computed(() => baseUrl.value.trim())
const nameError = computed(() => {
  if (isEditing.value) {
    return ''
  }

  if (!hasSubmitted.value) {
    return ''
  }

  if (!normalizedName.value) {
    return 'Name is required.'
  }

  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(normalizedName.value)) {
    return 'Use lowercase letters, numbers, and single hyphens.'
  }

  return ''
})
const baseUrlError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }

  if (!trimmedBaseUrl.value) {
    return 'Base URL is required.'
  }

  try {
    const parsed = new URL(trimmedBaseUrl.value)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return 'Use an HTTP or HTTPS URL.'
    }
  } catch {
    return 'Use a valid HTTP or HTTPS URL.'
  }

  return ''
})
const canSave = computed(
  () =>
    !nameError.value &&
    !baseUrlError.value &&
    (isEditing.value || Boolean(normalizedName.value)),
)

watch(
  [isOpen, () => props.provider],
  ([open]) => {
    if (!open) {
      return
    }

    name.value = props.provider?.name ?? ''
    displayName.value = props.provider?.displayName ?? ''
    type.value = props.provider?.type ?? 'openai'
    baseUrl.value = props.provider?.baseUrl ?? DEFAULT_BASE_URLS[type.value]
    apiKey.value = ''
    enabled.value = props.provider?.enabled ?? true
    clearApiKey.value = false
    hasSubmitted.value = false
  },
  { immediate: true },
)

watch(type, (nextType, previousType) => {
  if (props.provider) {
    return
  }

  if (!baseUrl.value || baseUrl.value === DEFAULT_BASE_URLS[previousType]) {
    baseUrl.value = DEFAULT_BASE_URLS[nextType]
  }
})

// save validates the form and emits the provider payload.
function save() {
  hasSubmitted.value = true

  if (!canSave.value) {
    return
  }

  const payload: UpdateProviderPayload = {
    displayName: displayName.value.trim(),
    type: type.value,
    baseUrl: trimmedBaseUrl.value,
    enabled: enabled.value,
  }

  const trimmedApiKey = apiKey.value.trim()
  if (clearApiKey.value) {
    payload.apiKey = ''
  } else if (trimmedApiKey) {
    payload.apiKey = trimmedApiKey
  }

  if (isEditing.value) {
    emit('save', payload)
    return
  }

  emit('save', {
    ...payload,
    name: normalizedName.value,
  })
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="640" :persistent="props.loading">
    <v-card rounded="xl" class="admin-providers-dialog">
      <v-card-title class="pt-6 px-6 text-h6">
        {{ title }}
      </v-card-title>

      <form class="admin-providers-dialog__form" @submit.prevent="save">
        <v-card-text class="px-6 pb-2">
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="name"
                label="Name"
                placeholder="openai-primary"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :readonly="isEditing"
                :error="Boolean(nameError)"
                :error-messages="nameError ? [nameError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="displayName"
                label="Display name"
                placeholder="OpenAI primary"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
              />
            </v-col>

            <v-col cols="12" md="5">
              <v-select
                v-model="type"
                :items="providerTypeOptions"
                label="Provider type"
                variant="outlined"
                density="comfortable"
              />
            </v-col>

            <v-col cols="12" md="7">
              <v-text-field
                v-model="baseUrl"
                label="Base URL"
                placeholder="https://api.openai.com/v1"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(baseUrlError)"
                :error-messages="baseUrlError ? [baseUrlError] : []"
              />
            </v-col>

            <v-col cols="12">
              <v-text-field
                v-model="apiKey"
                label="Virtual key"
                :placeholder="
                  props.provider?.hasApiKey
                    ? 'Leave blank to keep the saved key'
                    : 'Optional provider virtual key'
                "
                variant="outlined"
                density="comfortable"
                autocomplete="new-password"
                type="password"
                :disabled="clearApiKey"
              />
            </v-col>

            <v-col v-if="props.provider?.hasApiKey" cols="12">
              <v-checkbox
                v-model="clearApiKey"
                label="Clear saved virtual key"
                density="comfortable"
                hide-details
              />
            </v-col>
          </v-row>
        </v-card-text>

        <v-card-actions class="px-6 pb-6">
          <v-spacer />
          <AppDialogCloseButton label="Cancel" @click="isOpen = false" />
          <AppDialogActionButton
            color="primary"
            :label="submitLabel"
            type="submit"
            :loading="props.loading"
          />
        </v-card-actions>
      </form>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.admin-providers-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.admin-providers-dialog__form {
  display: contents;
}
</style>
