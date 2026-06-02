import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminMonitoringServiceDetailsDialog from '../../app/components/AdminMonitoring/AdminMonitoringServiceDetailsDialog.vue'
import type { MonitoringService } from '../../app/types/monitoring'

const degradedService: MonitoringService = {
  id: 'service-id',
  name: 'api-health',
  displayName: 'API health',
  url: 'https://api.example.com/health',
  expectedStatusCode: 204,
  intervalSeconds: 60,
  enabled: true,
  status: 'degraded',
  lastCheckedAt: '2026-01-01T00:00:00Z',
  lastStatusCode: 500,
  lastError: 'expected HTTP 204, got 500',
  lastLatencyMs: 42,
  consecutiveFailures: 3,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountDialog(service: MonitoringService | null = degradedService) {
  return mount(AdminMonitoringServiceDetailsDialog, {
    props: {
      loading: false,
      modelValue: true,
      service,
    },
    global: {
      stubs: {
        AppDialogCard: {
          props: [
            'icon',
            'iconColor',
            'loading',
            'maxWidth',
            'modelValue',
            'subtitle',
            'title',
          ],
          template:
            '<section v-if="modelValue" data-test="dialog" :data-title="title" :data-subtitle="subtitle" :data-icon-color="iconColor"><slot /><slot name="actions" /></section>',
        },
        AppDialogCloseButton: {
          template: '<button>Close</button>',
        },
        VChip: {
          props: ['color'],
          template:
            '<span data-test="status-chip" :data-color="color"><slot /></span>',
        },
        VSpacer: { template: '<span />' },
      },
    },
  })
}

describe('AdminMonitoringServiceDetailsDialog', () => {
  it('renders degraded check details without duplicating the error message', () => {
    const wrapper = mountDialog()

    expect(wrapper.get('[data-test="dialog"]').attributes('data-title')).toBe(
      'API health details',
    )
    expect(
      wrapper.get('[data-test="dialog"]').attributes('data-subtitle'),
    ).toBe('https://api.example.com/health')
    expect(wrapper.get('[data-test="status-chip"]').text()).toBe('Degraded')
    expect(wrapper.get('[data-test="status-chip"]').attributes('data-color')).toBe(
      'warning',
    )

    const text = wrapper.text()
    const errorMatches = text.match(/expected HTTP 204, got 500/g) ?? []

    expect(text).toContain('expected HTTP 204, got 500')
    expect(errorMatches).toHaveLength(1)
    expect(text).toContain('Expected HTTP')
    expect(text).toContain('204')
    expect(text).toContain('Received HTTP')
    expect(text).toContain('500')
    expect(text).toContain('42 ms')
    expect(text).toContain('3 failures')
  })
})
