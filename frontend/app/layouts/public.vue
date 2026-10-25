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
      circle at 10% 10%,
      rgba(var(--app-accent-butter), 0.3),
      transparent 30%
    ),
    radial-gradient(
      circle at 94% 84%,
      rgba(var(--app-accent-coral), 0.17),
      transparent 30%
    ),
    linear-gradient(
      145deg,
      rgb(var(--app-shell-surface-muted)) 0%,
      rgb(var(--v-theme-background)) 50%,
      rgba(var(--v-theme-primary), 0.1) 100%
    );
}

.public-layout::before {
  position: absolute;
  z-index: -1;
  inset: 0;
  background-image: radial-gradient(
    rgba(var(--v-theme-primary), 0.16) 1.3px,
    transparent 1.3px
  );
  background-size: 28px 28px;
  content: '';
  -webkit-mask-image: linear-gradient(
    to bottom,
    rgba(0, 0, 0, 0.42),
    transparent 76%
  );
  mask-image: linear-gradient(to bottom, rgba(0, 0, 0, 0.42), transparent 76%);
  pointer-events: none;
}

.public-layout::after {
  position: absolute;
  z-index: -1;
  top: 8%;
  right: -80px;
  width: 210px;
  height: 210px;
  transform: rotate(12deg);
  border: 2px solid rgba(var(--v-theme-primary), 0.09);
  border-radius: 44% 56% 62% 38%;
  box-shadow:
    -42px 32px 0 -28px rgba(var(--app-accent-coral), 0.16),
    -84px 112px 0 -52px rgba(var(--app-accent-butter), 0.22);
  content: '';
  pointer-events: none;
}

.public-layout-main {
  position: relative;
  min-height: 100vh;
}
</style>
