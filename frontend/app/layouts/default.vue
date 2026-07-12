<template>
  <v-app>
    <AppDrawer />
    <AppBar />
    <v-main class="default-layout-main">
      <AppMonitoringBanner />
      <slot />
    </v-main>
    <AppFooter />
  </v-app>
</template>

<style scoped>
.default-layout-main {
  padding-top: 0;
  padding-bottom: 0;
  margin-top: 64px;
  margin-bottom: 32px;
  min-height: calc(100dvh - 64px - 32px);
  transition-property: padding;
}
</style>
