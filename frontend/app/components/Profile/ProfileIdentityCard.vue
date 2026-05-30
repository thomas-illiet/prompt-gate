<script setup lang="ts">
const props = defineProps<{
  displayEmail: string
  displayName: string
  displayUsername: string
  initials: string
}>()
</script>

<template>
  <v-card rounded="xl" class="profile-identity-card">
    <v-card-text class="profile-identity-card__content">
      <v-avatar size="88" class="profile-identity-card__avatar">
        <span>{{ props.initials }}</span>
      </v-avatar>

      <div class="profile-identity-card__copy">
        <h1>{{ props.displayName }}</h1>
        <p>{{ props.displayUsername }}</p>
        <v-chip
          label
          variant="tonal"
          prepend-icon="mdi-email-outline"
          class="profile-identity-card__email"
        >
          {{ props.displayEmail }}
        </v-chip>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.profile-identity-card {
  height: 100%;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background:
    linear-gradient(
      90deg,
      rgba(var(--v-theme-primary), 0.08) 0,
      rgba(var(--v-theme-primary), 0) 5px
    ),
    linear-gradient(
      180deg,
      rgba(var(--app-shell-surface-strong), 0.98) 0%,
      rgba(var(--app-shell-surface), 0.98) 100%
    ),
    rgb(var(--app-shell-surface-strong));
  box-shadow: var(--app-card-shadow-soft);
}

.profile-identity-card__content {
  min-height: 246px;
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 28px;
}

.profile-identity-card__avatar {
  flex: 0 0 auto;
  color: rgb(var(--v-theme-primary));
  background: rgba(var(--v-theme-primary), 0.12);
  border: 1px solid rgba(var(--v-theme-primary), 0.24);
}

.profile-identity-card__avatar span {
  font-size: 1.55rem;
  font-weight: 800;
  letter-spacing: 0.08em;
}

.profile-identity-card__copy {
  min-width: 0;
}

.profile-identity-card__copy h1 {
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 1.65rem;
  font-weight: 800;
  line-height: 1.2;
}

.profile-identity-card__copy p {
  margin: 8px 0 16px;
  overflow-wrap: anywhere;
  color: rgb(var(--app-shell-text-secondary));
}

.profile-identity-card__email {
  max-width: 100%;
}

@media (max-width: 720px) {
  .profile-identity-card__content {
    align-items: flex-start;
    flex-direction: column;
    min-height: 0;
  }
}
</style>
