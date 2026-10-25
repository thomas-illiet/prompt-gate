<script setup lang="ts">
import { computed } from 'vue'
import {
  syntaxLanguageLabel,
  tokenizeSyntaxCode,
} from '~/utils/syntax-highlighting'

const props = withDefaults(
  defineProps<{
    code: string
    language?: string
  }>(),
  { language: 'plaintext' },
)

const codeLines = computed(() => tokenizeSyntaxCode(props.code, props.language))
const languageLabel = computed(() => syntaxLanguageLabel(props.language))
</script>

<template>
  <div
    class="app-syntax-code-block"
    :data-language="props.language"
    data-test="syntax-code-block"
  >
    <div class="app-syntax-code-block__topbar">
      <div class="app-syntax-code-block__dots" aria-hidden="true">
        <span />
        <span />
        <span />
      </div>
      <span class="app-syntax-code-block__label">{{ languageLabel }}</span>
    </div>

    <pre><code><span
      v-for="(line, lineIndex) in codeLines"
      :key="lineIndex"
      class="app-syntax-code-block__line"
      data-test="syntax-code-line"
    ><span class="app-syntax-code-block__line-number">{{ String(lineIndex + 1).padStart(2, '0') }}</span><span class="app-syntax-code-block__line-code"><span
      v-for="(token, tokenIndex) in line"
      :key="`${lineIndex}-${tokenIndex}`"
      :class="token.type ? `app-syntax-code-block__token--${token.type}` : undefined"
    >{{ token.text }}</span><span v-if="line.length === 0">{{ ' ' }}</span></span></span></code></pre>
  </div>
</template>

<style scoped>
.app-syntax-code-block {
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.55);
  border-radius: var(--app-card-radius);
  background: rgb(var(--app-shell-surface-muted));
}

.app-syntax-code-block__topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 42px;
  padding: 0 14px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.5);
  background: rgba(var(--app-shell-surface), 0.86);
}

.app-syntax-code-block__dots {
  display: flex;
  gap: 6px;
}

.app-syntax-code-block__dots span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(var(--app-shell-text-muted), 0.42);
}

.app-syntax-code-block__dots span:nth-child(1) {
  background: rgb(var(--v-theme-error));
}

.app-syntax-code-block__dots span:nth-child(2) {
  background: rgb(var(--v-theme-warning));
}

.app-syntax-code-block__dots span:nth-child(3) {
  background: rgb(var(--v-theme-success));
}

.app-syntax-code-block__label {
  color: rgb(var(--app-shell-text-muted));
  font-family: var(--app-font-mono);
  font-size: 0.78rem;
  font-weight: 700;
}

.app-syntax-code-block pre {
  max-height: 460px;
  margin: 0;
  padding: 14px 0;
  overflow: auto;
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.8125rem;
  line-height: 1.7;
  tab-size: 2;
}

.app-syntax-code-block code {
  display: grid;
  font-family: var(--app-font-mono);
}

.app-syntax-code-block__line {
  display: grid;
  grid-template-columns: 54px minmax(0, 1fr);
  min-width: max-content;
  padding-right: 18px;
}

.app-syntax-code-block__line:hover {
  background: rgba(var(--v-theme-primary), 0.08);
}

.app-syntax-code-block__line-number {
  padding: 0 14px;
  color: rgb(var(--app-shell-text-muted));
  opacity: 0.62;
  text-align: right;
  user-select: none;
}

.app-syntax-code-block__line-code {
  white-space: pre;
}

.app-syntax-code-block__token--kwd {
  color: color-mix(
    in srgb,
    rgb(var(--app-accent-coral)) 64%,
    rgb(var(--app-shell-text-primary))
  );
  font-weight: 700;
}

.app-syntax-code-block__token--oper,
.app-syntax-code-block__token--esc,
.app-syntax-code-block__token--deleted,
.app-syntax-code-block__token--err,
.app-syntax-code-block__token--var {
  color: color-mix(
    in srgb,
    rgb(var(--app-accent-coral)) 68%,
    rgb(var(--app-shell-text-primary))
  );
}

.app-syntax-code-block__token--str,
.app-syntax-code-block__token--insert {
  color: color-mix(
    in srgb,
    rgb(var(--v-theme-primary)) 78%,
    rgb(var(--app-shell-text-primary))
  );
}

.app-syntax-code-block__token--num,
.app-syntax-code-block__token--bool,
.app-syntax-code-block__token--type,
.app-syntax-code-block__token--section {
  color: color-mix(
    in srgb,
    rgb(var(--app-accent-sky)) 68%,
    rgb(var(--app-shell-text-primary))
  );
}

.app-syntax-code-block__token--class,
.app-syntax-code-block__token--func {
  color: color-mix(
    in srgb,
    rgb(var(--app-accent-amber)) 62%,
    rgb(var(--app-shell-text-primary))
  );
  font-weight: 650;
}

.app-syntax-code-block__token--cmnt,
.app-syntax-code-block__token--bracket {
  color: rgb(var(--app-shell-text-muted));
}

.app-syntax-code-block__token--cmnt {
  font-style: italic;
}
</style>
