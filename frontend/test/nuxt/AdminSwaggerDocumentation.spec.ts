import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import SwaggerDocumentation from '../../app/components/AdminDocumentation/SwaggerDocumentation.vue'

const {
  apiFetch,
  createRoot,
  reactRootRender,
  reactRootUnmount,
  swaggerPreset,
  swaggerUI,
  useApiFetchMock,
} = vi.hoisted(() => {
  const apiFetch = vi.fn()
  const createRoot = vi.fn()
  const reactRootRender = vi.fn()
  const reactRootUnmount = vi.fn()
  const swaggerPreset = vi.fn()
  const swaggerUI = vi.fn()

  return {
    apiFetch,
    createRoot,
    reactRootRender,
    reactRootUnmount,
    swaggerPreset,
    swaggerUI,
    useApiFetchMock: vi.fn(() => apiFetch),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

vi.mock('react-dom/client', () => ({ createRoot }))
vi.mock('swagger-ui', () => {
  Object.assign(swaggerUI, { presets: { apis: swaggerPreset } })
  return { default: swaggerUI }
})
const openAPIDocument = {
  openapi: '3.1.0',
  info: { title: 'PromptGate API', version: '1.0.0' },
  paths: {},
}

function mountDocumentation() {
  return mount(SwaggerDocumentation, {
    global: {
      stubs: {
        AppSectionCard: {
          template: '<section><slot /></section>',
        },
        VAlert: {
          props: ['title', 'type'],
          template:
            '<aside :data-alert-type="type"><strong>{{ title }}</strong><slot /></aside>',
        },
        VBtn: {
          emits: ['click'],
          template:
            '<button type="button" @click="$emit(\'click\')"><slot /></button>',
        },
        VProgressLinear: { template: '<div data-test="loading" />' },
      },
    },
  })
}

describe('Admin Swagger documentation', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    createRoot.mockReset()
    reactRootRender.mockReset()
    reactRootUnmount.mockReset()
    swaggerUI.mockReset()
    useApiFetchMock.mockClear()
    document.documentElement.className = ''

    createRoot.mockImplementation((domNode: HTMLElement) => ({
      render(element: unknown) {
        reactRootRender(element)
        const renderedMarker = document.createElement('div')
        renderedMarker.dataset.test = 'swagger-rendered'
        domNode.append(renderedMarker)
      },
      unmount: reactRootUnmount,
    }))

    swaggerUI.mockImplementation(
      (options: { domNode?: HTMLElement; presets?: unknown[] }) => {
        const lifecyclePreset = options.presets?.[1] as
          | ((context: {
              getSystem: () => {
                getComponent: () => () => null
              }
            }) => {
              rootInjects: { render: (domNode: HTMLElement) => void }
            })
          | undefined
        const plugin = lifecyclePreset?.({
          getSystem: () => ({ getComponent: () => () => null }),
        })

        if (options.domNode) {
          plugin?.rootInjects.render(options.domNode)
        }

        // The real Swagger system has no destroy or unmount method.
        return {}
      },
    )
  })

  afterEach(() => {
    document.documentElement.className = ''
  })

  it('loads the protected spec and configures session-authenticated requests', async () => {
    apiFetch.mockResolvedValue(openAPIDocument)
    const wrapper = mountDocumentation()

    await vi.waitFor(() => expect(swaggerUI).toHaveBeenCalledOnce())

    expect(useApiFetchMock).toHaveBeenCalledOnce()
    expect(apiFetch).toHaveBeenCalledWith('/api/v1/admin/openapi.json')
    expect(swaggerUI).toHaveBeenCalledWith(
      expect.objectContaining({
        deepLinking: true,
        displayRequestDuration: true,
        filter: true,
        persistAuthorization: false,
        presets: [swaggerPreset, expect.any(Function)],
        queryConfigEnabled: false,
        spec: openAPIDocument,
        supportedSubmitMethods: ['get', 'post', 'put', 'patch', 'delete'],
        validatorUrl: null,
        withCredentials: true,
      }),
    )

    const swaggerOptions = swaggerUI.mock.calls[0]?.[0] as
      { domNode?: HTMLElement } | undefined
    expect(swaggerOptions?.domNode).toBeInstanceOf(HTMLElement)
    expect(createRoot).toHaveBeenCalledWith(swaggerOptions?.domNode)
    expect(reactRootRender).toHaveBeenCalledOnce()
    expect(swaggerOptions).not.toHaveProperty('requestInterceptor')
    expect(JSON.stringify(swaggerOptions)).not.toContain('X-Admin-API-Key')
    expect(wrapper.text()).not.toContain('Requests can change data')
    expect(wrapper.text()).not.toContain('Try it out sends real requests')

    wrapper.unmount()
  })

  it('retries after a loading error without restoring the removed warning', async () => {
    apiFetch
      .mockRejectedValueOnce(new Error('OpenAPI is temporarily unavailable'))
      .mockResolvedValueOnce(openAPIDocument)
    const wrapper = mountDocumentation()

    await vi.waitFor(() =>
      expect(wrapper.text()).toContain('OpenAPI is temporarily unavailable'),
    )
    expect(wrapper.text()).not.toContain('Requests can change data')
    expect(swaggerUI).not.toHaveBeenCalled()

    await wrapper.get('button').trigger('click')
    await vi.waitFor(() => expect(swaggerUI).toHaveBeenCalledOnce())

    expect(apiFetch).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('OpenAPI is temporarily unavailable')
    expect(wrapper.text()).not.toContain('Requests can change data')

    wrapper.unmount()
  })

  it('unmounts its React root without changing global document classes', async () => {
    document.documentElement.className = 'existing-root-class'
    apiFetch.mockResolvedValue(openAPIDocument)
    const wrapper = mountDocumentation()

    await vi.waitFor(() => expect(swaggerUI).toHaveBeenCalledOnce())

    const swaggerRoot = wrapper.get('.swagger-documentation')
    expect(document.documentElement.className).toBe('existing-root-class')
    expect(swaggerRoot.find('[data-test="swagger-rendered"]').exists()).toBe(
      true,
    )

    const swaggerElement = swaggerRoot.element
    wrapper.unmount()

    expect(reactRootUnmount).toHaveBeenCalledOnce()
    expect(swaggerElement.childElementCount).toBe(0)
    expect(document.documentElement.className).toBe('existing-root-class')
  })
})
