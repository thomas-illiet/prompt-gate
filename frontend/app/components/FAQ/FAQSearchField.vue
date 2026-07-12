<script setup lang="ts">
const props = defineProps<{
  resultCount: number
  totalCount: number
}>()

const search = defineModel<string>({ default: '' })
const resultLabel = computed(() => {
  if (!search.value.trim()) {
    return props.totalCount === 1 ? '1 question' : `${props.totalCount} questions`
  }

  return props.resultCount === 1
    ? '1 result'
    : `${props.resultCount} results`
})
</script>

<template>
  <div class="faq-search">
    <v-text-field
      v-model="search"
      aria-label="Search FAQ questions"
      bg-color="surface"
      clearable
      density="comfortable"
      hide-details
      placeholder="Search the FAQ…"
      prepend-inner-icon="mdi-magnify"
      rounded="xl"
      variant="solo-filled"
    />
    <v-chip
      class="faq-search__count"
      color="primary"
      prepend-icon="mdi-text-search"
      size="small"
      variant="tonal"
    >
      {{ resultLabel }}
    </v-chip>
  </div>
</template>

<style scoped>
.faq-search {
  position: relative;
  width: 100%;
}

.faq-search :deep(.v-field) {
  border: 1px solid rgba(var(--v-theme-primary), 0.16);
  box-shadow: 0 8px 24px rgba(var(--v-theme-primary), 0.08);
}

.faq-search :deep(.v-field--focused) {
  border-color: rgba(var(--v-theme-primary), 0.48);
  box-shadow: 0 10px 28px rgba(var(--v-theme-primary), 0.14);
}

.faq-search__count {
  position: absolute;
  top: 50%;
  right: 48px;
  transform: translateY(-50%);
  pointer-events: none;
}

@media (max-width: 600px) {
  .faq-search__count {
    display: none;
  }
}
</style>
