<template>
  <v-app class="public-layout">
    <v-main class="public-layout-main">
      <slot />
    </v-main>
  </v-app>
</template>

<style scoped>
.public-layout {
  background:
    radial-gradient(
      circle at top,
      rgba(var(--v-theme-primary), 0.18),
      transparent 42%
    ),
    linear-gradient(180deg, rgb(245, 247, 251) 0%, rgb(232, 236, 244) 100%);
}

.public-layout-main {
  min-height: 100vh;
}
</style>
