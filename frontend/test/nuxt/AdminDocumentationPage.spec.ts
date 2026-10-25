import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import DocumentationPage from '../../app/pages/admin/documentation.vue'

const { definePageMetaMock } = vi.hoisted(() => ({
  definePageMetaMock: vi.fn(),
}))

mockNuxtImport('definePageMeta', () => definePageMetaMock)

describe('Admin documentation page', () => {
  it('is admin-only and composes the current-session Swagger surface', () => {
    const wrapper = mount(DocumentationPage, {
      global: {
        stubs: {
          AdminDocumentationSwaggerDocumentation: {
            template: '<div data-test="swagger-documentation" />',
          },
          AppPageHeader: {
            props: ['kicker', 'title', 'copy', 'statLabel', 'statValue'],
            template:
              '<header>{{ kicker }} {{ title }} {{ copy }} {{ statLabel }} {{ statValue }}</header>',
          },
          ClientOnly: { template: '<div><slot /></div>' },
          VCol: { template: '<div><slot /></div>' },
          VContainer: { template: '<main><slot /></main>' },
          VRow: { template: '<div><slot /></div>' },
        },
      },
    })

    expect(definePageMetaMock).toHaveBeenCalledWith({
      requiredRoles: ['admin'],
      title: 'API documentation',
      icon: 'mdi-api',
      drawerIndex: 11,
      drawerSection: 'Developer tools',
    })
    expect(wrapper.text()).toContain('Developer tools')
    expect(wrapper.text()).toContain('Authentication Current session')
    expect(wrapper.find('[data-test="swagger-documentation"]').exists()).toBe(
      true,
    )
  })
})
