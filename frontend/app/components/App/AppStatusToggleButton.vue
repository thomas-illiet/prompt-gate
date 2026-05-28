<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    active: boolean
    activeLabel?: string
    inactiveLabel?: string
  }>(),
  {
    activeLabel: 'Enabled',
    inactiveLabel: 'Disabled',
  },
)

const emit = defineEmits<{
  click: []
}>()
</script>

<template>
  <v-btn
    size="small"
    variant="tonal"
    rounded="lg"
    :color="props.active ? 'success' : 'grey'"
    class="text-none"
    @click="emit('click')"
  >
    {{ props.active ? props.activeLabel : props.inactiveLabel }}
  </v-btn>
</template>
