<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    color?: string
    disabled?: boolean
    label: string
    loading?: boolean
    prependIcon?: string
    ariaLabel?: string
    type?: 'button' | 'reset' | 'submit'
    variant?: 'flat' | 'elevated' | 'tonal'
  }>(),
  {
    color: 'primary',
    disabled: false,
    loading: false,
    prependIcon: '',
    ariaLabel: '',
    type: 'button',
    variant: 'flat',
  },
)

const emit = defineEmits<{
  click: []
}>()
</script>

<template>
  <v-btn
    :color="props.color"
    :disabled="props.disabled"
    :loading="props.loading"
    :prepend-icon="props.prependIcon || undefined"
    :aria-label="props.ariaLabel || props.label"
    :type="props.type"
    :variant="props.variant"
    rounded="lg"
    class="app-dialog-action-button"
    @click="emit('click')"
  >
    {{ props.label }}
  </v-btn>
</template>

<style scoped>
.app-dialog-action-button {
  min-width: 128px;
}
</style>
