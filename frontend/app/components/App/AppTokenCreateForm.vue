<script setup lang="ts">
export interface AppTokenCreatePayload {
  description: string
  expiresInDays: number
  name: string
}

const props = withDefaults(
  defineProps<{
    autofocus?: boolean
    defaultLifetime?: number
    inline?: boolean
    loading: boolean
    namePlaceholder?: string
    submitIcon?: string
    submitLabel?: string
  }>(),
  {
    autofocus: false,
    defaultLifetime: 30,
    inline: true,
    namePlaceholder: 'personal_cli',
    submitIcon: 'mdi-plus',
    submitLabel: 'Generate key',
  },
)

const emit = defineEmits<{
  create: [payload: AppTokenCreatePayload]
}>()

const tokenName = shallowRef('')
const description = shallowRef('')
const expiresInDays = shallowRef(props.defaultLifetime)
const isInvalid = computed(
  () =>
    tokenName.value.trim().length === 0 ||
    Number(expiresInDays.value) < 1 ||
    Number(expiresInDays.value) > 365,
)

watch(
  () => props.defaultLifetime,
  (value) => {
    expiresInDays.value = value
  },
)

// reset clears the token form back to defaults.
function reset() {
  tokenName.value = ''
  description.value = ''
  expiresInDays.value = props.defaultLifetime
}

// createToken emits the normalized token creation payload.
function createToken() {
  if (isInvalid.value) {
    return
  }

  emit('create', {
    description: description.value.trim(),
    expiresInDays: Number(expiresInDays.value),
    name: tokenName.value.trim(),
  })
}

defineExpose({ reset })
</script>

<template>
  <form
    class="app-token-create-form"
    :class="{ 'app-token-create-form--inline': props.inline }"
    @submit.prevent="createToken"
  >
    <v-text-field
      v-model="tokenName"
      label="Virtual key name"
      :placeholder="props.namePlaceholder"
      prepend-inner-icon="mdi-key-outline"
      variant="outlined"
      density="comfortable"
      :autofocus="props.autofocus"
      :disabled="props.loading"
      required
    />

    <v-text-field
      v-model="description"
      label="Description"
      prepend-inner-icon="mdi-text-box-outline"
      variant="outlined"
      density="comfortable"
      :disabled="props.loading"
    />

    <v-text-field
      v-model.number="expiresInDays"
      label="Lifetime"
      suffix="days"
      type="number"
      min="1"
      max="365"
      prepend-inner-icon="mdi-calendar-clock"
      variant="outlined"
      density="comfortable"
      :disabled="props.loading"
      required
    />

    <AppDialogActionButton
      color="primary"
      class="app-token-create-form__submit"
      :disabled="isInvalid"
      :label="props.submitLabel"
      :loading="props.loading"
      :prepend-icon="props.submitIcon"
      type="submit"
    />
  </form>
</template>

<style scoped>
.app-token-create-form {
  display: grid;
  gap: 12px;
}

.app-token-create-form__submit {
  min-height: 44px;
}

@media (min-width: 960px) {
  .app-token-create-form--inline {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr) 160px auto;
    align-items: start;
  }

  .app-token-create-form--inline .app-token-create-form__submit {
    margin-top: 2px;
  }
}
</style>
