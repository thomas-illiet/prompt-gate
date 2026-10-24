<template>
  <v-footer app class="app-shell-footer">
    <div class="footer-signature" aria-hidden="true">
      <span class="footer-signature__beacon" />
      <span>Friendly control room</span>
    </div>
    <v-spacer />
    <div class="footer-actions">
      <v-defaults-provider
        :defaults="{ VBtn: { variant: 'text', size: 'x-small' } }"
      >
        <AppNotification />
      </v-defaults-provider>
    </div>
  </v-footer>
</template>

<style scoped>
.v-footer {
  min-height: 42px;
  padding: 0 14px;
}

.footer-signature {
  display: flex;
  align-items: center;
  gap: 8px;
  color: rgb(var(--app-shell-text-muted));
  font-family: var(--app-font-mono);
  font-size: 0.66rem;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}

.footer-signature__beacon {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: rgb(var(--v-theme-primary));
  animation: app-signal-pulse 2.4s var(--app-ease-standard) infinite;
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

:deep(.footer-actions .v-btn--icon .v-icon) {
  height: 1.25em;
  width: 1.25em;
}

@media (max-width: 600px) {
  .footer-signature {
    display: none;
  }
}
</style>
