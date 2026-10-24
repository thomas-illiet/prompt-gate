<script setup lang="ts">
const props = defineProps<{
  html: string
  loading: boolean
}>()
</script>

<template>
  <section class="admin-faq-preview" aria-label="FAQ Markdown preview">
    <header class="admin-faq-preview__header">
      <div class="admin-faq-preview__heading">
        <v-avatar color="primary" size="32" variant="tonal">
          <v-icon icon="mdi-eye-check-outline" size="18" />
        </v-avatar>
        <div>
          <strong>Live preview</strong>
          <span>Rendered exactly as users will read it</span>
        </div>
      </div>
      <v-chip
        color="primary"
        label
        size="x-small"
        variant="tonal"
        :prepend-icon="
          props.loading ? 'mdi-loading' : 'mdi-check-circle-outline'
        "
      >
        {{ props.loading ? 'Rendering' : 'Ready' }}
      </v-chip>
    </header>

    <div class="admin-faq-preview__body" :aria-busy="props.loading">
      <v-skeleton-loader v-if="props.loading" type="article" />
      <AppEmptyState
        v-else-if="!props.html"
        compact
        icon="mdi-eye-off-outline"
        title="Nothing to preview"
        text="Add Markdown content in the editor first."
      />
      <!-- eslint-disable-next-line vue/no-v-html -- backend-sanitized HTML -->
      <div v-else class="app-markdown" v-html="props.html" />
    </div>
  </section>
</template>

<style scoped>
.admin-faq-preview {
  min-height: 280px;
  overflow: hidden;
  border: 1px solid rgba(var(--app-shell-border), 0.58);
  border-radius: var(--app-card-radius);
  background:
    radial-gradient(
      circle at 94% 0%,
      rgba(var(--v-theme-primary), 0.1),
      transparent 34%
    ),
    rgba(var(--app-shell-surface), 0.92);
}

.admin-faq-preview__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(var(--app-shell-border), 0.46);
  background: rgba(var(--app-shell-surface-muted), 0.58);
}

.admin-faq-preview__heading {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.admin-faq-preview__heading > div {
  display: grid;
  min-width: 0;
  gap: 1px;
}

.admin-faq-preview__heading strong {
  color: rgb(var(--app-shell-text-primary));
  font-size: 0.88rem;
}

.admin-faq-preview__heading span {
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.76rem;
}

.admin-faq-preview__body {
  min-height: 218px;
  padding: 18px;
}

@media (max-width: 600px) {
  .admin-faq-preview__header {
    align-items: flex-start;
  }

  .admin-faq-preview__heading span {
    display: none;
  }

  .admin-faq-preview__body {
    padding: 14px;
  }
}
</style>
