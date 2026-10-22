import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardCurrentState from '../../app/components/Dashboard/DashboardCurrentState.vue'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return { apiFetch, useApiFetchMock: vi.fn(() => apiFetch) }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

function quotaResponse(used5h: number, quota5h: number | null, used7d: number) {
  return {
    hasSubscription: true,
    plan: { id: 'plan', name: 'Builder' },
    used5hTokens: used5h,
    quota5hTokens: quota5h,
    remaining5hTokens:
      quota5h == null ? null : Math.max(0, quota5h - used5h),
    reset5hAt: '2026-10-22T12:00:00Z',
    used7dTokens: used7d,
    quota7dTokens: 1000,
    remaining7dTokens: Math.max(0, 1000 - used7d),
    reset7dAt: '2026-10-29T12:00:00Z',
  }
}

function mountState(scope: 'self' | 'global' = 'self') {
  return mount(DashboardCurrentState, {
    props: { refreshVersion: 0, scope },
    global: {
      stubs: {
        AppSectionCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        NuxtLink: {
          props: ['to'],
          template:
            '<a data-test="risk-link" :data-to="to.path + \'?search=\' + to.query.search"><slot /></a>',
        },
        VAlert: { template: '<div><slot /></div>' },
        VBtn: { template: '<button><slot /></button>' },
        VChip: { template: '<span><slot /></span>' },
        VIcon: true,
        VProgressLinear: {
          props: ['modelValue', 'color'],
          template:
            '<div data-test="progress" :data-value="modelValue" :data-color="color" />',
        },
      },
    },
  })
}

describe('DashboardCurrentState', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    useApiFetchMock.mockClear()
  })

  it('uses neutral below 80 percent and warning from 80 percent', async () => {
    apiFetch.mockResolvedValueOnce(quotaResponse(799, 1000, 800))
    const wrapper = mountState()

    await vi.waitFor(() => expect(wrapper.text()).toContain('79.9% used'))

    expect(wrapper.text()).toContain('Within limit')
    expect(wrapper.text()).toContain('Approaching limit')
    const progress = wrapper.findAll('[data-test="progress"]')
    expect(progress[0]?.attributes('data-color')).toBe('primary')
    expect(progress[1]?.attributes('data-color')).toBe('warning')
  })

  it('marks 100 percent critical and renders unlimited without an empty bar', async () => {
    apiFetch.mockResolvedValueOnce(quotaResponse(10, null, 1000))
    const wrapper = mountState()

    await vi.waitFor(() => expect(wrapper.text()).toContain('Quota exhausted'))

    expect(wrapper.text()).toContain('Unlimited')
    expect(wrapper.text()).toContain('No token ceiling for this window')
    expect(wrapper.findAll('[data-test="progress"]')).toHaveLength(1)
    expect(wrapper.get('[data-test="progress"]').attributes('data-color')).toBe(
      'error',
    )
  })

  it('links quota risk rows to the matching filtered admin list', async () => {
    apiFetch.mockResolvedValueOnce({
      generatedAt: '2026-10-22T10:00:00Z',
      projectionSyncedAt: '2026-10-22T09:58:00Z',
      atRiskCount: 1,
      exhaustedCount: 1,
      items: [
        {
          accountId: 'service-account-id',
          accountType: 'service',
          displayName: 'Build bot',
          planName: 'Automation',
          window: '5h',
          usedTokens: 110,
          quotaTokens: 100,
          remainingTokens: 0,
          percent: 110,
          resetAt: '2026-10-22T12:00:00Z',
          syncedAt: '2026-10-22T09:58:00Z',
          status: 'critical',
        },
      ],
    })
    const wrapper = mountState('global')

    await vi.waitFor(() => expect(wrapper.text()).toContain('Build bot'))

    expect(wrapper.get('[data-test="risk-link"]').attributes('data-to')).toBe(
      '/admin/service-accounts?search=service-account-id',
    )
    expect(wrapper.get('.dashboard-current-state__risk-chip').text()).toBe(
      'Critical · 110%',
    )
    expect(wrapper.text()).toContain('0 tokens left')
    expect(wrapper.text()).toContain('Resets')
    expect(apiFetch.mock.calls[0]?.[0]).toBe(
      '/api/v1/admin/dashboard/quota-risk',
    )
  })
})
