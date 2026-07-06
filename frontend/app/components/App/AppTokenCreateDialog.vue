<script setup lang="ts">
import type { AppTokenCreatePayload } from '~/components/App/AppTokenCreateForm.vue'

const props = withDefaults(
  defineProps<{
    defaultLifetime?: number
    loading: boolean
    maxLifetime?: number
    maxWidth?: number | string
    namePlaceholder?: string
    submitIcon?: string
    submitLabel?: string
    subtitle?: string
    title?: string
  }>(),
  {
    defaultLifetime: 30,
    maxLifetime: 365,
    maxWidth: 760,
    namePlaceholder: 'personal_cli',
    submitIcon: 'mdi-plus',
    submitLabel: 'Generate key',
    subtitle: 'Generate a virtual key for CLI, SDK, or proxy access.',
    title: 'Create virtual key',
  },
)

const emit = defineEmits<{
  create: [payload: AppTokenCreatePayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const createFormKey = shallowRef(0)

watch(isOpen, (open) => {
  if (open) {
    createFormKey.value += 1
  }
})
</script>

<template>
  <AppDialogCard
    v-model="isOpen"
    icon="mdi-key-plus"
    :max-width="props.maxWidth"
    :title="props.title"
    :subtitle="props.subtitle"
    :loading="props.loading"
  >
    <AppTokenCreateForm
      :key="createFormKey"
      :autofocus="true"
      :default-lifetime="props.defaultLifetime"
      :inline="false"
      :loading="props.loading"
      :max-lifetime="props.maxLifetime"
      :name-placeholder="props.namePlaceholder"
      :submit-icon="props.submitIcon"
      :submit-label="props.submitLabel"
      @create="emit('create', $event)"
    />

    <template #actions>
      <v-spacer />
      <AppDialogCloseButton :disabled="props.loading" @click="isOpen = false" />
    </template>
  </AppDialogCard>
</template>
