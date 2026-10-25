<script setup lang="ts">
import { storeToRefs } from 'pinia'
import type { RouteRecordRaw } from 'vue-router'

import { hasRequiredRole, isBlockedUser } from '~/utils/auth'

const router = useRouter()
const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
const drawerState = useState('drawer', () => true)

const { mobile, lgAndUp, width } = useVDisplay()
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
    :rail-width="72"
    :width="288"
    aria-label="Primary navigation"
    floating
    class="app-shell-drawer"
  >
    <template #prepend>
      <v-list class="drawer-brand-shell">
        <v-list-item class="drawer-brand" data-test="drawer-brand">
          <template #prepend>
            <span class="drawer-brand__mark" aria-hidden="true">
              <v-icon
                icon="custom:promptgate-mark"
                size="30"
                class="drawer-header-icon"
                color="primary"
              />
            </span>
          </template>
          <v-list-item-title class="drawer-brand__title font-weight-bold">
            Prompt<span class="text-primary">Gate</span>
          </v-list-item-title>
          <v-list-item-subtitle class="drawer-brand__subtitle">
            <span class="drawer-brand__beacon" aria-hidden="true" />
            Secure model access
          </v-list-item-subtitle>
        </v-list-item>
      </v-list>
    </template>
    <v-list nav density="compact" class="drawer-navigation">
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
  transition-duration: var(--app-motion-standard);
  transition-timing-function: var(--app-ease-standard);
  overflow: hidden;
  &.v-navigation-drawer--rail {
    border-top-right-radius: 0px;
    border-bottom-right-radius: 0px;
    &.v-navigation-drawer--is-hovering {
      border-top-right-radius: var(--app-card-radius);
      border-bottom-right-radius: var(--app-card-radius);
      box-shadow:
        0 20px 48px -30px
          rgba(var(--app-shell-shadow), var(--app-shell-shadow-strong-opacity)),
        inset -1px 0 rgba(var(--app-shell-border), 0.4);
    }
    &:not(.v-navigation-drawer--is-hovering) {
      .drawer-brand__mark {
        width: 46px;
        height: 46px;
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
      .v-list-group__items {
        display: none;
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
    filter: drop-shadow(0 8px 12px rgba(var(--v-theme-primary), 0.2));
    transition:
      width var(--app-motion-standard) var(--app-ease-out),
      height var(--app-motion-standard) var(--app-ease-out),
      transform var(--app-motion-standard) var(--app-ease-out);
    animation:
      app-enter-soft var(--app-motion-expressive) var(--app-ease-out) both,
      app-float 6s ease-in-out 1.2s infinite;
  }
  .drawer-brand-shell {
    padding: 10px 9px 6px;
  }
  .drawer-brand {
    min-height: 66px;
    padding: 8px 10px;
    border: 1px solid rgba(var(--app-shell-border), 0.55);
    border-radius: 20px !important;
    background:
      radial-gradient(
        circle at 90% 12%,
        rgba(var(--app-accent-coral), 0.1),
        transparent 34%
      ),
      rgba(var(--app-shell-surface-strong), 0.76);
    box-shadow: 0 12px 28px -24px rgba(var(--app-shell-shadow), 0.58);
    pointer-events: none;
    cursor: default;
    .v-list-item__overlay {
      display: none;
    }
  }
  .drawer-brand__mark {
    display: inline-flex;
    width: 48px;
    height: 48px;
    flex: 0 0 auto;
    justify-content: center;
    align-items: center;
    transform: rotate(-4deg);
    border: 1px solid rgba(var(--v-theme-primary), 0.16);
    border-radius: 17px;
    background:
      radial-gradient(
        circle at 28% 24%,
        rgba(var(--app-accent-butter), 0.46),
        transparent 52%
      ),
      rgba(var(--v-theme-primary), 0.08);
    box-shadow: 3px 4px 0 rgba(var(--app-accent-coral), 0.1);
    transition:
      width var(--app-motion-standard) var(--app-ease-out),
      height var(--app-motion-standard) var(--app-ease-out);
  }
  .drawer-brand__title {
    color: rgb(var(--app-shell-text-primary));
    font-size: 1.2rem;
    line-height: 1.25;
    letter-spacing: -0.035em;
  }
  .drawer-brand__subtitle {
    display: flex;
    align-items: center;
    gap: 7px;
    margin-top: -2px;
    font-family: var(--app-font-mono);
    font-size: 0.64rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .drawer-brand__beacon {
    display: inline-block;
    width: 6px;
    height: 6px;
    flex: 0 0 auto;
    border-radius: 50%;
    background: rgb(var(--v-theme-primary));
    animation: app-signal-pulse 2.4s var(--app-ease-standard) infinite;
  }
  .v-list-group__items {
    --v-list-indent: 28px;
  }
  .drawer-navigation {
    padding: 4px 9px 18px;
  }
  .v-list-item {
    transition:
      background-color var(--app-motion-fast) var(--app-ease-standard),
      border-color var(--app-motion-fast) var(--app-ease-standard),
      transform var(--app-motion-fast) var(--app-ease-out);
  }
}
</style>
