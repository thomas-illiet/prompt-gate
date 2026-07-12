<script setup lang="ts">
import type {
  SubscriptionPlan,
  SubscriptionPlanPayload,
} from '~/types/subscriptions'

const props = defineProps<{
  loading: boolean
  plan: SubscriptionPlan | null
}>()

const emit = defineEmits<{
  save: [payload: SubscriptionPlanPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const name = shallowRef('')
const description = shallowRef('')
const quota5h = shallowRef('')
const quota7d = shallowRef('')
const isDefault = shallowRef(false)
const hasSubmitted = shallowRef(false)

const title = computed(() =>
  props.plan ? 'Update subscription plan' : 'Create subscription plan',
)
const submitLabel = computed(() =>
  props.plan ? 'Save plan' : 'Create plan',
)
const nameError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }
  return name.value.trim() ? '' : 'Name is required.'
})
const quota5hError = computed(() => quotaError(quota5h.value, hasSubmitted.value))
const quota7dError = computed(() => quotaError(quota7d.value, hasSubmitted.value))
const canSave = computed(
  () => !nameError.value && !quota5hError.value && !quota7dError.value,
)
const formId = useId()

watch(
  [isOpen, () => props.plan],
  ([open]) => {
    if (!open) {
      return
    }
    name.value = props.plan?.name ?? ''
    description.value = props.plan?.description ?? ''
    quota5h.value = quotaInputValue(props.plan?.quota5hTokens)
    quota7d.value = quotaInputValue(props.plan?.quota7dTokens)
    isDefault.value = props.plan?.isDefault ?? false
    hasSubmitted.value = false
  },
  { immediate: true },
)

function quotaInputValue(value: number | null | undefined) {
  return value == null ? '' : String(value)
}

function parseQuota(value: string) {
  const trimmed = value.trim()
  if (!trimmed) {
    return null
  }
  const parsed = Number(trimmed)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

function quotaError(value: string, submitted: boolean) {
  if (!submitted || !value.trim()) {
    return ''
  }
  const parsed = Number(value.trim())
  return Number.isInteger(parsed) && parsed > 0
    ? ''
    : 'Use a positive whole number or leave blank.'
}

function save() {
  hasSubmitted.value = true
  if (!canSave.value) {
    return
  }
  emit('save', {
    name: name.value.trim(),
    description: description.value.trim(),
    quota5hTokens: parseQuota(quota5h.value),
    quota7dTokens: parseQuota(quota7d.value),
    isDefault: isDefault.value,
  })
}
</script>

<template>
  <AppDialogCard v-model="isOpen" icon="mdi-wallet-membership" :loading="props.loading" max-width="640" subtitle="Define token allowances and whether this plan is assigned by default." :title="title">
      <form :id="formId" @submit.prevent="save">
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="name"
                label="Name"
                placeholder="Team Pro"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(nameError)"
                :error-messages="nameError ? [nameError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-checkbox
                v-model="isDefault"
                label="Default plan"
                density="comfortable"
                hide-details
              />
            </v-col>

            <v-col cols="12">
              <v-textarea
                v-model="description"
                label="Description"
                rows="3"
                variant="outlined"
                density="comfortable"
                auto-grow
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="quota5h"
                label="5h token quota"
                placeholder="Leave blank for unlimited"
                variant="outlined"
                density="comfortable"
                inputmode="numeric"
                :error="Boolean(quota5hError)"
                :error-messages="quota5hError ? [quota5hError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                v-model="quota7d"
                label="7d token quota"
                placeholder="Leave blank for unlimited"
                variant="outlined"
                density="comfortable"
                inputmode="numeric"
                :error="Boolean(quota7dError)"
                :error-messages="quota7dError ? [quota7dError] : []"
              />
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
.subscription-plan-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.subscription-plan-dialog__form {
  display: contents;
}
</style>
