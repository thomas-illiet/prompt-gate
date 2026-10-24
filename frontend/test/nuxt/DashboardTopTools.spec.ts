import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardTopTools from '../../app/components/Dashboard/DashboardTopTools.vue'
import type {
  DashboardTopToolsResponse,
  UsageWindow,
} from '../../app/types/user-service'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return { apiFetch, useApiFetchMock: vi.fn(() => apiFetch) }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function response(
  items: DashboardTopToolsResponse['items'],
  options: {
    attributedCalls?: number
    totalCalls?: number
    window?: UsageWindow
  } = {},
): DashboardTopToolsResponse {
  return {
    window: options.window ?? '7d',
    startsAt: '2026-10-17T00:00:00Z',
    endsAt: '2026-10-24T00:00:00Z',
    totalCalls: options.totalCalls ?? 10,
    attributedCalls: options.attributedCalls ?? 8,
    items,
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, reject, resolve }
}

function mountTopTools(
  props: {
    refreshVersion?: number
    scope?: 'self' | 'global'
    window?: UsageWindow
  } = {},
) {
  return mount(DashboardTopTools, {
    props: {
      refreshVersion: props.refreshVersion ?? 0,
      scope: props.scope ?? 'self',
      window: props.window ?? '7d',
    },
    global: {
      stubs: {
        AppSectionCard: {
          props: ['title', 'subtitle'],
          template:
            '<section><h2>{{ title }}</h2><p>{{ subtitle }}</p><slot /></section>',
        },
        VAlert: { template: '<div data-test="alert"><slot /></div>' },
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\')"><slot /></button>',
        },
      },
    },
  })
}

describe('DashboardTopTools', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  it('ranks tools by observed calls and reports attribution coverage', async () => {
    apiFetch.mockResolvedValueOnce(
      response([
        { name: 'search', calls: 3 },
        { name: 'calculator', calls: 5 },
      ]),
    )

    const wrapper = mountTopTools()

    await vi.waitFor(() => expect(wrapper.text()).toContain('calculator'))
    expect(apiFetch).toHaveBeenCalledWith(
      '/api/v1/me/dashboard/top-tools?window=7d',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(wrapper.get('h2').text()).toBe('Top tool calls')
    expect(wrapper.text()).toContain(
      '8 of 10 observed calls attributed by name (80%).',
    )
    expect(wrapper.text()).toContain(
      'Older calls may predate named tool tracking.',
    )

    const rows = wrapper.findAll('[data-test="tool-item"]')
    expect(rows[0]?.text()).toContain('calculator')
    expect(rows[0]?.text()).toContain('5 observed calls')
    expect(rows[1]?.text()).toContain('search')
  })

  it('aborts obsolete requests and never shows data from another scope', async () => {
    const selfRequest = deferred<DashboardTopToolsResponse>()
    const globalRequest = deferred<DashboardTopToolsResponse>()
    apiFetch
      .mockReturnValueOnce(selfRequest.promise)
      .mockReturnValueOnce(globalRequest.promise)
    const wrapper = mountTopTools()

    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1))
    const selfSignal = apiFetch.mock.calls[0]?.[1]?.signal as AbortSignal
    await wrapper.setProps({ scope: 'global' })

    expect(selfSignal.aborted).toBe(true)
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2))
    expect(apiFetch.mock.calls[1]?.[0]).toBe(
      '/api/v1/admin/dashboard/top-tools?window=7d',
    )

    selfRequest.resolve(response([{ name: 'self-only', calls: 9 }]))
    await Promise.resolve()
    expect(wrapper.text()).not.toContain('self-only')

    globalRequest.resolve(response([{ name: 'global-tool', calls: 4 }]))
    await vi.waitFor(() => expect(wrapper.text()).toContain('global-tool'))
    expect(wrapper.text()).not.toContain('self-only')
  })

  it('keeps the last loaded data when a refresh fails and can retry', async () => {
    apiFetch
      .mockResolvedValueOnce(response([{ name: 'stable-tool', calls: 6 }]))
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(response([{ name: 'recovered-tool', calls: 7 }]))
    const wrapper = mountTopTools()

    await vi.waitFor(() => expect(wrapper.text()).toContain('stable-tool'))
    await wrapper.setProps({ refreshVersion: 1 })

    await vi.waitFor(() =>
      expect(wrapper.text()).toContain(
        'Unable to refresh top tool calls. Showing the last loaded data.',
      ),
    )
    expect(wrapper.text()).toContain('stable-tool')

    await wrapper.get('button').trigger('click')
    await vi.waitFor(() => expect(wrapper.text()).toContain('recovered-tool'))
    expect(wrapper.text()).not.toContain('stable-tool')
  })

  it('shows the initial error and recovers when retry succeeds', async () => {
    apiFetch
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(response([{ name: 'retried-tool', calls: 2 }]))
    const wrapper = mountTopTools()

    await vi.waitFor(() =>
      expect(wrapper.text()).toContain('Unable to load top tool calls.'),
    )
    await wrapper.get('button').trigger('click')

    await vi.waitFor(() => expect(wrapper.text()).toContain('retried-tool'))
    expect(wrapper.text()).not.toContain('Unable to load top tool calls.')
  })

  it('keeps the complete accessible name for long tool identifiers', async () => {
    const longName =
      'organization__knowledge-base__search-with-a-very-long-tool-identifier'
    apiFetch.mockResolvedValueOnce(
      response([{ name: longName, calls: 1 }], {
        attributedCalls: 1,
        totalCalls: 1,
      }),
    )
    const wrapper = mountTopTools()

    await vi.waitFor(() => expect(wrapper.text()).toContain(longName))
    expect(wrapper.get('[data-test="tool-item"]').text()).toContain(longName)
  })

  it('distinguishes no observed calls from calls without a tool name', async () => {
    apiFetch
      .mockResolvedValueOnce(
        response([], { attributedCalls: 0, totalCalls: 0 }),
      )
      .mockResolvedValueOnce(
        response([], {
          attributedCalls: 0,
          totalCalls: 4,
          window: '30d',
        }),
      )
    const wrapper = mountTopTools()

    await vi.waitFor(() =>
      expect(wrapper.text()).toContain(
        'No observed tool calls for this period.',
      ),
    )
    await wrapper.setProps({ window: '30d' })

    await vi.waitFor(() =>
      expect(wrapper.text()).toContain(
        'Observed tool calls could not be attributed by name for this period.',
      ),
    )
  })
})
