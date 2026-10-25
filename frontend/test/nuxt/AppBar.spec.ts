import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, shallowRef } from 'vue'

import AppBar from '../../app/components/App/AppBar.vue'

const { useRouteMock, useStateMock } = vi.hoisted(() => ({
  useRouteMock: vi.fn(),
  useStateMock: vi.fn(),
}))

mockNuxtImport('useRoute', () => useRouteMock)
mockNuxtImport('useState', () => useStateMock)

const AppBarNavIconStub = defineComponent({
  emits: ['click'],
  template:
    '<button v-bind="$attrs" type="button" @click="$emit(\'click\')">Menu</button>',
})

const BreadcrumbsStub = defineComponent({
  props: ['items'],
  template: `
    <nav aria-label="Breadcrumbs">
      <span
        v-for="item in items"
        :key="item.to"
        data-test="breadcrumb"
        :data-disabled="String(item.disabled)"
      >
        {{ item.title }}
      </span>
    </nav>
  `,
})

describe('AppBar', () => {
  const drawer = shallowRef(true)

  beforeEach(() => {
    drawer.value = true
    useStateMock.mockReset()
    useStateMock.mockReturnValue(drawer)
    useRouteMock.mockReset()
    useRouteMock.mockReturnValue({
      matched: [
        { meta: { title: 'Administration' }, path: '/admin' },
        { meta: {}, path: '/admin/hidden' },
        { meta: { title: 'User management' }, path: '/admin/users' },
      ],
      path: '/admin/users',
    })
  })

  function mountBar(props: { contextLabel?: string } = {}) {
    return mount(AppBar, {
      props,
      global: {
        stubs: {
          AppUserMenu: { template: '<div data-test="user-menu" />' },
          VAppBar: { template: '<header><slot /></header>' },
          VAppBarNavIcon: AppBarNavIconStub,
          VBreadcrumbs: BreadcrumbsStub,
          VIcon: true,
          VSpacer: { template: '<span />' },
        },
      },
    })
  }

  it('toggles the shared drawer from an accessible navigation control', async () => {
    const wrapper = mountBar()
    const toggle = wrapper.get('[data-test="navigation-toggle"]')

    expect(toggle.attributes('aria-label')).toBe('Toggle navigation')
    expect(drawer.value).toBe(true)

    await toggle.trigger('click')

    expect(drawer.value).toBe(false)
    expect(useStateMock).toHaveBeenCalledWith('drawer')
  })

  it('shows the green-room context and marks the current breadcrumb', () => {
    const wrapper = mountBar()
    const breadcrumbs = wrapper.findAll('[data-test="breadcrumb"]')

    expect(wrapper.text()).toContain('Green room')
    expect(wrapper.text()).toContain('User management')
    expect(breadcrumbs).toHaveLength(2)
    expect(breadcrumbs[0]?.attributes('data-disabled')).toBe('false')
    expect(breadcrumbs[1]?.attributes('data-disabled')).toBe('true')
  })

  it('shows the supplied administration context', () => {
    const wrapper = mountBar({ contextLabel: 'Administration' })

    expect(wrapper.text()).toContain('Administration')
    expect(wrapper.text()).not.toContain('Green room')
  })
})
