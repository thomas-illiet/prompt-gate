<script setup lang="ts">
import { markRaw } from 'vue'
import type { HelpSetupProvider } from '~/types/user-service'
import HelpSetupAspNetSnippet from '~/components/HelpSetup/docs/HelpSetupAspNetSnippet.vue'
import HelpSetupClaudeCodeSnippet from '~/components/HelpSetup/docs/HelpSetupClaudeCodeSnippet.vue'
import HelpSetupClineSnippet from '~/components/HelpSetup/docs/HelpSetupClineSnippet.vue'
import HelpSetupContinueSnippet from '~/components/HelpSetup/docs/HelpSetupContinueSnippet.vue'
import HelpSetupCurlSnippet from '~/components/HelpSetup/docs/HelpSetupCurlSnippet.vue'
import HelpSetupGoSnippet from '~/components/HelpSetup/docs/HelpSetupGoSnippet.vue'
import HelpSetupJavaSnippet from '~/components/HelpSetup/docs/HelpSetupJavaSnippet.vue'
import HelpSetupLuaSnippet from '~/components/HelpSetup/docs/HelpSetupLuaSnippet.vue'
import HelpSetupOpenClawSnippet from '~/components/HelpSetup/docs/HelpSetupOpenClawSnippet.vue'
import HelpSetupOpenCodeSnippet from '~/components/HelpSetup/docs/HelpSetupOpenCodeSnippet.vue'
import HelpSetupPowerShellSnippet from '~/components/HelpSetup/docs/HelpSetupPowerShellSnippet.vue'
import HelpSetupPythonSnippet from '~/components/HelpSetup/docs/HelpSetupPythonSnippet.vue'

const props = defineProps<{
  provider: HelpSetupProvider
}>()

const openAICompatibleDocuments = [
  {
    component: markRaw(HelpSetupCurlSnippet),
    icon: 'mdi-console-line',
    key: 'curl',
    title: 'curl',
  },
  {
    component: markRaw(HelpSetupPythonSnippet),
    icon: 'mdi-language-python',
    key: 'python',
    title: 'Python',
  },
  {
    component: markRaw(HelpSetupGoSnippet),
    icon: 'mdi-language-go',
    key: 'go',
    title: 'Go',
  },
  {
    component: markRaw(HelpSetupJavaSnippet),
    icon: 'mdi-language-java',
    key: 'java',
    title: 'Java',
  },
  {
    component: markRaw(HelpSetupAspNetSnippet),
    icon: 'mdi-dot-net',
    key: 'aspnet',
    title: 'ASP.NET',
  },
  {
    component: markRaw(HelpSetupPowerShellSnippet),
    icon: 'mdi-powershell',
    key: 'powershell',
    title: 'PowerShell',
  },
  {
    component: markRaw(HelpSetupLuaSnippet),
    icon: 'mdi-language-lua',
    key: 'lua',
    title: 'Lua',
  },
  {
    component: markRaw(HelpSetupClineSnippet),
    icon: 'mdi-robot-outline',
    key: 'cline',
    title: 'Cline',
  },
  {
    component: markRaw(HelpSetupContinueSnippet),
    icon: 'mdi-infinity',
    key: 'continue',
    title: 'Continue',
  },
  {
    component: markRaw(HelpSetupOpenClawSnippet),
    icon: 'mdi-application-braces-outline',
    key: 'openclaw',
    title: 'OpenClaw',
  },
  {
    component: markRaw(HelpSetupOpenCodeSnippet),
    icon: 'mdi-code-json',
    key: 'opencode',
    title: 'OpenCode',
  },
] as const

const anthropicDocuments = [
  {
    component: markRaw(HelpSetupClaudeCodeSnippet),
    icon: 'mdi-alpha-c-circle-outline',
    key: 'claude-code',
    title: 'Claude Code',
  },
] as const

const documents = computed(() =>
  props.provider.type === 'anthropic'
    ? anthropicDocuments
    : openAICompatibleDocuments,
)

type HelpDocumentKey =
  | (typeof openAICompatibleDocuments)[number]['key']
  | (typeof anthropicDocuments)[number]['key']
type HelpDocument =
  | (typeof openAICompatibleDocuments)[number]
  | (typeof anthropicDocuments)[number]

function byDocumentTitle(left: HelpDocument, right: HelpDocument) {
  return left.title.localeCompare(right.title, undefined, {
    sensitivity: 'base',
  })
}

const sortedDocuments = computed<HelpDocument[]>(() =>
  [...documents.value].sort(byDocumentTitle),
)
const activeDocumentKey = defineModel<HelpDocumentKey>('documentKey', {
  default: 'curl',
})
const selectedModel = defineModel<string>('model', { required: true })
const activeDocument = computed<HelpDocument>(
  () =>
    sortedDocuments.value.find(
      (document) => document.key === activeDocumentKey.value,
    ) ??
    sortedDocuments.value[0] ??
    openAICompatibleDocuments[0],
)

watch(
  documents,
  (items) => {
    if (!items.some((document) => document.key === activeDocumentKey.value)) {
      activeDocumentKey.value =
        props.provider.type === 'anthropic' ? 'claude-code' : 'curl'
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="help-setup-documentation-panel">
    <div class="help-setup-documentation-panel__sidebar">
      <div class="help-setup-documentation-panel__label">Documentation</div>
      <v-list
        bg-color="transparent"
        class="help-setup-documentation-panel__client-list"
        density="compact"
        nav
      >
        <v-list-item
          v-for="document in sortedDocuments"
          :key="document.key"
          :active="activeDocumentKey === document.key"
          class="help-setup-documentation-panel__client-item"
          :prepend-icon="document.icon"
          rounded="lg"
          :title="document.title"
          :value="document.key"
          @click="activeDocumentKey = document.key"
        />
      </v-list>
    </div>

    <div class="help-setup-documentation-panel__main">
      <v-select
        v-model="activeDocumentKey"
        bg-color="surface"
        class="help-setup-documentation-panel__client-select"
        density="comfortable"
        hide-details
        item-title="title"
        item-value="key"
        :items="sortedDocuments"
        menu-icon="mdi-chevron-down"
        prepend-inner-icon="mdi-file-code-outline"
        rounded="lg"
        variant="solo-filled"
      />

      <component
        :is="activeDocument.component"
        :model="selectedModel"
        :provider="props.provider"
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
  display: grid;
  gap: 10px;
  padding: 14px;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: var(--app-card-radius);
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.help-setup-documentation-panel__label {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.help-setup-documentation-panel__client-list {
  display: grid;
  gap: 4px;
  padding: 0;
}

.help-setup-documentation-panel__client-item {
  min-height: 42px;
}

.help-setup-documentation-panel__main {
  display: grid;
  min-width: 0;
  gap: 12px;
}

.help-setup-documentation-panel__client-select {
  display: none;
}

@media (max-width: 820px) {
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
