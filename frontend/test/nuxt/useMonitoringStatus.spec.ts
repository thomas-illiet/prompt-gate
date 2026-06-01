import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import { useMonitoringStatus } from '../../app/composables/useMonitoringStatus'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    useApiFetchMock: vi.fn(() => apiFetch),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function mountProbe() {
  const holder = {} as { state: ReturnType<typeof useMonitoringStatus> }
  const Probe = defineComponent({
    setup() {
      holder.state = useMonitoringStatus({ pollMs: 50 })
      return () => null
    },
  })

  const wrapper = mount(Probe)
  return { state: holder.state, wrapper }
}

describe('useMonitoringStatus', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads status on mount and polls periodically', async () => {
    apiFetch
      .mockResolvedValueOnce({
        status: 'degraded',
        services: [
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
        ],
      })
      .mockResolvedValueOnce({ status: 'ok', services: [] })

    const { state, wrapper } = mountProbe()
    await flushPromises()

    expect(state.loading.value).toBe(false)
    expect(apiFetch).toHaveBeenNthCalledWith(1, '/api/v1/monitoring/status')
    expect(state.status.value).toBe('degraded')
    expect(state.services.value).toHaveLength(1)

    await vi.advanceTimersByTimeAsync(50)
    await flushPromises()

    expect(apiFetch).toHaveBeenCalledTimes(2)
    expect(state.status.value).toBe('ok')
    expect(state.services.value).toEqual([])
    wrapper.unmount()
  })

  it('keeps existing state and stores an error when status loading fails', async () => {
    apiFetch.mockRejectedValueOnce(new Error('network unavailable'))

    const { state, wrapper } = mountProbe()
    await flushPromises()

    expect(state.loading.value).toBe(false)

    expect(state.status.value).toBe('ok')
    expect(state.services.value).toEqual([])
    expect(state.error.value).toBe('network unavailable')
    wrapper.unmount()
  })
})
