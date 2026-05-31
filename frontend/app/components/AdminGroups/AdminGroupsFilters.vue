<script setup lang="ts">
const props = defineProps<{
  search: string
}>()

const emit = defineEmits<{
  'update:search': [value: string]
}>()
</script>

<template>
  <AppFilterCard
    title="Group filters"
    subtitle="Search access groups by name or description."
  >
    <v-text-field
      :model-value="props.search"
      prepend-inner-icon="mdi-magnify"
      label="Search groups"
      placeholder="Name or description"
      density="compact"
      variant="outlined"
      flat
      clearable
      hide-details
      @update:model-value="
        emit('update:search', ($event as string | null) ?? '')
      "
    />
  </AppFilterCard>
</template>
