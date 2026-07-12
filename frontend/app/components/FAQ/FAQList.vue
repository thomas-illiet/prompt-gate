<script setup lang="ts">
import type { PublicFAQEntry } from '~/types/faq'

const props = defineProps<{ entries: PublicFAQEntry[]; loading: boolean }>()
const search = shallowRef('')
const normalizedSearch = computed(() => search.value.trim().toLocaleLowerCase())
const filteredEntries = computed(() => {
  if (!normalizedSearch.value) return props.entries
  return props.entries.filter((entry) => entry.question.toLocaleLowerCase().includes(normalizedSearch.value))
})
const questionsSubtitle = computed(() => {
  if (props.loading) return 'Loading published documentation…'
  if (normalizedSearch.value) {
    return filteredEntries.value.length === 1
      ? '1 matching question'
      : `${filteredEntries.value.length} matching questions`
  }
  return props.entries.length === 1
    ? '1 published question'
    : `${props.entries.length} published questions`
})
</script>

<template>
  <div class="faq-list">
    <AppSectionCard
      icon="mdi-magnify"
      title="Search the FAQ"
      subtitle="Quickly find documentation by question."
    >
      <div class="faq-list__search">
        <FAQSearchField
          v-model="search"
          :result-count="filteredEntries.length"
          :total-count="props.entries.length"
        />
      </div>
    </AppSectionCard>

    <AppSectionCard
      icon="mdi-frequently-asked-questions"
      title="Questions"
      :subtitle="questionsSubtitle"
    >
      <div class="faq-list__questions">
        <v-skeleton-loader
          v-if="props.loading"
          type="list-item-three-line@4"
        />
        <AppEmptyState
          v-else-if="filteredEntries.length === 0"
          icon="mdi-text-search"
          :title="props.entries.length ? 'No matching questions' : 'No FAQ entries yet'"
          :text="props.entries.length ? 'Try a different search.' : 'Published documentation will appear here.'"
        />
        <v-expansion-panels
          v-else
          class="faq-list__panels"
          multiple
        >
          <v-expansion-panel
            v-for="entry in filteredEntries"
            :key="entry.id"
            class="faq-list__panel"
          >
            <v-expansion-panel-title class="faq-list__question">
              {{ entry.question }}
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <!-- eslint-disable-next-line vue/no-v-html -- backend-sanitized HTML -->
              <article class="app-markdown" v-html="entry.renderedHtml" />
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>
      </div>
    </AppSectionCard>
  </div>
</template>

<style scoped>
.faq-list {
  display: grid;
  gap: 20px;
}

.faq-list__search {
  padding: 4px 24px 24px;
}

.faq-list__questions {
  padding: 0 24px 24px;
}

.faq-list__panels {
  display: flex;
  flex-direction: column;
  flex-wrap: nowrap;
  align-items: stretch;
  justify-content: flex-start;
  width: 100%;
  gap: 10px;
}

.faq-list__panel {
  flex: 0 0 auto !important;
  width: 100%;
  max-width: none !important;
  box-sizing: border-box;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.42);
  border-radius: var(--app-card-radius) !important;
  color: rgb(var(--v-theme-on-surface));
  background: rgba(var(--app-shell-surface-muted), 0.58) !important;
  box-shadow: none !important;
  transition:
    border-color 160ms ease,
    background-color 160ms ease;
}

.faq-list__panel::after {
  display: none;
}

.faq-list__question {
  min-height: 52px;
  padding: 12px 16px;
  font-size: 0.95rem;
  font-weight: 650;
  line-height: 1.4;
  background: transparent;
}

.faq-list__question:hover,
.faq-list__question:focus-visible,
.faq-list__panel.v-expansion-panel--active .faq-list__question {
  background: rgba(var(--v-theme-primary), 0.045);
}

.faq-list__panel:hover,
.faq-list__panel:focus-within {
  border-color: rgba(var(--v-theme-primary), 0.24);
}

.faq-list__panel :deep(.v-expansion-panel-text__wrapper) {
  padding: 4px 16px 16px;
  border-top: 1px solid rgba(var(--app-shell-border), 0.32);
  background: rgba(var(--app-shell-surface-strong), 0.72);
}

@media (max-width: 600px) {
  .faq-list__search {
    padding-inline: 16px;
  }

  .faq-list__questions {
    padding-inline: 16px;
  }
}
</style>
