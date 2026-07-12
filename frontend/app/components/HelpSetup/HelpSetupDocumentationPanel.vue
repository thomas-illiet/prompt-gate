<script setup lang="ts">
import HelpSetupSnippetCard from '~/components/HelpSetup/HelpSetupSnippetCard.vue'
import type { HelpSetupProvider } from '~/types/user-service'
import type { SetupGuide } from '~/types/setup-guides'
import {
  renderSetupGuideTemplate,
  setupGuideContext,
} from '~/utils/setup-guide-template'

const props = defineProps<{
  provider: HelpSetupProvider
  guides: SetupGuide[]
}>()
const activeGuideId = defineModel<string>('guideId', { default: '' })
const selectedModel = defineModel<string>('model', { required: true })
const activeGuide = computed(
  () =>
    props.guides.find((guide) => guide.id === activeGuideId.value) ??
    props.guides[0] ??
    null,
)
const renderedCode = computed(() =>
  activeGuide.value
    ? renderSetupGuideTemplate(
        activeGuide.value.template,
        setupGuideContext(props.provider, selectedModel.value),
      )
    : '',
)

watch(
  () => props.guides,
  (guides) => {
    if (!guides.some((guide) => guide.id === activeGuideId.value))
      activeGuideId.value = guides[0]?.id ?? ''
  },
  { immediate: true },
)
</script>

<template>
  <div v-if="activeGuide" class="help-setup-documentation-panel">
    <div class="help-setup-documentation-panel__sidebar">
      <div class="help-setup-documentation-panel__label">Documentation</div>
      <v-list bg-color="transparent" density="compact" nav>
        <v-list-item
          v-for="guide in guides"
          :key="guide.id"
          :active="guide.id === activeGuideId"
          :prepend-icon="guide.icon"
          rounded="lg"
          :title="guide.title"
          @click="activeGuideId = guide.id"
        />
      </v-list>
    </div>
    <div class="help-setup-documentation-panel__main">
      <v-select
        v-model="activeGuideId"
        class="help-setup-documentation-panel__client-select"
        :items="guides"
        item-title="title"
        item-value="id"
        hide-details
      />
      <HelpSetupSnippetCard
        :code="renderedCode"
        :file-paths="activeGuide.filePaths"
        :subtitle="activeGuide.subtitle"
        :title="activeGuide.title"
      />
    </div>
  </div>
</template>

<style scoped>
.help-setup-documentation-panel {
  display: grid;
  align-items: start;
  gap: 16px;
  grid-template-columns: 220px minmax(0, 1fr);
}
.help-setup-documentation-panel__sidebar {
  position: sticky;
  top: 88px;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: var(--app-card-radius);
  background: rgb(var(--app-shell-surface));
}
.help-setup-documentation-panel__label {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.help-setup-documentation-panel__main {
  display: grid;
  min-width: 0;
  gap: 12px;
}
.help-setup-documentation-panel__client-select {
  display: none;
}
@media (max-width: 959px) {
  .help-setup-documentation-panel {
    grid-template-columns: 1fr;
  }
  .help-setup-documentation-panel__sidebar {
    display: none;
  }
  .help-setup-documentation-panel__client-select {
    display: block;
  }
}
</style>
