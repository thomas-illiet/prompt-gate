import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AdminDashboardPage from '../../app/pages/admin/dashboard.vue'
import AdminActivityPage from '../../app/pages/admin/activity.vue'
import AdminQuotaResetsPage from '../../app/pages/admin/quota-resets.vue'
import AdminUsagePage from '../../app/pages/admin/usage.vue'

const { definePageMetaMock, routeQuery, useRouteMock } = vi.hoisted(() => {
  const routeQuery: Record<string, string> = {}
  return {
    definePageMetaMock: vi.fn(),
    routeQuery,
    useRouteMock: vi.fn(() => ({ query: routeQuery })),
  }
})

mockNuxtImport('definePageMeta', () => definePageMetaMock)
mockNuxtImport('useRoute', () => useRouteMock)

let routerReplace: ReturnType<typeof vi.spyOn>

describe('AdminDashboardPage', () => {
  beforeEach(() => {
    for (const key of Object.keys(routeQuery))
      Reflect.deleteProperty(routeQuery, key)
    routerReplace = vi
      .spyOn(useRouter(), 'replace')
      .mockResolvedValue(undefined)
  })

  afterEach(() => {
    routerReplace.mockRestore()
  })

  it('is the admin-only overview in the Dashboards section', () => {
    const wrapper = mount(AdminDashboardPage, {
      global: {
        stubs: {
          AdminDashboardOverview: {
            template: '<div data-test="overview" />',
          },
        },
      },
    })

    expect(definePageMetaMock).toHaveBeenCalledWith({
      requiredRoles: ['admin'],
      title: 'Overview',
      icon: 'mdi-view-dashboard-outline',
      drawerIndex: 0,
      drawerSection: 'Dashboards',
    })
    expect(wrapper.get('[data-test="overview"]')).toBeDefined()
  })

  it('replace-navigates legacy analytical queries to Usage & costs', async () => {
    Object.assign(routeQuery, {
      breakdown: 'providers',
      ignored: 'value',
      metric: 'tokens',
      window: '30d',
    })

    const wrapper = mount(AdminDashboardPage, {
      global: { stubs: { AdminDashboardOverview: true } },
    })

    await vi.waitFor(() => expect(routerReplace).toHaveBeenCalled())
    expect(routerReplace).toHaveBeenLastCalledWith({
      path: '/admin/usage',
      query: {
        breakdown: 'providers',
        metric: 'tokens',
        window: '30d',
      },
    })
    expect(
      wrapper.findComponent({ name: 'AdminDashboardOverview' }).exists(),
    ).toBe(false)
  })

  it('registers all four thin admin dashboard routes in menu order', () => {
    const routes = [
      {
        component: AdminDashboardPage,
        meta: {
          drawerIndex: 0,
          drawerSection: 'Dashboards',
          icon: 'mdi-view-dashboard-outline',
          requiredRoles: ['admin'],
          title: 'Overview',
        },
        stub: 'AdminDashboardOverview',
      },
      {
        component: AdminUsagePage,
        meta: {
          drawerIndex: 1,
          drawerSection: 'Dashboards',
          icon: 'mdi-chart-line',
          requiredRoles: ['admin'],
          title: 'Usage & costs',
        },
        stub: 'AdminUsageDashboard',
      },
      {
        component: AdminActivityPage,
        meta: {
          drawerIndex: 2,
          drawerSection: 'Dashboards',
          icon: 'mdi-account-group-outline',
          requiredRoles: ['admin'],
          title: 'Activity & adoption',
        },
        stub: 'AdminActivityDashboard',
      },
      {
        component: AdminQuotaResetsPage,
        meta: {
          drawerIndex: 3,
          drawerSection: 'Dashboards',
          icon: 'mdi-timer-refresh-outline',
          requiredRoles: ['admin'],
          title: 'Quotas & resets',
        },
        stub: 'AdminQuotaResetsDashboard',
      },
    ]

    for (const route of routes) {
      mount(route.component, {
        global: { stubs: { [route.stub]: true } },
      })
      expect(definePageMetaMock).toHaveBeenCalledWith(route.meta)
    }
  })
})
