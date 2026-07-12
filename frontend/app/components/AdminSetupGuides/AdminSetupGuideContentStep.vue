<script setup lang="ts">
import AdminSetupGuidePreview from './AdminSetupGuidePreview.vue'
import type { SetupGuidePayload } from '~/types/setup-guides'
import { SETUP_GUIDE_VARIABLES } from '~/utils/setup-guide-template'

const props = defineProps<{
  guide: SetupGuidePayload
  templateError: string | null
}>()
const filePathsText = defineModel<string>('filePathsText', { required: true })
const template = defineModel<string>('template', { required: true })
const activeTab = shallowRef<'template' | 'preview'>('template')
const variables = SETUP_GUIDE_VARIABLES.map((value) => `{{${value}}}`)
const modelLoopHelp = '{{#models}}...{{/models}}'
</script>

<template>
  <section class="setup-guide-content" aria-labelledby="guide-content-title">
    <div class="setup-guide-content__heading">
      <h3 id="guide-content-title">Guide content</h3>
      <p>Write the copy shown to users and verify the final rendering.</p>
    </div>

    <v-tabs v-model="activeTab" class="setup-guide-content__tabs" grow>
      <v-tab value="template" prepend-icon="mdi-code-braces">
        Template
      </v-tab>
      <v-tab value="preview" prepend-icon="mdi-eye-outline">
        Preview
      </v-tab>
    </v-tabs>

    <div v-show="activeTab === 'template'" class="setup-guide-content__panel">
      <div class="setup-guide-content__editor">
        <v-textarea
          v-model="template"
          class="setup-guide-content__textarea"
          label="Template"
          placeholder="Use {{baseUrl}}, {{token}} and {{model}} where needed."
          variant="outlined"
          rows="7"
          hide-details="auto"
          :error-messages="props.templateError ? [props.templateError] : []"
        />
      </div>

      <div class="setup-guide-content__secondary">
        <div class="setup-guide-content__card">
          <div class="setup-guide-content__card-heading">
            <v-icon icon="mdi-file-document-outline" size="20" />
            <div>
              <h4>Suggested file paths</h4>
              <p>Optional locations shown with the guide.</p>
            </div>
          </div>
          <v-textarea
            v-model="filePathsText"
            aria-label="Suggested file paths"
            placeholder="~/.config/tool/config.json"
            variant="outlined"
            rows="3"
            hide-details
          />
        </div>

        <div class="setup-guide-content__card">
          <div class="setup-guide-content__card-heading">
            <v-icon icon="mdi-code-tags" size="20" />
            <div>
              <h4>Available variables</h4>
              <p>Insert these placeholders in the template.</p>
            </div>
          </div>
          <div class="setup-guide-content__variables">
            <v-chip
              v-for="variable in variables"
              :key="variable"
              size="small"
              variant="tonal"
            >
              {{ variable }}
            </v-chip>
          </div>
          <p class="setup-guide-content__loop">
            Model loop: <code>{{ modelLoopHelp }}</code>
          </p>
        </div>
      </div>
    </div>

    <div v-show="activeTab === 'preview'" class="setup-guide-content__panel">
      <div class="setup-guide-content__preview">
        <AdminSetupGuidePreview :guide="props.guide" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.setup-guide-content__heading {
  margin-bottom: 20px;
}

.setup-guide-content__heading h3,
.setup-guide-content__heading p,
.setup-guide-content__loop {
  margin: 0;
}

.setup-guide-content__heading p,
.setup-guide-content__loop {
  margin-top: 4px;
  color: rgb(var(--app-shell-text-secondary));
}

.setup-guide-content__variables {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.setup-guide-content__loop {
  margin-top: 16px;
  font-size: 0.875rem;
}

.setup-guide-content__tabs {
  height: auto !important;
  min-height: 58px;
  margin-bottom: 20px;
  padding: 6px;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: 12px;
  background: rgba(var(--app-shell-surface-muted), 0.5);
  overflow: visible;
}

.setup-guide-content__tabs :deep(.v-slide-group__container) {
  min-height: 44px;
  overflow: visible;
}

.setup-guide-content__tabs :deep(.v-tab) {
  min-height: 44px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: 8px;
  color: rgb(var(--app-shell-text-secondary));
  background: rgb(var(--app-shell-surface-strong));
  cursor: pointer;
  transition:
    border-color 150ms ease,
    color 150ms ease,
    background-color 150ms ease,
    box-shadow 150ms ease;
}

.setup-guide-content__tabs :deep(.v-tab + .v-tab) {
  margin-left: 8px;
}

.setup-guide-content__tabs :deep(.v-tab:hover) {
  border-color: rgba(var(--v-theme-primary), 0.42);
  color: rgb(var(--v-theme-primary));
  background: rgba(var(--v-theme-primary), 0.06);
}

.setup-guide-content__tabs :deep(.v-tab--selected) {
  border-color: rgba(var(--v-theme-primary), 0.6);
  color: rgb(var(--v-theme-primary));
  background: rgba(var(--v-theme-primary), 0.14);
  box-shadow: 0 2px 8px rgba(var(--v-theme-primary), 0.12);
  font-weight: 700;
}

.setup-guide-content__tabs :deep(.v-tab__slider) {
  display: none;
}

.setup-guide-content__panel {
  min-height: 430px;
}

.setup-guide-content__editor {
  margin-bottom: 24px;
}

.setup-guide-content__textarea :deep(textarea) {
  min-height: 170px;
  max-height: 520px;
  resize: vertical !important;
}

.setup-guide-content__secondary {
  display: grid;
  grid-template-columns: minmax(280px, 0.85fr) minmax(0, 1.15fr);
  gap: 20px;
}

.setup-guide-content__card {
  padding: 18px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: 14px;
  background: rgba(var(--app-shell-surface-muted), 0.42);
}

.setup-guide-content__card-heading {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 16px;
  color: rgb(var(--v-theme-primary));
}

.setup-guide-content__card-heading h4,
.setup-guide-content__card-heading p {
  margin: 0;
}

.setup-guide-content__card-heading h4 {
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.95rem;
}

.setup-guide-content__card-heading p {
  margin-top: 3px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.825rem;
}

.setup-guide-content__preview {
  min-height: 430px;
  padding: 24px;
  border: 1px solid rgba(var(--app-shell-border), 0.5);
  border-radius: 14px;
  background: rgba(var(--app-shell-surface-muted), 0.52);
}

@media (max-width: 760px) {
  .setup-guide-content__secondary {
    grid-template-columns: 1fr;
  }

  .setup-guide-content__panel,
  .setup-guide-content__preview {
    min-height: 0;
  }
}
</style>
