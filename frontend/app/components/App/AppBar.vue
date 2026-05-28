<script setup lang="ts">
const drawer = useState('drawer')
const route = useRoute()
const breadcrumbs = computed(() => {
  return route!.matched
    .filter((item) => item.meta && item.meta.title)
    .map((r) => ({
      title: r.meta.title!,
      disabled: r.path === route.path || false,
      to: r.path,
    }))
})
</script>

<template>
  <v-app-bar flat class="app-shell-bar">
    <v-app-bar-nav-icon
      aria-label="Toggle navigation"
      @click="drawer = !drawer"
    />
    <v-breadcrumbs :items="breadcrumbs" />
    <v-spacer />
    <div id="app-bar" />
    <AppUserMenu />
  </v-app-bar>
</template>
