<script setup lang="ts">
import type {
  AccessGroup,
  GroupModelPatternValidationPayload,
  GroupModelPatternValidationResponse,
  GroupPayload,
} from '~/types/groups'
import type { Provider } from '~/types/providers'

const props = defineProps<{
  group: AccessGroup | null
  loading: boolean
  modelValidation: GroupModelPatternValidationResponse | null
  modelValidationError: string | null
  modelValidationLoading: boolean
  providers: Provider[]
}>()

const emit = defineEmits<{
  clearModelValidation: []
  save: [payload: GroupPayload]
  validateModels: [payload: GroupModelPatternValidationPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const name = shallowRef('')
const displayName = shallowRef('')
const description = shallowRef('')
const providerIds = shallowRef<string[]>([])
const modelPatterns = shallowRef<string[]>([])
const hasSubmitted = shallowRef(false)
const hasValidatedModels = shallowRef(false)

const title = computed(() => (props.group ? 'Update group' : 'Create group'))
const submitLabel = computed(() =>
  props.group ? 'Save group' : 'Create group',
)
const normalizedName = computed(() => name.value.trim().toLowerCase())
const providerItems = computed(() =>
  props.providers.map((provider) => ({
    title: provider.displayName || provider.name,
    value: provider.id,
    props: {
      subtitle: provider.name,
    },
  })),
)
const normalizedPatterns = computed(() =>
  modelPatterns.value.map((pattern) => pattern.trim()).filter(Boolean),
)
const uniqueNormalizedPatterns = computed(() => [...new Set(normalizedPatterns.value)])
const nameError = computed(() => {
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
const currentRegexError = computed(() => {
  for (const pattern of normalizedPatterns.value) {
    try {
      new RegExp(pattern)
    } catch {
      return `Invalid regex: ${pattern}`
    }
  }
  return ''
})
const regexError = computed(() =>
  hasSubmitted.value || hasValidatedModels.value ? currentRegexError.value : '',
)
const canSave = computed(
  () => Boolean(normalizedName.value) && !nameError.value && !regexError.value,
)
const canValidateModels = computed(
  () =>
    uniqueNormalizedPatterns.value.length > 0 &&
    !currentRegexError.value &&
    !props.modelValidationLoading,
)
const modelValidationSummary = computed(() => {
  const result = props.modelValidation
  if (!result) {
    return ''
  }
  const matchLabel =
    result.matchedModelCount === 1
      ? '1 model matched.'
      : `${result.matchedModelCount} models matched.`
  if (result.unavailableProviderCount === 0) {
    return matchLabel
  }
  const providerLabel =
    result.unavailableProviderCount === 1
      ? '1 provider could not be checked.'
      : `${result.unavailableProviderCount} providers could not be checked.`
  return `${matchLabel} ${providerLabel}`
})
const modelValidationTone = computed(() =>
  (props.modelValidation?.matchedModelCount ?? 0) > 0 ? 'success' : 'warning',
)

watch(
  [isOpen, () => props.group],
  ([open]) => {
    if (!open) {
      return
    }
    name.value = props.group?.name ?? ''
    displayName.value = props.group?.displayName ?? ''
    description.value = props.group?.description ?? ''
    providerIds.value =
      props.group?.providers.map((provider) => provider.id) ?? []
    modelPatterns.value = props.group?.modelPatterns ?? []
    hasSubmitted.value = false
    hasValidatedModels.value = false
    emit('clearModelValidation')
  },
  { immediate: true },
)

watch([providerIds, modelPatterns], () => {
  if (!isOpen.value) {
    return
  }
  hasValidatedModels.value = false
  emit('clearModelValidation')
})

function save() {
  hasSubmitted.value = true
  if (!canSave.value) {
    return
  }

  emit('save', {
    name: normalizedName.value,
    displayName: displayName.value.trim(),
    description: description.value.trim(),
    providerIds: providerIds.value,
    modelPatterns: uniqueNormalizedPatterns.value,
  })
}

function validateModels() {
  hasValidatedModels.value = true
  if (!canValidateModels.value) {
    return
  }

  emit('validateModels', {
    providerIds: providerIds.value,
    modelPatterns: uniqueNormalizedPatterns.value,
  })
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="720" :persistent="props.loading">
    <v-card rounded="xl" class="admin-group-dialog">
      <v-card-title class="pt-6 px-6 text-h6">
        {{ title }}
      </v-card-title>

      <form class="admin-group-dialog__form" @submit.prevent="save">
        <v-card-text class="px-6 pb-2">
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="name"
                label="Name"
                placeholder="engineering"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(nameError)"
                :error-messages="nameError ? [nameError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="displayName"
                label="Display name"
                placeholder="Engineering"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
              />
            </v-col>

            <v-col cols="12">
              <v-textarea
                v-model="description"
                label="Description"
                variant="outlined"
                density="comfortable"
                rows="2"
                auto-grow
              />
            </v-col>

            <v-col cols="12">
              <v-select
                v-model="providerIds"
                :items="providerItems"
                label="Allowed providers"
                variant="outlined"
                density="comfortable"
                multiple
                chips
                closable-chips
              />
            </v-col>

            <v-col cols="12">
              <div class="admin-group-dialog__model-validation">
                <v-combobox
                  v-model="modelPatterns"
                  label="Allowed model regex"
                  placeholder="^gpt-5"
                  variant="outlined"
                  density="comfortable"
                  multiple
                  chips
                  closable-chips
                  :error="Boolean(regexError)"
                  :error-messages="regexError ? [regexError] : []"
                />
                <v-btn
                  color="primary"
                  variant="tonal"
                  rounded="lg"
                  prepend-icon="mdi-check-decagram-outline"
                  class="admin-group-dialog__model-validation-button"
                  :disabled="!canValidateModels"
                  :loading="props.modelValidationLoading"
                  @click="validateModels"
                >
                  Validate
                </v-btn>
              </div>

              <v-alert
                v-if="props.modelValidation"
                :type="modelValidationTone"
                variant="tonal"
                rounded="lg"
                density="compact"
              >
                {{ modelValidationSummary }}
              </v-alert>

              <v-alert
                v-else-if="props.modelValidationError"
                type="warning"
                variant="tonal"
                rounded="lg"
                density="compact"
              >
                {{ props.modelValidationError }}
              </v-alert>
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
.admin-group-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.admin-group-dialog__form {
  display: contents;
}

.admin-group-dialog__model-validation {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: start;
}

.admin-group-dialog__model-validation-button {
  height: 48px;
  min-height: 48px;
}
</style>
