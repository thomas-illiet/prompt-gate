<script setup lang="ts">
import { computed, type VNode } from 'vue'
import { Notify } from '~/stores/notification'
import { copyTextToClipboard } from '~/utils/clipboard'

const props = withDefaults(
  defineProps<{
    code?: string
    filePaths?: string[]
    subtitle: string
    title: string
  }>(),
  {
    code: '',
    filePaths: () => [],
  },
)

const slots = defineSlots<{
  controls?: () => VNode[]
  default?: () => VNode[]
}>()

// textFromSlotValue extracts plain text from a rendered snippet slot.
function textFromSlotValue(value: unknown): string {
  if (Array.isArray(value)) {
    return value.map(textFromSlotValue).join('')
  }

  if (typeof value === 'string' || typeof value === 'number') {
    return String(value)
  }

  if (value && typeof value === 'object') {
    const vnode = value as VNode
    if (vnode.type === 'br') {
      return '\n'
    }

    if ('children' in vnode) {
      return textFromSlotValue(vnode.children)
    }
  }

  return ''
}

// normalizeSnippetText trims snippet indentation for copy and display.
function normalizeSnippetText(value: string) {
  const lines = value.replace(/\r\n/g, '\n').split('\n')

  while (lines[0]?.trim() === '') {
    lines.shift()
  }

  while (lines.at(-1)?.trim() === '') {
    lines.pop()
  }

  const indents = lines
    .filter((line) => line.trim())
    .map((line) => line.match(/^\s*/)?.[0].length ?? 0)
  const trimSize = indents.length > 0 ? Math.min(...indents) : 0

  return lines.map((line) => line.slice(trimSize)).join('\n')
}

const slotCode = computed(() =>
  normalizeSnippetText(textFromSlotValue(slots.default?.() ?? [])),
)
const snippetCode = computed(() => props.code || slotCode.value)

// copyCode writes the snippet content to the clipboard.
async function copyCode() {
  if (!snippetCode.value || !import.meta.client) {
    return
  }

  const copied = await copyTextToClipboard(snippetCode.value)
  if (copied) {
    Notify.success('Snippet copied.')
    return
  }

  Notify.error('Unable to copy snippet.')
}

const codeLines = computed(() => snippetCode.value.split('\n'))
</script>

<template>
  <v-card rounded="lg" class="help-setup-snippet-card">
    <div class="help-setup-snippet-card__header">
      <div class="help-setup-snippet-card__heading">
        <span class="help-setup-snippet-card__eyebrow">Client guide</span>
        <v-card-title class="help-setup-snippet-card__title">
          {{ props.title }}
        </v-card-title>
        <v-card-subtitle class="help-setup-snippet-card__subtitle">
          {{ props.subtitle }}
        </v-card-subtitle>
        <div
          v-if="props.filePaths.length > 0"
          class="help-setup-snippet-card__file-paths"
        >
          <v-icon icon="mdi-file-document-outline" size="16" />
          <span class="help-setup-snippet-card__file-path-label">
            {{
              props.filePaths.length === 1
                ? 'Default file path'
                : 'Default file paths'
            }}
          </span>
          <code
            v-for="filePath in props.filePaths"
            :key="filePath"
            class="help-setup-snippet-card__file-path"
          >
            {{ filePath }}
          </code>
        </div>
      </div>

      <div class="help-setup-snippet-card__actions">
        <v-btn
          prepend-icon="mdi-content-copy"
          variant="tonal"
          color="primary"
          size="small"
          rounded="lg"
          aria-label="Copy snippet"
          @click="copyCode"
        >
          Copy
        </v-btn>
      </div>
    </div>

    <div v-if="$slots.controls" class="help-setup-snippet-card__controls">
      <slot name="controls" />
    </div>

    <v-card-text class="help-setup-snippet-card__body">
      <div class="help-setup-snippet-card__code-shell">
        <div class="help-setup-snippet-card__code-topbar">
          <div class="help-setup-snippet-card__dots" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
          <span class="help-setup-snippet-card__code-label">snippet</span>
        </div>

        <pre><code><span
          v-for="(line, index) in codeLines"
          :key="`${index}-${line}`"
          class="help-setup-snippet-card__line"
        ><span class="help-setup-snippet-card__line-number">{{ String(index + 1).padStart(2, '0') }}</span><span class="help-setup-snippet-card__line-code">{{ line || ' ' }}</span></span></code></pre>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.help-setup-snippet-card {
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background: linear-gradient(
    180deg,
    rgba(var(--app-shell-surface-strong), 1),
    rgba(var(--app-shell-surface), 1)
  );
  box-shadow: var(--app-card-shadow-soft);
  overflow: hidden;
}

.help-setup-snippet-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 22px 16px;
}

.help-setup-snippet-card__heading {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.help-setup-snippet-card__eyebrow {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.74rem;
  font-weight: 800;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.help-setup-snippet-card__title {
  padding: 0;
  font-size: 1.25rem;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1.25;
}

.help-setup-snippet-card__subtitle {
  padding: 0;
  color: rgb(var(--app-shell-text-secondary));
  opacity: 1;
  white-space: normal;
}

.help-setup-snippet-card__file-paths {
  display: flex;
  align-items: center;
  min-width: 0;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 4px;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.78rem;
}

.help-setup-snippet-card__file-path-label {
  font-weight: 700;
  text-transform: uppercase;
}

.help-setup-snippet-card__file-path {
  padding: 3px 7px;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: var(--app-chip-radius);
  background: rgba(var(--app-shell-surface-muted), 0.76);
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.78rem;
}

.help-setup-snippet-card__actions {
  flex: 0 0 auto;
}

.help-setup-snippet-card__controls {
  padding: 0 22px 16px;
}

.help-setup-snippet-card__body {
  padding: 0 18px 18px;
}

.help-setup-snippet-card__code-shell {
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: var(--app-card-radius);
  background: rgb(var(--app-shell-surface-muted));
}

.help-setup-snippet-card__code-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 42px;
  padding: 0 14px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.5);
  background: rgba(var(--app-shell-surface), 0.86);
}

.help-setup-snippet-card__dots {
  display: flex;
  gap: 6px;
}

.help-setup-snippet-card__dots span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(var(--app-shell-text-muted), 0.42);
}

.help-setup-snippet-card__dots span:nth-child(1) {
  background: rgb(var(--v-theme-error));
}

.help-setup-snippet-card__dots span:nth-child(2) {
  background: rgb(var(--v-theme-warning));
}

.help-setup-snippet-card__dots span:nth-child(3) {
  background: rgb(var(--v-theme-success));
}

.help-setup-snippet-card__code-label {
  color: rgb(var(--app-shell-text-muted));
  font-family: monospace;
  font-size: 0.78rem;
  font-weight: 700;
}

.help-setup-snippet-card__body pre {
  max-height: 460px;
  margin: 0;
  padding: 14px 0;
  overflow: auto;
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.8125rem;
  line-height: 1.7;
}

.help-setup-snippet-card__body code {
  display: grid;
  font-family: monospace;
}

.help-setup-snippet-card__line {
  display: grid;
  grid-template-columns: 54px minmax(0, 1fr);
  min-width: max-content;
  padding-right: 18px;
}

.help-setup-snippet-card__line:hover {
  background: rgba(var(--v-theme-primary), 0.08);
}

.help-setup-snippet-card__line-number {
  user-select: none;
  color: rgb(var(--app-shell-text-muted));
  opacity: 0.62;
  padding: 0 14px;
  text-align: right;
}

.help-setup-snippet-card__line-code {
  white-space: pre;
}

@media (max-width: 640px) {
  .help-setup-snippet-card__header {
    align-items: stretch;
    flex-direction: column;
  }

  .help-setup-snippet-card__actions {
    display: flex;
  }
}
</style>
