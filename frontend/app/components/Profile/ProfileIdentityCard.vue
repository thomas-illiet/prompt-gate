<script setup lang="ts">
const props = defineProps<{
  displayEmail: string
  displayName: string
  displayUsername: string
  initials: string
  lastLoginLabel: string
  roleColor: string
  roleLabel: string
  statusColor: string
  statusLabel: string
}>()
</script>

<template>
  <v-card rounded="xl" class="profile-identity-card">
    <v-card-text class="profile-identity-card__content">
      <div class="profile-identity-card__header">
        <v-avatar size="88" class="profile-identity-card__avatar">
          <span>{{ props.initials }}</span>
        </v-avatar>

        <div class="profile-identity-card__copy">
          <h2 class="profile-identity-card__name">{{ props.displayName }}</h2>
          <p class="profile-identity-card__username">
            {{ props.displayUsername }}
          </p>
          <v-chip
            label
            variant="tonal"
            prepend-icon="mdi-email-outline"
            class="profile-identity-card__email"
          >
            {{ props.displayEmail }}
          </v-chip>
        </div>
      </div>

      <div class="profile-identity-card__summary" aria-label="Account summary">
        <div class="profile-identity-card__summary-item">
          <v-icon icon="mdi-shield-account-outline" />
          <span class="profile-identity-card__summary-label">Role</span>
          <v-chip label variant="tonal" :color="props.roleColor" size="small">
            {{ props.roleLabel }}
          </v-chip>
        </div>

        <div class="profile-identity-card__summary-item">
          <v-icon icon="mdi-check-decagram-outline" />
          <span class="profile-identity-card__summary-label">Status</span>
          <v-chip label variant="tonal" :color="props.statusColor" size="small">
            {{ props.statusLabel }}
          </v-chip>
        </div>

        <div class="profile-identity-card__summary-item">
          <v-icon icon="mdi-login-variant" />
          <span class="profile-identity-card__summary-label">Last login</span>
          <strong class="profile-identity-card__summary-value">
            {{ props.lastLoginLabel }}
          </strong>
        </div>
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
  min-height: 260px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 28px;
  padding: 28px;
}

.profile-identity-card__header {
  display: flex;
  align-items: center;
  gap: 24px;
  width: 100%;
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

.profile-identity-card__name {
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 1.65rem;
  font-weight: 800;
  line-height: 1.2;
}

.profile-identity-card__username {
  margin: 8px 0 16px;
  overflow-wrap: anywhere;
  color: rgb(var(--app-shell-text-secondary));
}

.profile-identity-card__email {
  max-width: 100%;
}

.profile-identity-card__summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  width: 100%;
}

.profile-identity-card__summary-item {
  min-width: 0;
  display: grid;
  align-content: start;
  gap: 8px;
  min-height: 104px;
  padding: 16px;
  border: 1px solid rgba(var(--app-shell-border), 0.5);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.72);
}

.profile-identity-card__summary-item .v-icon {
  color: rgb(var(--v-theme-primary));
}

.profile-identity-card__summary-label {
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.82rem;
  font-weight: 650;
}

.profile-identity-card__summary-value {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 0.95rem;
  line-height: 1.35;
}

@media (max-width: 960px) {
  .profile-identity-card__summary {
    grid-template-columns: 1fr;
  }

  .profile-identity-card__summary-item {
    grid-template-areas:
      'icon label'
      'icon value';
    grid-template-columns: 32px minmax(0, 1fr);
    align-items: center;
    min-height: 0;
    padding: 12px 14px;
    gap: 4px 10px;
  }

  .profile-identity-card__summary-item .v-icon {
    grid-area: icon;
  }

  .profile-identity-card__summary-label {
    grid-area: label;
  }

  .profile-identity-card__summary-item .v-chip,
  .profile-identity-card__summary-value {
    grid-area: value;
    justify-self: start;
  }
}

@media (max-width: 720px) {
  .profile-identity-card__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .profile-identity-card__content {
    min-height: 0;
  }
}
</style>
