import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AdminDashboardPage from '../../app/pages/admin/dashboard.vue'

const { definePageMetaMock } = vi.hoisted(() => ({
  definePageMetaMock: vi.fn(),
}))

mockNuxtImport('definePageMeta', () => definePageMetaMock)

describe('AdminDashboardPage', () => {
  it('is admin-only and composes the cockpit with a fixed global scope', () => {
    const wrapper = mount(AdminDashboardPage, {
      global: {
        stubs: {
          DashboardCockpit: {
            props: ['scope'],
            template: '<div data-test="cockpit" :data-scope="scope" />',
          },
        },
      },
    })

    expect(definePageMetaMock).toHaveBeenCalledWith({
      requiredRoles: ['admin'],
      title: 'Global dashboard',
      icon: 'mdi-monitor-dashboard',
      drawerIndex: 0,
    })
    expect(wrapper.get('[data-test="cockpit"]').attributes('data-scope')).toBe(
      'global',
    )
  })
})
