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
  <v-dialog v-model="isOpen" max-width="620" :persistent="props.loading">
    <v-card rounded="xl" class="service-account-dialog">
      <v-card-item class="px-6 pt-6 pb-2">
        <template #prepend>
          <v-avatar color="primary" variant="tonal" size="44">
            <v-icon icon="mdi-robot-outline" />
          </v-avatar>
        </template>

        <v-card-title class="text-h6">{{ title }}</v-card-title>
      </v-card-item>

      <form class="service-account-dialog__form" @submit.prevent="save">
        <v-card-text class="service-account-dialog__body px-6 pb-2">
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
.service-account-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
  background: linear-gradient(
    180deg,
    rgb(var(--app-shell-surface)) 0%,
    rgb(var(--app-shell-surface-muted)) 100%
  );
}

.service-account-dialog__form {
  display: contents;
}

.service-account-dialog__body {
  display: grid;
  gap: 16px;
}
</style>
