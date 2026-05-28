<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    disabled?: boolean
    label?: string
    type?: 'button' | 'reset' | 'submit'
  }>(),
  {
    disabled: false,
    label: 'Close',
    type: 'button',
  },
)

const emit = defineEmits<{
  click: []
}>()
</script>

<template>
  <AppDialogActionButton
    color="primary"
    variant="tonal"
    :disabled="props.disabled"
    :label="props.label"
    :type="props.type"
    @click="emit('click')"
  />
</template>
