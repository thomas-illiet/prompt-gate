import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

import AppDrawerItem from '../../app/components/App/AppDrawerItem.vue'
import { useAuthStore } from '../../app/stores/auth'

const { useRouteMock } = vi.hoisted(() => ({
  useRouteMock: vi.fn(),
}))

mockNuxtImport('useRoute', () => useRouteMock)

const ListItemStub = defineComponent({
  inheritAttrs: false,
  props: {
    active: Boolean,
    title: String,
    to: [Object, String],
  },
  setup(props, { attrs, slots }) {
    return () =>
      h('a', { ...attrs, 'data-title': props.title }, [
        slots.prepend?.(),
        h('span', props.title),
      ])
  },
})

const ListGroupStub = defineComponent({
  setup(_, { slots }) {
    return () =>
      h('section', { class: 'v-list-group' }, [
        slots.activator?.({ props: {} }),
        h('div', { class: 'v-list-group__items' }, slots.default?.()),
      ])
  },
})

const RouteViewStub = defineComponent({
  render: () => null,
})

function mountDrawerItem(item: RouteRecordRaw) {
  return mount(AppDrawerItem, {
    props: { item },
    global: {
      stubs: {
        VIcon: true,
        VListGroup: ListGroupStub,
        VListItem: ListItemStub,
        VListSubheader: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('AppDrawerItem', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useRouteMock.mockReset()
    useRouteMock.mockReturnValue({ matched: [], path: '/outside' })

    const authStore = useAuthStore()
    authStore.user = {
      email: 'admin@example.com',
      id: 'admin-id',
      isActive: true,
      lastLoginAt: '2026-10-24T00:00:00Z',
      name: 'Admin User',
      preferredUsername: 'admin',
      role: 'admin',
      sub: 'admin-subject',
    }
  })

  it('increments the visual depth for recursively rendered groups', () => {
    const item: RouteRecordRaw = {
      children: [
        {
          children: [
            {
              component: RouteViewStub,
              meta: { icon: 'mdi-file-outline', title: 'Leaf' },
              name: 'admin-leaf',
              path: 'leaf',
            },
          ],
          component: RouteViewStub,
          meta: {
            drawerGroup: true,
            drawerSection: 'Content',
            icon: 'mdi-folder-outline',
            title: 'Section',
          },
          name: 'admin-section',
          path: 'section',
        },
      ],
      component: RouteViewStub,
      meta: {
        drawerGroup: true,
        icon: 'mdi-shield-outline',
        title: 'Administration',
      },
      name: 'admin',
      path: '/admin',
    }

    const wrapper = mountDrawerItem(item)

    expect(
      wrapper
        .findAll('.app-drawer-item')
        .map((drawerItem) => drawerItem.attributes('data-depth')),
    ).toEqual(['0', '1', '2'])
  })

  it('uses depth zero when the prop is omitted', () => {
    const wrapper = mountDrawerItem({
      component: RouteViewStub,
      meta: { icon: 'mdi-view-dashboard-outline', title: 'Dashboard' },
      name: 'dashboard',
      path: '/dashboard',
    })

    expect(wrapper.get('.app-drawer-item').attributes('data-depth')).toBe('0')
  })
})
