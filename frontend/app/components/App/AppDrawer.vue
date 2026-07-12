<script setup lang="ts">
import { storeToRefs } from 'pinia'
import type { RouteRecordRaw } from 'vue-router'

import { hasRequiredRole, isBlockedUser } from '~/utils/auth'

const router = useRouter()
const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
const drawerState = useState('drawer', () => true)

const { mobile, lgAndUp, width } = useDisplay()
const drawer = computed({
  get() {
    return drawerState.value || !mobile.value
  },
  set(val: boolean) {
    drawerState.value = val
  },
})
const rail = computed(() => !drawerState.value && !mobile.value)
// isImmediateChildRoute catches Nuxt child pages even when getRoutes exposes a flat record list.
function isImmediateChildRoute(
  parent: RouteRecordRaw,
  candidate: RouteRecordRaw,
) {
  if (parent.path === '/' || candidate.path === parent.path) {
    return false
  }

  const childPrefix = `${parent.path}/`
  if (!candidate.path.startsWith(childPrefix)) {
    return false
  }

  return !candidate.path.slice(childPrefix.length).includes('/')
}

// withDrawerChildren merges explicit route children with flat child records for drawer groups.
function withDrawerChildren(
  route: RouteRecordRaw,
  allRoutes: RouteRecordRaw[],
): RouteRecordRaw {
  const children = [...(route.children ?? [])]
  const seenChildren = new Set(
    children.map((child) => String(child.name ?? child.path)),
  )

  for (const candidate of allRoutes) {
    const key = String(candidate.name ?? candidate.path)
    if (!seenChildren.has(key) && isImmediateChildRoute(route, candidate)) {
      children.push(candidate)
      seenChildren.add(key)
    }
  }

  return { ...route, children }
}

const routes = computed(() => {
  if (isBlockedUser(user.value)) {
    return []
  }

  const allRoutes = router.getRoutes() as RouteRecordRaw[]

  return allRoutes
    .filter((route) => route.path.lastIndexOf('/') === 0)
    .filter((route) => route.meta?.icon && route.meta?.auth !== false)
    .filter((route) => hasRequiredRole(user.value, route.meta?.requiredRoles))
    .map((route) => withDrawerChildren(route, allRoutes))
    .sort((a, b) => (a.meta?.drawerIndex ?? 99) - (b.meta?.drawerIndex ?? 98))
})

drawerState.value = lgAndUp.value && width.value !== 1280
</script>

<template>
  <v-navigation-drawer
    v-model="drawer"
    :expand-on-hover="rail"
    :rail="rail"
    floating
    class="app-shell-drawer"
  >
    <template #prepend>
      <v-list>
        <v-list-item class="pa-1 drawer-brand">
          <template #prepend>
            <v-icon
              icon="custom:promptgate-mark"
              size="x-large"
              class="drawer-header-icon"
              color="primary"
            />
          </template>
          <v-list-item-title
            class="text-headline-small font-weight-bold"
            style="line-height: 2rem"
          >
            Prompt<span class="text-primary">Gate</span>
          </v-list-item-title>
        </v-list-item>
      </v-list>
    </template>
    <v-list nav density="compact">
      <AppDrawerItem v-for="route in routes" :key="route.name" :item="route" />
    </v-list>
  </v-navigation-drawer>
</template>

<style>
.v-navigation-drawer.app-shell-drawer {
  --app-drawer-rail-item-inset: 6px;

  transition-property:
    box-shadow, transform, visibility, width, height, left, right, top, bottom,
    border-radius;
  overflow: hidden;
  &.v-navigation-drawer--rail {
    border-top-right-radius: 0px;
    border-bottom-right-radius: 0px;
    &.v-navigation-drawer--is-hovering {
      border-top-right-radius: 15px;
      border-bottom-right-radius: 15px;
      box-shadow:
        0px 1px 2px 0px rgb(0 0 0 / 30%),
        0px 1px 3px 1px rgb(0 0 0 / 15%);
    }
    &:not(.v-navigation-drawer--is-hovering) {
      .drawer-header-icon {
        height: 1em;
        width: 1em;
        margin-right: 0;
      }
      .v-list {
        padding-inline: 0;
      }
      .v-navigation-drawer__content {
        scrollbar-gutter: auto;
      }
      .v-list-item {
        width: calc(100% - (var(--app-drawer-rail-item-inset) * 2));
        min-width: 0;
        margin-inline: auto;
        padding-inline: 0;
        grid-template-areas: 'prepend';
        grid-template-columns: minmax(0, 1fr);
        justify-items: center;
      }
      .v-list-item__prepend {
        width: 100%;
        min-width: 0;
        justify-content: center;
      }
      .v-list-item__spacer,
      .v-list-item__prepend > .v-badge ~ .v-list-item__spacer,
      .v-list-item__prepend > .v-icon ~ .v-list-item__spacer,
      .v-list-item__prepend > .v-tooltip ~ .v-list-item__spacer,
      .v-list-item__prepend > .v-avatar ~ .v-list-item__spacer {
        display: none;
        width: 0;
      }
      .v-list-item__content,
      .v-list-item__append {
        display: none;
      }
      .v-list-group__items .v-list-item {
        margin-inline: auto;
      }
    }
  }
  .v-navigation-drawer__content {
    overflow-y: auto;
    @supports (scrollbar-gutter: stable) {
      scrollbar-gutter: stable;
    }
  }
  .drawer-header-icon {
    opacity: 1;
    height: 1.2em;
    width: 1.2em;
    transition: all 0.2s;
    margin-right: -4px;
  }
  .drawer-brand {
    pointer-events: none;
    cursor: default;
    .v-list-item__overlay {
      display: none;
    }
  }
  .v-list-group__items {
    --v-list-indent: 28px;
  }
  .v-list-item {
    transition: all 0.2s;
  }
}
</style>
