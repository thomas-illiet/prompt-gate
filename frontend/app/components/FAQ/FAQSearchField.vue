<script setup lang="ts">
const props = defineProps<{
  resultCount: number
  totalCount: number
}>()

const search = defineModel<string>({ default: '' })
const resultLabel = computed(() => {
  if (!search.value.trim()) {
    return props.totalCount === 1
      ? '1 question'
      : `${props.totalCount} questions`
  }

  return props.resultCount === 1 ? '1 result' : `${props.resultCount} results`
})
</script>

<template>
  <div class="faq-search">
    <v-text-field
      v-model="search"
      aria-label="Search FAQ questions"
      bg-color="surface"
      clearable
      density="compact"
      hide-details
      placeholder="Search the FAQ…"
      prepend-inner-icon="mdi-magnify"
      rounded="xl"
      variant="solo-filled"
    />
    <span class="faq-search__status" role="status" aria-live="polite">
      {{ resultLabel }}
    </span>
  </div>
</template>

<style scoped>
.faq-search {
  position: relative;
  display: grid;
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

.faq-search__status {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  border: 0;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
}
</style>
