<template>
  <v-card rounded="lg" class="help-setup-provider-loading-card">
    <v-card-text class="help-setup-provider-loading-card__body">
      <v-progress-circular
        color="primary"
        indeterminate
        :size="42"
        :width="4"
      />

      <div class="help-setup-provider-loading-card__copy">
        <h2>Loading providers</h2>
        <p>Preparing available endpoints and models.</p>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.help-setup-provider-loading-card {
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  background: rgb(var(--app-shell-surface));
  box-shadow: var(--app-card-shadow-soft);
}

.help-setup-provider-loading-card__body {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 112px;
  padding: 24px;
}

.help-setup-provider-loading-card__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.help-setup-provider-loading-card__copy h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 800;
}

.help-setup-provider-loading-card__copy p {
  margin: 0;
  color: rgb(var(--app-shell-text-secondary));
}
</style>
