import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FetchError } from 'ofetch'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'

import {
  toAdminMonitoringErrorMessage,
  useAdminMonitoring,
} from '../../app/composables/useAdminMonitoring'
import type {
  MonitoringService,
  MonitoringServiceListResponse,
  MonitoringServicePayload,
} from '../../app/types/monitoring'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    useApiFetchMock: vi.fn(() => apiFetch),
  }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function apiError(code: string) {
  return Object.assign(Object.create(FetchError.prototype), {
    response: {
      _data: { error: code },
    },
  }) as FetchError
}

const service: MonitoringService = {
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
  consecutiveFailures: 1,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const payload: MonitoringServicePayload = {
  name: 'api-health',
  displayName: 'API health',
  url: 'https://api.example.com/health',
  expectedStatusCode: 204,
  intervalSeconds: 60,
  enabled: true,
}

function response(
  items: MonitoringService[],
  total = items.length,
): MonitoringServiceListResponse {
  return {
    items,
    page: 1,
    pageSize: 10,
    total,
  }
}

describe('useAdminMonitoring', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
    setActivePinia(createPinia())
  })

  it('loads services on creation and supports reload', async () => {
    apiFetch
      .mockResolvedValueOnce(response([service]))
      .mockResolvedValueOnce(response([]))

    const adminMonitoring = useAdminMonitoring()
    await vi.waitFor(() => expect(adminMonitoring.loading.value).toBe(false))

    expect(apiFetch).toHaveBeenNthCalledWith(
      1,
      '/api/v1/admin/monitoring/services?page=1&pageSize=10&sortBy=name&sortDir=asc',
    )
    expect(adminMonitoring.services.value).toEqual([service])
    expect(adminMonitoring.enabledServicesCount.value).toBe(1)
    expect(adminMonitoring.degradedServicesCount.value).toBe(1)

    await adminMonitoring.reload()

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/monitoring/services?page=1&pageSize=10&sortBy=name&sortDir=asc',
    )
    expect(adminMonitoring.services.value).toEqual([])
  })

  it('creates, updates, checks, loads, and deletes services through admin endpoints', async () => {
    apiFetch
      .mockResolvedValueOnce(response([]))
      .mockResolvedValueOnce(service)
      .mockResolvedValueOnce(response([service]))
      .mockResolvedValueOnce(service)
      .mockResolvedValueOnce(service)
      .mockResolvedValueOnce(response([service]))
      .mockResolvedValueOnce({ ...service, status: 'ok' })
      .mockResolvedValueOnce(response([{ ...service, status: 'ok' }]))
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(response([]))

    const adminMonitoring = useAdminMonitoring()
    await vi.waitFor(() => expect(adminMonitoring.loading.value).toBe(false))

    await adminMonitoring.createService(payload)
    await adminMonitoring.loadService(service.id)
    await adminMonitoring.updateService(service.id, payload)
    await adminMonitoring.checkService(service.id)
    await adminMonitoring.deleteService(service.id)

    expect(apiFetch).toHaveBeenNthCalledWith(
      2,
      '/api/v1/admin/monitoring/services',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      4,
      `/api/v1/admin/monitoring/services/${service.id}`,
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      5,
      `/api/v1/admin/monitoring/services/${service.id}`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      7,
      `/api/v1/admin/monitoring/services/${service.id}/check`,
      { method: 'POST' },
    )
    expect(apiFetch).toHaveBeenNthCalledWith(
      9,
      `/api/v1/admin/monitoring/services/${service.id}`,
      { method: 'DELETE' },
    )
  })

  it('stores list errors without throwing', async () => {
    apiFetch.mockRejectedValueOnce(apiError('invalid_sort'))

    const adminMonitoring = useAdminMonitoring()
    await vi.waitFor(() => expect(adminMonitoring.loading.value).toBe(false))

    expect(adminMonitoring.listError.value).toBe(
      'Selected monitoring sort is invalid.',
    )
  })

  it('maps API errors to readable messages', () => {
    expect(
      toAdminMonitoringErrorMessage(apiError('monitoring_service_not_found')),
    ).toBe('Monitoring service no longer exists.')
    expect(toAdminMonitoringErrorMessage(apiError('name_conflict'))).toBe(
      'Another monitoring service already uses this name.',
    )
    expect(toAdminMonitoringErrorMessage(apiError('invalid_status_code'))).toBe(
      'Expected HTTP status code must be between 100 and 599.',
    )
    expect(toAdminMonitoringErrorMessage(apiError('invalid_interval'))).toBe(
      'Check interval must be between 30 seconds and 24 hours.',
    )
  })
})
