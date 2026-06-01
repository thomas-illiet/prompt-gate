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
  height: calc(100vh - 64px - 32px);
  overflow-y: scroll;
  scrollbar-gutter: stable;
  transition-property: padding;
}
</style>
