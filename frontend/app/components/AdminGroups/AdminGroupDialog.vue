<script setup lang="ts">
import type {
  AccessGroup,
  CreateGroupPayload,
  GroupModelPatternValidationPayload,
  GroupModelPatternValidationResponse,
  UpdateGroupPayload,
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
  save: [payload: CreateGroupPayload | UpdateGroupPayload]
  validateModels: [payload: GroupModelPatternValidationPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const name = shallowRef('')
const displayName = shallowRef('')
const description = shallowRef('')
const providerIds = shallowRef<string[]>([])
const modelPatterns = shallowRef<string[]>([])
const excludedModelPatterns = shallowRef<string[]>([])
const hasSubmitted = shallowRef(false)
const hasValidatedModels = shallowRef(false)
const displayNameEdited = shallowRef(false)
const allModelsPattern = '.*'

const title = computed(() => (props.group ? 'Update group' : 'Create group'))
const submitLabel = computed(() =>
  props.group ? 'Save group' : 'Create group',
)
const isEditing = computed(() => Boolean(props.group))
const normalizedName = computed(() => name.value.trim().toLowerCase())
const normalizedDisplayName = computed(() => displayName.value.trim())
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
const uniqueNormalizedPatterns = computed(() => [
  ...new Set(normalizedPatterns.value),
])
const normalizedExcludedPatterns = computed(() =>
  excludedModelPatterns.value.map((pattern) => pattern.trim()).filter(Boolean),
)
const uniqueNormalizedExcludedPatterns = computed(() => [
  ...new Set(normalizedExcludedPatterns.value),
])
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
const displayNameError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }
  if (!normalizedDisplayName.value) {
    return 'Display name is required.'
  }
  return ''
})
const providerError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }
  if (providerIds.value.length === 0) {
    return 'Select at least one provider.'
  }
  return ''
})
const currentRegexError = computed(() => {
  for (const pattern of [
    ...normalizedPatterns.value,
    ...normalizedExcludedPatterns.value,
  ]) {
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
  () =>
    (isEditing.value || Boolean(normalizedName.value)) &&
    Boolean(normalizedDisplayName.value) &&
    providerIds.value.length > 0 &&
    !nameError.value &&
    !displayNameError.value &&
    !providerError.value &&
    !regexError.value,
)
const formId = useId()
const canValidateModels = computed(
  () =>
    (uniqueNormalizedPatterns.value.length > 0 ||
      uniqueNormalizedExcludedPatterns.value.length > 0) &&
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
    displayName.value =
      props.group?.displayName || formatDisplayName(props.group?.name ?? '')
    displayNameEdited.value = Boolean(props.group)
    description.value = props.group?.description ?? ''
    providerIds.value =
      props.group?.providers.map((provider) => provider.id) ?? []
    modelPatterns.value = props.group?.modelPatterns.length
      ? props.group.modelPatterns
      : [allModelsPattern]
    excludedModelPatterns.value = props.group?.excludedModelPatterns ?? []
    hasSubmitted.value = false
    hasValidatedModels.value = false
    emit('clearModelValidation')
  },
  { immediate: true },
)

watch(name, () => {
  if (!isOpen.value || displayNameEdited.value) {
    return
  }
  displayName.value = formatDisplayName(name.value)
})

watch([providerIds, modelPatterns, excludedModelPatterns], () => {
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

  const payload: UpdateGroupPayload = {
    displayName: normalizedDisplayName.value,
    description: description.value.trim(),
    providerIds: providerIds.value,
    modelPatterns: uniqueNormalizedPatterns.value,
    excludedModelPatterns: uniqueNormalizedExcludedPatterns.value,
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

function updateDisplayName(value: string | null) {
  displayNameEdited.value = true
  displayName.value = value ?? ''
}

function formatDisplayName(value: string) {
  return value
    .trim()
    .toLowerCase()
    .split(/[-\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function validateModels() {
  hasValidatedModels.value = true
  if (!canValidateModels.value) {
    return
  }

  emit('validateModels', {
    providerIds: providerIds.value,
    modelPatterns: uniqueNormalizedPatterns.value,
    excludedModelPatterns: uniqueNormalizedExcludedPatterns.value,
  })
}
</script>

<template>
  <AppDialogCard v-model="isOpen" icon="mdi-account-group-outline" :loading="props.loading" max-width="720" subtitle="Configure provider access and the model patterns available to this group." :title="title">
      <form :id="formId" @submit.prevent="save">
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="name"
                label="Name"
                placeholder="engineering"
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
                :model-value="displayName"
                label="Display name"
                placeholder="Engineering"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(displayNameError)"
                :error-messages="displayNameError ? [displayNameError] : []"
                @update:model-value="updateDisplayName"
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
                :error="Boolean(providerError)"
                :error-messages="providerError ? [providerError] : []"
              />
            </v-col>

            <v-col cols="12">
              <div class="admin-group-dialog__model-validation">
                <div class="admin-group-dialog__model-patterns">
                  <v-combobox
                    v-model="modelPatterns"
                    label="Allowed model regex"
                    placeholder=".*"
                    variant="outlined"
                    density="comfortable"
                    multiple
                    chips
                    closable-chips
                    :error="Boolean(regexError)"
                    :error-messages="regexError ? [regexError] : []"
                  />
                  <v-combobox
                    v-model="excludedModelPatterns"
                    label="Excluded model regex"
                    placeholder="^bge"
                    variant="outlined"
                    density="comfortable"
                    multiple
                    chips
                    closable-chips
                    :error="Boolean(regexError)"
                    :error-messages="regexError ? [regexError] : []"
                  />
                </div>
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
      </form>
      <template #actions>
          <AppDialogCloseButton :disabled="props.loading" label="Cancel" @click="isOpen = false" />
          <AppDialogActionButton
            color="primary"
            :form="formId"
            :label="submitLabel"
            type="submit"
            :loading="props.loading"
          />
      </template>
  </AppDialogCard>
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

.admin-group-dialog__model-patterns {
  display: grid;
  gap: 4px;
}

.admin-group-dialog__model-validation-button {
  height: 48px;
  min-height: 48px;
}
</style>
