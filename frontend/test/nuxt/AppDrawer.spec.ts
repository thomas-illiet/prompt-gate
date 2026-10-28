import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, shallowRef } from 'vue'
import { createVuetify } from 'vuetify'

import AppDrawer from '../../app/components/App/AppDrawer.vue'
import { useAuthStore } from '../../app/stores/auth'
import type { AppRole } from '../../app/types/auth'

const { useStateMock } = vi.hoisted(() => ({
  useStateMock: vi.fn(),
}))

mockNuxtImport('useState', () => useStateMock)

const NavigationDrawerStub = defineComponent({
  props: {
    expandOnHover: Boolean,
    modelValue: Boolean,
    rail: Boolean,
  },
  setup(props, { attrs, slots }) {
    return () =>
      h('aside', { ...attrs, 'data-rail': String(props.rail) }, [
        slots.prepend?.(),
        slots.default?.(),
        h('footer', { 'data-test': 'drawer-append' }, slots.append?.()),
      ])
  },
})

const ListItemStub = defineComponent({
  inheritAttrs: false,
  props: {
    title: String,
    to: [Object, String],
  },
  setup(props, { attrs, slots }) {
    return () =>
      h(
        'a',
        {
          ...attrs,
          'data-title': props.title,
          'data-to': typeof props.to === 'string' ? props.to : '',
        },
        [slots.prepend?.(), slots.default?.(), props.title],
      )
  },
})

const DrawerItemStub = defineComponent({
  props: { item: { type: Object, required: true } },
  setup(props) {
    return () => {
      const item = props.item as RouteRecordRaw
      return h(
        'a',
        {
          'data-name': String(item.name ?? ''),
          'data-path': item.path,
          'data-test': 'drawer-route',
        },
        String(item.meta?.title ?? ''),
      )
    }
  },
})

const TooltipStub = defineComponent({
  props: { text: String },
  setup(props, { slots }) {
    return () =>
      h('span', { 'data-tooltip': props.text }, [
        slots.activator?.({ props: {} }),
      ])
  },
})

function setUser(role: AppRole) {
  const authStore = useAuthStore()
  authStore.user = {
    email: `${role}@example.com`,
    id: `${role}-id`,
    isActive: true,
    lastLoginAt: '2026-10-25T00:00:00Z',
    name: `${role} user`,
    preferredUsername: role,
    role,
    sub: `${role}-subject`,
  }
}

function mountDrawer(mode: 'admin' | 'user') {
  const vuetify = createVuetify()

  return mount(AppDrawer, {
    props: { mode },
    global: {
      plugins: [vuetify],
      stubs: {
        AppDrawerItem: DrawerItemStub,
        AppEnvironmentBadge: {
          template: '<span data-test="environment-badge">Development</span>',
        },
        VIcon: true,
        VList: { template: '<div><slot /></div>' },
        VListItem: ListItemStub,
        VListItemSubtitle: { template: '<span><slot /></span>' },
        VListItemTitle: { template: '<strong><slot /></strong>' },
        VListSubheader: {
          template: '<div data-test="drawer-section"><slot /></div>',
        },
        VNavigationDrawer: NavigationDrawerStub,
        VTooltip: TooltipStub,
      },
    },
  })
}

describe('AppDrawer', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useStateMock.mockReset()
    useStateMock.mockReturnValue(shallowRef(true))
  })

  it('shows user routes and the administration switch for an admin', () => {
    setUser('admin')

    const wrapper = mountDrawer('user')
    const routePaths = wrapper
      .findAll('[data-test="drawer-route"]')
      .map((item) => item.attributes('data-path'))
    const contextSwitch = wrapper.get('[data-test="drawer-context-switch"]')

    expect(routePaths).toEqual([
      '/dashboard',
      '/tokens',
      '/models',
      '/help',
      '/faq',
    ])
    expect(contextSwitch.attributes('data-to')).toBe('/admin/dashboard')
    expect(contextSwitch.attributes('aria-label')).toBe('Administration')
    expect(wrapper.get('[data-tooltip]').attributes('data-tooltip')).toBe(
      'Administration',
    )
    expect(wrapper.get('aside').attributes('aria-label')).toBe(
      'Primary navigation',
    )
    expect(wrapper.get('[data-test="environment-badge"]').text()).toBe(
      'Development',
    )
    expect(wrapper.text()).not.toContain('Secure model access')
  })

  it('does not expose administration from the user menu to other roles', () => {
    setUser('user')

    const wrapper = mountDrawer('user')

    expect(wrapper.find('[data-test="drawer-context-switch"]').exists()).toBe(
      false,
    )
    expect(
      wrapper
        .findAll('[data-test="drawer-route"]')
        .map((item) => item.attributes('data-path')),
    ).toEqual(['/dashboard', '/tokens', '/models', '/help', '/faq'])
  })

  it('shows only ordered administration routes and a user-space switch', () => {
    setUser('admin')

    const wrapper = mountDrawer('admin')
    const routeNames = wrapper
      .findAll('[data-test="drawer-route"]')
      .map((item) => item.attributes('data-name'))
    const sections = wrapper
      .findAll('[data-test="drawer-section"]')
      .map((section) => section.text())
    const contextSwitch = wrapper.get('[data-test="drawer-context-switch"]')

    expect(routeNames.slice(0, 4)).toEqual([
      'admin-dashboard',
      'admin-usage',
      'admin-activity',
      'admin-quota-resets',
    ])
    expect(routeNames).toContain('admin-users')
    expect(routeNames).toContain('admin-monitoring')
    expect(routeNames).not.toContain('admin-documentation')
    expect(routeNames).not.toContain('dashboard')
    expect(sections).toEqual([
      'Dashboards',
      'Content',
      'Identities',
      'Access',
      'Infrastructure',
      'Observability',
    ])
    expect(contextSwitch.attributes('data-to')).toBe('/dashboard')
    expect(contextSwitch.attributes('aria-label')).toBe('Back to user space')
    expect(wrapper.get('aside').attributes('aria-label')).toBe(
      'Administration navigation',
    )
    expect(wrapper.get('[data-test="environment-badge"]').text()).toBe(
      'Development',
    )
    expect(wrapper.text()).not.toContain('Admin console')
    expect(wrapper.text()).not.toContain('Secure model access')
  })
})
