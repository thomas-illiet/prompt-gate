<script setup lang="ts">
import type {
  ServiceAccount,
  ServiceAccountFormPayload,
} from '~/types/service-accounts'
import type { SubscriptionPlan } from '~/types/subscriptions'

const props = defineProps<{
  account: ServiceAccount | null
  loading: boolean
  subscriptionPlans: SubscriptionPlan[]
}>()

const emit = defineEmits<{
  save: [payload: ServiceAccountFormPayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const identifier = shallowRef('')
const name = shallowRef('')
const isActive = shallowRef(true)
const selectedPlanId = shallowRef<string | null>(null)

const title = computed(() =>
  props.account ? 'Update service account' : 'Create service account',
)
const submitLabel = computed(() =>
  props.account ? 'Save account' : 'Create account',
)
const formId = useId()
const subscriptionPlanOptions = computed(() => [
  { title: 'Inherit default', value: null },
  ...props.subscriptionPlans.map((plan) => ({
    title: plan.isDefault ? `${plan.name} (default)` : plan.name,
    value: plan.id,
  })),
])

watch(
  () => props.account,
  (account) => {
    identifier.value = account?.identifier ?? ''
    name.value = account?.name ?? ''
    isActive.value = account?.isActive ?? true
    selectedPlanId.value = account?.subscriptionPlanId ?? null
  },
  { immediate: true },
)

watch(isOpen, (open) => {
  if (!open || props.account) {
    return
  }

  identifier.value = ''
  name.value = ''
  isActive.value = true
  selectedPlanId.value = null
})

// save validates the form and emits the service account payload.
function save() {
  emit('save', {
    identifier: identifier.value.trim(),
    name: name.value.trim(),
    isActive: isActive.value,
    subscriptionPlanId: selectedPlanId.value,
  })
}
</script>

<template>
  <AppDialogCard v-model="isOpen" icon="mdi-robot-outline" :loading="props.loading" max-width="620" subtitle="Configure a non-human identity and its subscription plan." :title="title">
      <form :id="formId" @submit.prevent="save">
        <div class="service-account-dialog__body">
          <v-text-field
            v-model="identifier"
            label="Identifier"
            placeholder="ci_runner"
            prepend-inner-icon="mdi-at"
            variant="outlined"
            density="comfortable"
            :disabled="props.loading"
            required
          />

          <v-text-field
            v-model="name"
            label="Name"
            placeholder="CI runner"
            prepend-inner-icon="mdi-robot-outline"
            variant="outlined"
            density="comfortable"
            :disabled="props.loading"
            required
          />

          <v-select
            v-model="selectedPlanId"
            :items="subscriptionPlanOptions"
            label="Subscription plan"
            variant="outlined"
            density="comfortable"
            :disabled="props.loading"
          />
        </div>
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
.service-account-dialog__body {
  display: grid;
  gap: 16px;
}
</style>
