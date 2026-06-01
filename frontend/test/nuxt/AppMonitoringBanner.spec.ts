import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'

import AppMonitoringBanner from '../../app/components/App/AppMonitoringBanner.vue'
import type { MonitoringStatusService } from '../../app/types/monitoring'

const { useMonitoringStatusMock, services } = vi.hoisted(() => {
  const services = { value: [] as MonitoringStatusService[] }
  return {
    services,
    useMonitoringStatusMock: vi.fn(() => ({
      error: { value: null },
      loading: { value: false },
      refresh: vi.fn(),
      services,
      status: { value: 'ok' },
    })),
  }
})

mockNuxtImport('useMonitoringStatus', () => useMonitoringStatusMock)

function mountBanner() {
  return mount(AppMonitoringBanner, {
    global: {
      stubs: {
        VAlert: {
          props: ['variant'],
          template:
            '<section data-test="banner" :data-variant="variant"><slot /></section>',
        },
      },
    },
  })
}

describe('AppMonitoringBanner', () => {
  beforeEach(() => {
    services.value = []
    useMonitoringStatusMock.mockClear()
  })

  it('does not render without degraded services', () => {
    const wrapper = mountBanner()

    expect(wrapper.find('[data-test="banner"]').exists()).toBe(false)
  })

  it('renders one degraded service', () => {
    services.value = [
      {
        id: 'service-id',
        name: 'api-health',
        displayName: 'API health',
        status: 'degraded',
        lastCheckedAt: '2026-01-01T00:00:00Z',
        lastStatusCode: 500,
        lastError: 'expected HTTP 200, got 500',
        lastLatencyMs: 42,
      },
    ]

    const wrapper = mountBanner()

    expect(wrapper.get('[data-test="banner"]').text()).toContain(
      'Service perturbe: API health',
    )
    expect(wrapper.get('[data-test="banner"]').attributes('data-variant')).toBe(
      'flat',
    )
  })

  it('renders multiple degraded services compactly', () => {
    services.value = [
      service('api', 'API'),
      service('proxy', 'Proxy'),
      service('mcp', 'MCP'),
      service('docs', 'Docs'),
    ]

    const wrapper = mountBanner()

    expect(wrapper.get('[data-test="banner"]').text()).toContain(
      'Services perturbes: API, Proxy, MCP +1',
    )
  })
})

function service(name: string, displayName: string): MonitoringStatusService {
  return {
    id: name,
    name,
    displayName,
    status: 'degraded',
    lastCheckedAt: '2026-01-01T00:00:00Z',
    lastStatusCode: 500,
    lastError: 'expected HTTP 200, got 500',
    lastLatencyMs: 42,
  }
}
