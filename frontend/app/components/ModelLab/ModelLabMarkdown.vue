<script setup lang="ts">
import { renderModelLabMarkdown } from '~/utils/model-lab-markdown'

const props = defineProps<{
  content: string
}>()

const rendered = computed(() => renderModelLabMarkdown(props.content))
</script>

<template>
  <!-- The renderer sanitizes provider output with an explicit allowlist. -->
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="app-markdown model-lab-markdown" v-html="rendered" />
</template>

<style scoped>
.model-lab-markdown {
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
