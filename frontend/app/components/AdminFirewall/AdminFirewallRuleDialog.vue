<script setup lang="ts">
import type {
  FirewallAction,
  FirewallRule,
  FirewallRulePayload,
} from '~/types/firewall'

const props = defineProps<{
  defaultPriority: number
  loading: boolean
  rule: FirewallRule | null
}>()

const emit = defineEmits<{
  save: [payload: FirewallRulePayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const address = shallowRef('')
const description = shallowRef('')
const priority = shallowRef(1)
const action = shallowRef<FirewallAction>('deny')
const hasSubmitted = shallowRef(false)

const actionOptions = [
  { title: 'Deny', value: 'deny' as const },
  { title: 'Allow', value: 'allow' as const },
]

const title = computed(() =>
  props.rule ? 'Update firewall rule' : 'Create firewall rule',
)
const submitLabel = computed(() => (props.rule ? 'Save rule' : 'Create rule'))
const normalizedPriority = computed(() =>
  Number.isFinite(priority.value) ? priority.value : 0,
)
const addressError = computed(() => {
  if (!hasSubmitted.value || address.value.trim()) {
    return ''
  }

  return 'Address is required.'
})
const priorityError = computed(() => {
  if (
    !hasSubmitted.value ||
    (normalizedPriority.value >= 1 && normalizedPriority.value <= 9999)
  ) {
    return ''
  }

  return 'Priority must be between 1 and 9999.'
})
const canSave = computed(
  () =>
    Boolean(address.value.trim()) &&
    normalizedPriority.value >= 1 &&
    normalizedPriority.value <= 9999,
)

watch(
  [isOpen, () => props.rule, () => props.defaultPriority],
  ([open]) => {
    if (!open) {
      return
    }

    address.value = props.rule?.address ?? ''
    description.value = props.rule?.description ?? ''
    priority.value = props.rule?.priority ?? props.defaultPriority
    action.value = props.rule?.action ?? 'deny'
    hasSubmitted.value = false
  },
  { immediate: true },
)

// updatePriority converts the numeric input value into rule priority state.
function updatePriority(value: string | number | null) {
  const nextValue = Number(value)
  priority.value = Number.isFinite(nextValue) ? nextValue : 0
}

// save validates the form and emits the firewall rule payload.
function save() {
  hasSubmitted.value = true

  if (!canSave.value) {
    return
  }

  emit('save', {
    address: address.value.trim(),
    description: description.value.trim(),
    priority: normalizedPriority.value,
    action: action.value,
    enabled: props.rule?.enabled ?? true,
  })
}
</script>

<template>
  <v-dialog v-model="isOpen" max-width="560" :persistent="props.loading">
    <v-card rounded="xl" class="admin-firewall-dialog">
      <v-card-title class="pt-6 px-6 text-h6">
        {{ title }}
      </v-card-title>

      <form class="admin-firewall-dialog__form" @submit.prevent="save">
        <v-card-text class="px-6 pb-2">
          <v-row>
            <v-col cols="12">
              <v-text-field
                v-model="address"
                label="IPv4 address or CIDR"
                placeholder="192.168.1.10 or 192.168.1.0/24"
                variant="outlined"
                density="comfortable"
                autocomplete="off"
                :error="Boolean(addressError)"
                :error-messages="addressError ? [addressError] : []"
              />
            </v-col>

            <v-col cols="12">
              <v-textarea
                v-model="description"
                label="Description"
                placeholder="Why this rule exists, ticket, owner, environment"
                variant="outlined"
                density="comfortable"
                rows="2"
                auto-grow
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                :model-value="priority"
                label="Priority"
                type="number"
                min="1"
                max="9999"
                step="1"
                variant="outlined"
                density="comfortable"
                :error="Boolean(priorityError)"
                :error-messages="priorityError ? [priorityError] : []"
                @update:model-value="updatePriority"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-select
                v-model="action"
                :items="actionOptions"
                label="Action"
                variant="outlined"
                density="comfortable"
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
.admin-firewall-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.admin-firewall-dialog__form {
  display: contents;
}
</style>
