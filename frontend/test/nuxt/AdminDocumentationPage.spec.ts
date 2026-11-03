import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DocumentationPage from '../../app/pages/documentation.vue'


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

    expect(useRouter().getRoutes().find(route => route.path === '/documentation')?.meta).toMatchObject({
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
