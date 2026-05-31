import type { AppRole } from '~/types/auth'

declare module '#app' {
  interface PageMeta {
    auth?: boolean
    icon?: string
    title?: string
    subtitle?: string
    drawerIndex?: number
    drawerGroup?: boolean
    allowBlocked?: boolean
    requiredRoles?: AppRole[]
  }
}

declare module 'vue-router' {
  interface RouteMeta {
    auth?: boolean
    icon?: string
    title?: string
    subtitle?: string
    drawerIndex?: number
    drawerGroup?: boolean
    allowBlocked?: boolean
    requiredRoles?: AppRole[]
  }
}

export {}
