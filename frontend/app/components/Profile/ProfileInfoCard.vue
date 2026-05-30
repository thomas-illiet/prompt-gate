<script setup lang="ts">
const props = defineProps<{
  icon: string
  subtitle: string
  title: string
}>()
</script>

<template>
  <AppSectionCard
    :icon="props.icon"
    :title="props.title"
    :subtitle="props.subtitle"
  >
    <slot />
  </AppSectionCard>
</template>
