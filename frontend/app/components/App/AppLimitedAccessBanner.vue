<script setup lang="ts">
const props = defineProps<{
  contactEmail?: string | null
}>()

const contactEmail = computed(() => props.contactEmail?.trim() || null)
const contactHref = computed(() => {
  if (!contactEmail.value) return undefined

  const separatorIndex = contactEmail.value.lastIndexOf('@')
  if (separatorIndex < 1) return undefined

  const localPart = encodeURIComponent(
    contactEmail.value.slice(0, separatorIndex),
  )
  const domain = contactEmail.value.slice(separatorIndex + 1)
  return `mailto:${localPart}@${domain}`
})
</script>

<template>
  <v-alert
    class="app-limited-access-banner"
    data-test="limited-access-banner"
    density="compact"
    icon="mdi-information-outline"
    role="status"
    type="info"
    variant="tonal"
    aria-live="polite"
  >
    <strong>Limited access.</strong>
    Your account can only use a restricted set of PromptGate features.
    <template v-if="contactEmail">
      Contact the site administrator at
      <a
        class="app-limited-access-banner__contact"
        :href="contactHref"
      >
        {{ contactEmail }}
      </a>
    </template>
    <template v-else>Contact a site administrator</template>
    to request additional permissions.
  </v-alert>
</template>

<style scoped>
.app-limited-access-banner {
  position: sticky;
  z-index: 2;
  top: 64px;
  border-block: 1px solid rgba(var(--v-theme-info), 0.3);
  border-radius: 0;
  background:
    linear-gradient(
      90deg,
      rgba(var(--v-theme-info), 0.13),
      rgba(var(--app-shell-surface-strong), 0.96) 48%,
      rgba(var(--app-accent-butter), 0.1)
    ),
    rgb(var(--app-shell-surface-strong)) !important;
  box-shadow: 0 10px 22px -20px rgba(var(--app-shell-shadow), 0.7);
  color: rgb(var(--app-shell-text-primary));
}

.app-limited-access-banner :deep(.v-alert__content) {
  line-height: 1.45;
}

.app-limited-access-banner strong {
  font-family: var(--app-font-display);
  font-weight: 800;
}

.app-limited-access-banner__contact {
  color: inherit;
  font-weight: 700;
}

@media (max-width: 599px) {
  .app-limited-access-banner {
    position: static;
  }
}
</style>
