<template>
  <v-app class="public-layout">
    <v-main class="public-layout-main">
      <slot />
    </v-main>
  </v-app>
</template>

<style scoped>
.public-layout {
  position: relative;
  isolation: isolate;
  overflow-x: hidden;
  background:
    radial-gradient(
      circle at 12% 8%,
      rgba(var(--v-theme-primary), 0.2),
      transparent 34%
    ),
    radial-gradient(
      circle at 92% 88%,
      rgba(var(--app-accent-coral, 244, 124, 96), 0.13),
      transparent 32%
    ),
    linear-gradient(
      145deg,
      rgb(var(--app-shell-surface-muted)) 0%,
      rgb(var(--v-theme-background)) 52%,
      rgba(var(--v-theme-primary), 0.08) 100%
    );
}

.public-layout::before {
  position: absolute;
  z-index: -1;
  inset: 0;
  background-image:
    linear-gradient(rgba(var(--app-shell-border), 0.16) 1px, transparent 1px),
    linear-gradient(
      90deg,
      rgba(var(--app-shell-border), 0.16) 1px,
      transparent 1px
    );
  background-size: 44px 44px;
  content: '';
  -webkit-mask-image: linear-gradient(
    to bottom,
    rgba(0, 0, 0, 0.42),
    transparent 76%
  );
  mask-image: linear-gradient(to bottom, rgba(0, 0, 0, 0.42), transparent 76%);
  pointer-events: none;
}

.public-layout-main {
  position: relative;
  min-height: 100vh;
}
</style>
