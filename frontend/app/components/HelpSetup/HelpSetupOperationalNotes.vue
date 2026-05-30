<template>
  <AppSectionCard
    icon="mdi-clipboard-check-outline"
    title="Before running snippets"
    subtitle="Quick checks for a clean client setup."
  >
    <div class="help-setup-operational-notes">
      <div class="help-setup-operational-notes__item">
        <v-icon color="primary" icon="mdi-key-chain" size="22" />
        <div>
          <h3>Use an active virtual key</h3>
          <p>
            Create a personal or service-account key first. The raw key is shown
            once, so store it before closing the dialog.
          </p>
        </div>
      </div>

      <div class="help-setup-operational-notes__item">
        <v-icon color="primary" icon="mdi-server-network" size="22" />
        <div>
          <h3>Match the provider route</h3>
          <p>
            Snippets target the selected provider route on the PromptGate proxy.
            Keep the model name aligned with that provider.
          </p>
        </div>
      </div>

      <div class="help-setup-operational-notes__item">
        <v-icon color="primary" icon="mdi-shield-check-outline" size="22" />
        <div>
          <h3>Check firewall decisions</h3>
          <p>
            A forbidden proxy response usually means a global or scoped firewall
            rule denied the client IP.
          </p>
        </div>
      </div>
    </div>
  </AppSectionCard>
</template>

<style scoped>
.help-setup-operational-notes {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 20px 24px;
}

.help-setup-operational-notes__item {
  display: grid;
  align-content: start;
  gap: 10px;
  min-width: 0;
}

.help-setup-operational-notes__item h3 {
  margin: 0 0 4px;
  font-size: 0.98rem;
  font-weight: 700;
}

.help-setup-operational-notes__item p {
  margin: 0;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.9rem;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .help-setup-operational-notes {
    grid-template-columns: 1fr;
  }
}
</style>
