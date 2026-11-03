import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent } from 'vue'

import AdminLayout from '../../app/layouts/admin.vue'
import AdminPage from '../../app/pages/admin.vue'


const AuthenticatedShellStub = defineComponent({
  props: { mode: String },
  template:
    '<section data-test="authenticated-shell" :data-mode="mode"><slot /></section>',
})

describe('AdminLayout', () => {
  it('renders page content in the admin shell', () => {
    const wrapper = mount(AdminLayout, {
      slots: { default: '<div data-test="page-content">Page content</div>' },
      global: {
        stubs: { AppAuthenticatedShell: AuthenticatedShellStub },
      },
    })

    expect(
      wrapper.get('[data-test="authenticated-shell"]').attributes('data-mode'),
    ).toBe('admin')
    expect(wrapper.get('[data-test="page-content"]').text()).toBe(
      'Page content',
    )
  })

  it('makes the admin parent use the dedicated layout and dashboard landing', () => {
    mount(AdminPage, {
      global: { stubs: { NuxtPage: true } },
    })

    expect(useRouter().getRoutes().find(route => route.path === '/admin')?.meta).toMatchObject({
      layout: 'admin',
      title: 'Administration',
      requiredRoles: ['admin'],
    })
    expect(useRouter().getRoutes().find(route => route.path === '/admin')?.redirect).toBe('/admin/dashboard')
  })
})
