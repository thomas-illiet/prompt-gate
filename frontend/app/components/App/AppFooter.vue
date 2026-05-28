<script setup lang="ts">
const theme = useTheme()
const isDark = computed({
  get() {
    return theme.current.value.dark
  },
  set(v) {
    theme.change(v ? 'dark' : 'light')
  },
})
</script>

<template>
  <v-footer app>
    <v-spacer />
    <div class="footer-actions">
      <v-switch
        v-model="isDark"
        aria-label="Toggle color theme"
        color=""
        hide-details
        density="compact"
        false-icon="mdi-white-balance-sunny"
        true-icon="mdi-weather-night"
        class="opacity-80"
      />
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
  padding: 0 10px;
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

:deep(.footer-actions .v-switch) {
  margin-inline-end: 6px;
}

:deep(.footer-actions .v-btn--icon .v-icon) {
  height: 1.25em;
  width: 1.25em;
}
</style>
