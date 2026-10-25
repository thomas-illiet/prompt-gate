import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import DocumentationPage from '../../app/pages/documentation.vue'

const { definePageMetaMock } = vi.hoisted(() => ({
  definePageMetaMock: vi.fn(),
}))

mockNuxtImport('definePageMeta', () => definePageMetaMock)

describe('Documentation page', () => {
  it('is available to every authenticated role and composes the current-session Swagger surface', () => {
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
      requiredRoles: ['limited', 'user', 'admin'],
      title: 'API documentation',
    })
    expect(wrapper.text()).toContain('Developer tools')
    expect(wrapper.text()).toContain('Authentication Current session')
    expect(wrapper.find('[data-test="swagger-documentation"]').exists()).toBe(
      true,
    )
  })
})
