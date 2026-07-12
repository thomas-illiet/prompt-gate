<script setup lang="ts">
import { storeToRefs } from 'pinia'
import type { RouteRecordRaw } from 'vue-router'

import { hasRequiredRole } from '~/utils/auth'

const { item } = defineProps<{
  item: RouteRecordRaw
}>()

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

// sortDrawerRoutes orders navigation routes by configured display order.
function sortDrawerRoutes(routes: RouteRecordRaw[] = []): RouteRecordRaw[] {
  return [...routes].sort(
    (a, b) => (a.meta?.drawerIndex ?? 99) - (b.meta?.drawerIndex ?? 98),
  )
}

// isVisibleDrawerRoute checks whether a route should appear in the drawer.
function isVisibleDrawerRoute(route: RouteRecordRaw): boolean {
  return (
    route.meta?.auth !== false &&
    hasRequiredRole(user.value, route.meta?.requiredRoles)
  )
}

// hasDrawerMeta checks whether a route has enough metadata for drawer rendering.
function hasDrawerMeta(route: RouteRecordRaw): boolean {
  return Boolean(
    route.meta?.title || route.meta?.icon || route.meta?.drawerGroup,
  )
}

// normalizeDrawerRoutes filters and sorts drawer children recursively.
function normalizeDrawerRoutes(
  children: readonly RouteRecordRaw[] = [],
): RouteRecordRaw[] {
  return sortDrawerRoutes(
    children.filter(isVisibleDrawerRoute).flatMap((child) => {
      const normalizedChildren: RouteRecordRaw[] = normalizeDrawerRoutes(
        child.children ?? [],
      )

      if (hasDrawerMeta(child)) {
        return [{ ...child, children: normalizedChildren }]
      }

      return normalizedChildren
    }),
  )
}

const drawerChildren = computed(() =>
  normalizeDrawerRoutes(item.children ?? []),
)
const drawerSections = computed(() => {
  const sections: { items: RouteRecordRaw[]; title: string }[] = []

  for (const child of drawerChildren.value) {
    const title = child.meta?.drawerSection ?? ''
    const section = sections.find((candidate) => candidate.title === title)
    if (section) {
      section.items.push(child)
    } else {
      sections.push({ items: [child], title })
    }
  }

  return sections
})
const forceDrawerGroup = computed(
  () => item.meta?.drawerGroup && drawerChildren.value.length > 0,
)
const displayItem = computed<RouteRecordRaw>(() =>
  !forceDrawerGroup.value && drawerChildren.value.length === 1
    ? (drawerChildren.value[0] ?? item)
    : item,
)
const isItem = computed(
  () =>
    drawerChildren.value.length === 0 ||
    (!forceDrawerGroup.value && drawerChildren.value.length === 1),
)
const title = computed(() => displayItem.value.meta?.title ?? item.meta?.title)
const icon = computed(() => item.meta?.icon ?? displayItem.value.meta?.icon)
const to = computed(() => {
  if (displayItem.value.name) {
    return { name: displayItem.value.name }
  }

  if (item.name) {
    return { name: item.name }
  }

  return { path: item.path }
})
const route = useRoute()
const isActive = computed(
  () =>
    route.matched.some(
      (match) =>
        match.name === item.name || match.name === displayItem.value.name,
    ) || route.path.startsWith(item.path),
)
</script>

<template>
  <v-list-item
    v-if="isItem"
    :to="to"
    :prepend-icon="icon || undefined"
    active-class="text-primary"
    :title="title"
  />
  <v-list-group v-else :prepend-icon="icon || undefined" color="primary">
    <template #activator="{ props: vProps }">
      <v-list-item :title="title" v-bind="vProps" :active="isActive" />
    </template>
    <template v-for="section in drawerSections" :key="section.title">
      <v-list-subheader v-if="section.title" class="app-drawer-section">
        {{ section.title }}
      </v-list-subheader>
      <AppDrawerItem
        v-for="child in section.items"
        :key="child.name ?? child.path"
        :item="child"
      />
    </template>
  </v-list-group>
</template>

<style scoped>
.app-drawer-section {
  min-height: 32px;
  padding-inline-start: 28px;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.68rem;
  font-weight: 750;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
</style>
