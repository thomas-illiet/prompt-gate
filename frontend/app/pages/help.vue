<script setup lang="ts">
import HelpSetupConfigurationCard from '~/components/HelpSetup/HelpSetupConfigurationCard.vue'
import HelpSetupDocumentationPanel from '~/components/HelpSetup/HelpSetupDocumentationPanel.vue'
import HelpSetupOperationalNotes from '~/components/HelpSetup/HelpSetupOperationalNotes.vue'
import HelpSetupProviderLoadingCard from '~/components/HelpSetup/HelpSetupProviderLoadingCard.vue'
import { availableSetupProviders } from '~/utils/help-setup'
import { guideSupportsProvider } from '~/utils/setup-guide-template'

definePageMeta({
  icon: 'mdi-help-circle-outline',
  title: 'Setup guide',
  drawerIndex: 4,
  requiredRoles: ['user', 'manager', 'admin'],
})

const helpSetup = useHelpSetup()
const providers = computed(() =>
  availableSetupProviders(helpSetup.setup.value?.providers ?? []),
)
const snippetSelection = useHelpSnippetSelection(providers)
type ModelSelectMode = 'all' | 'none' | 'single'

const selectedGuideId = shallowRef('')
const compatibleGuides = computed(() => {
  const provider = snippetSelection.selectedProvider.value
  if (!provider) return []
  return (helpSetup.setup.value?.guides ?? []).filter((guide) =>
    guideSupportsProvider(guide, provider),
  )
})
const activeGuide = computed(
  () =>
    compatibleGuides.value.find(
      (guide) => guide.id === selectedGuideId.value,
    ) ?? compatibleGuides.value[0],
)
const modelSelectMode = computed<ModelSelectMode>(() => {
  return activeGuide.value?.modelMode ?? 'single'
})
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-help-circle-outline"
          kicker="Clients"
          title="Setup guide"
          copy="Configure curl, SDKs, shell scripts, Cline, Continue, Claude Code, and OpenCode with your PromptGate endpoint."
          stat-label="Providers"
          :stat-value="String(providers.length)"
        />
      </v-col>

      <v-col v-if="helpSetup.error.value" cols="12">
        <v-alert type="warning" variant="tonal" rounded="lg">
          {{ helpSetup.error.value }}
        </v-alert>
      </v-col>

      <v-col v-if="helpSetup.loading.value && providers.length === 0" cols="12">
        <HelpSetupProviderLoadingCard />
      </v-col>

      <v-col
        v-if="!helpSetup.loading.value && providers.length === 0"
        cols="12"
      >
        <AppSectionCard
          icon="mdi-file-code-outline"
          title="Setup snippets"
          subtitle="Client configuration"
        >
          <AppEmptyState
            icon="mdi-cloud-question-outline"
            title="No accessible setup provider yet"
            text="Once your groups grant access to a supported provider, client snippets appear here ready to copy."
          />
        </AppSectionCard>
      </v-col>

      <v-col v-if="providers.length > 0" cols="12">
        <HelpSetupConfigurationCard
          v-model:model="snippetSelection.selectedModel.value"
          v-model:provider-name="snippetSelection.selectedProviderName.value"
          :model-options="snippetSelection.modelOptions.value"
          :model-select-mode="modelSelectMode"
          :providers="providers"
          :selected-provider="snippetSelection.selectedProvider.value"
        />
      </v-col>

      <v-col v-if="providers.length > 0" cols="12">
        <HelpSetupOperationalNotes />
      </v-col>

      <v-col
        v-if="
          snippetSelection.selectedProvider.value && compatibleGuides.length
        "
        cols="12"
      >
        <HelpSetupDocumentationPanel
          v-model:guide-id="selectedGuideId"
          v-model:model="snippetSelection.selectedModel.value"
          :guides="compatibleGuides"
          :provider="snippetSelection.selectedProvider.value"
        />
      </v-col>
    </v-row>
  </v-container>
</template>
