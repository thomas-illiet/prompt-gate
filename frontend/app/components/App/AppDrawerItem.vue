<script setup lang="ts">
import { storeToRefs } from 'pinia'
import type { RouteLocationRaw, RouteRecordRaw } from 'vue-router'

import { hasRequiredRole } from '~/utils/auth'

const { depth = 0, item } = defineProps<{
  depth?: number
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
const visualDepth = computed(() => Math.min(depth, 2))
const to = computed<RouteLocationRaw>(() => {
  if (displayItem.value.name) {
    return { name: displayItem.value.name } as RouteLocationRaw
  }

  if (item.name) {
    return { name: item.name } as RouteLocationRaw
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
    :aria-label="String(title ?? '')"
    active-class="text-primary"
    class="app-drawer-item"
    :data-depth="visualDepth"
    :title="title"
  >
    <template v-if="icon" #prepend>
      <span class="app-drawer-item__icon" aria-hidden="true">
        <v-icon :icon="icon" size="20" />
      </span>
    </template>
  </v-list-item>
  <v-list-group v-else color="primary">
    <template #activator="{ props: vProps }">
      <v-list-item
        :aria-label="String(title ?? '')"
        :title="title"
        v-bind="vProps"
        :active="isActive"
        class="app-drawer-item app-drawer-item--group"
        :data-depth="visualDepth"
      >
        <template v-if="icon" #prepend>
          <span class="app-drawer-item__icon" aria-hidden="true">
            <v-icon :icon="icon" size="20" />
          </span>
        </template>
      </v-list-item>
    </template>
    <template v-for="section in drawerSections" :key="section.title">
      <v-list-subheader v-if="section.title" class="app-drawer-section">
        {{ section.title }}
      </v-list-subheader>
      <AppDrawerItem
        v-for="child in section.items"
        :key="child.name ?? child.path"
        :depth="depth + 1"
        :item="child"
      />
    </template>
  </v-list-group>
</template>

<style scoped>
.app-drawer-item {
  min-height: 48px;
  margin-block: 3px;
  color: rgb(var(--app-shell-text-secondary));
  font-size: 0.87rem;
  font-weight: 700;
}

.app-drawer-item[data-depth='1'] {
  --v-list-prepend-gap: 12px;

  width: calc(100% - 24px);
  min-height: 44px;
  margin-inline: 12px;
  padding-inline: 10px;
}

.app-drawer-item[data-depth='2'] {
  --v-list-prepend-gap: 10px;

  width: calc(100% - 44px);
  min-height: 44px;
  margin-inline: 22px;
  padding-inline: 8px;
}

.app-drawer-item__icon {
  display: inline-flex;
  width: 34px;
  height: 34px;
  justify-content: center;
  align-items: center;
  transform: rotate(-1deg);
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: 12px;
  background: rgba(var(--app-shell-surface-strong), 0.68);
  color: rgb(var(--app-shell-text-muted));
  box-shadow: 2px 3px 0 rgba(var(--app-accent-butter), 0.08);
  transition:
    transform var(--app-motion-fast) var(--app-ease-out),
    border-color var(--app-motion-fast) var(--app-ease-standard),
    background-color var(--app-motion-fast) var(--app-ease-standard),
    color var(--app-motion-fast) var(--app-ease-standard);
}

.app-drawer-item[data-depth='1'] .app-drawer-item__icon {
  width: 30px;
  height: 30px;
  border-radius: 10px;
}

.app-drawer-item[data-depth='2'] .app-drawer-item__icon {
  width: 28px;
  height: 28px;
  border-radius: 9px;
}

:deep(.app-drawer-item[data-depth='1'] .v-list-item-title) {
  font-size: 0.82rem;
  font-weight: 680;
}

:deep(.app-drawer-item[data-depth='2'] .v-list-item-title) {
  font-size: 0.78rem;
  font-weight: 660;
}

.app-drawer-item.v-list-item--active .app-drawer-item__icon {
  transform: rotate(-4deg);
  border-color: rgba(var(--v-theme-primary), 0.24);
  background: rgb(var(--v-theme-primary));
  color: rgb(var(--v-theme-on-primary));
  box-shadow: 3px 4px 0 rgba(var(--app-accent-coral), 0.14);
}

.app-drawer-item:not(.v-list-item--active):hover .app-drawer-item__icon {
  transform: translateY(-1px) rotate(-2deg);
  border-color: rgba(var(--app-accent-butter), 0.52);
  color: rgb(var(--v-theme-primary));
}

:deep(.app-drawer-item--group .v-list-item__append) {
  color: rgb(var(--app-shell-text-muted));
}

.app-drawer-section {
  display: flex;
  min-height: 32px;
  align-items: center;
  gap: 9px;
  padding-inline-start: 28px;
  color: rgb(var(--app-shell-text-muted));
  font-size: 0.68rem;
  font-weight: 750;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.app-drawer-section::after {
  height: 1px;
  flex: 1 1 auto;
  background: repeating-linear-gradient(
    90deg,
    rgba(var(--app-shell-border), 0.62) 0 4px,
    transparent 4px 8px
  );
  content: '';
}
</style>
