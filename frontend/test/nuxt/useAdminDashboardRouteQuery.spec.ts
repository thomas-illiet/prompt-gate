import { mount } from '@vue/test-utils'
import { defineComponent, nextTick, reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'

import {
  useAdminDashboardAnalysisQuery,
  useAdminDashboardWindowQuery,
} from '../../app/composables/useAdminDashboardRouteQuery'

const { routeHolder, useRouteMock } = vi.hoisted(() => {
  const routeHolder: { current: { query: Record<string, string> } } = {
    current: { query: {} },
  }
  return {
    routeHolder,
    useRouteMock: vi.fn(() => routeHolder.current),
  }
})

mockNuxtImport('useRoute', () => useRouteMock)

let routerReplace: ReturnType<typeof vi.spyOn>
let routeQuery: Record<string, string>

function mountProbe(
  allowed: Array<'models' | 'providers' | 'provider-types' | 'identities'> = [
    'models',
    'providers',
    'provider-types',
  ],
  fallback: 'models' | 'identities' = 'models',
) {
  const Probe = defineComponent({
    setup() {
      useAdminDashboardAnalysisQuery(allowed, fallback)
      return () => null
    },
  })
  return mount(Probe)
}

describe('useAdminDashboardAnalysisQuery', () => {
  beforeEach(() => {
    routeQuery = reactive<Record<string, string>>({})
    routeHolder.current = { query: routeQuery }
    routerReplace = vi
      .spyOn(useRouter(), 'replace')
      .mockResolvedValue(undefined)
  })

  afterEach(() => {
    routerReplace.mockRestore()
  })

  it('normalizes invalid analytical filters and removes the legacy scope', async () => {
    Object.assign(routeQuery, {
      breakdown: 'identities',
      kept: 'yes',
      metric: 'unknown',
      scope: 'global',
      window: 'invalid',
    })
    const wrapper = mountProbe()

    await vi.waitFor(() => expect(routerReplace).toHaveBeenCalled())
    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        breakdown: 'models',
        kept: 'yes',
        metric: 'requests',
        window: '7d',
      },
    })
    wrapper.unmount()
  })

  it('normalizes Activity to its identity-only breakdown while preserving valid filters', async () => {
    Object.assign(routeQuery, {
      breakdown: 'models',
      metric: 'cost',
      window: 'all',
    })
    const wrapper = mountProbe(['identities'], 'identities')

    await vi.waitFor(() => expect(routerReplace).toHaveBeenCalled())
    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        breakdown: 'identities',
        metric: 'cost',
        window: 'all',
      },
    })
    wrapper.unmount()
  })

  it('removes the legacy scope even when the overview window is already valid', async () => {
    Object.assign(routeQuery, { kept: 'yes', scope: 'global', window: '7d' })
    const Probe = defineComponent({
      setup() {
        useAdminDashboardWindowQuery()
        return () => null
      },
    })
    const wrapper = mount(Probe)

    await vi.waitFor(() => expect(routerReplace).toHaveBeenCalled())
    expect(routerReplace).toHaveBeenLastCalledWith({
      query: { kept: 'yes', window: '7d' },
    })
    wrapper.unmount()
  })

  it('renormalizes relevant query values mutated after mount', async () => {
    Object.assign(routeQuery, {
      breakdown: 'models',
      metric: 'requests',
      window: '7d',
    })
    const wrapper = mountProbe()
    expect(routerReplace).not.toHaveBeenCalled()

    routeQuery.scope = 'global'
    await nextTick()
    await vi.waitFor(() => expect(routerReplace).toHaveBeenCalled())
    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        breakdown: 'models',
        metric: 'requests',
        window: '7d',
      },
    })

    routerReplace.mockClear()
    Reflect.deleteProperty(routeQuery, 'scope')
    routeQuery.metric = 'invalid'
    await nextTick()
    await vi.waitFor(() => expect(routerReplace).toHaveBeenCalled())
    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        breakdown: 'models',
        metric: 'requests',
        window: '7d',
      },
    })
    wrapper.unmount()
  })
})
