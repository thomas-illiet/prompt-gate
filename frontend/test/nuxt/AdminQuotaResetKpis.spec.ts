import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminQuotaResetKpis from '../../app/components/AdminQuotaResets/AdminQuotaResetKpis.vue'

const summaries = {
  eventsSummary: {
    monthlyGrants: 0,
    adminGrants: 0,
    appliedResetUses: 0,
    pendingResetUsesTotal: 0,
  },
  statesSummary: {
    projectedAccounts: 0,
    atRiskAccounts: 0,
    exhaustedAccounts: 0,
  },
}

function mountKpis(statesAvailable: boolean, eventsAvailable: boolean) {
  return mount(AdminQuotaResetKpis, {
    props: {
      ...summaries,
      eventsAvailable,
      eventsLoading: false,
      statesAvailable,
      statesLoading: false,
    },
    global: {
      stubs: {
        VAvatar: { template: '<span><slot /></span>' },
        VIcon: true,
      },
    },
  })
}

describe('AdminQuotaResetKpis', () => {
  it('shows unavailable values instead of authoritative zeros after initial failures', () => {
    const wrapper = mountKpis(false, false)

    expect(wrapper.findAll('strong').map((item) => item.text())).toEqual([
      '—',
      '—',
      '—',
      '—',
      '—',
      '—',
    ])
    expect(wrapper.text()).toContain('Projection summary unavailable')
    expect(wrapper.text()).toContain('Event summary unavailable')
  })

  it('distinguishes successfully loaded zero counts', () => {
    const wrapper = mountKpis(true, true)

    expect(wrapper.findAll('strong').map((item) => item.text())).toEqual([
      '0',
      '0',
      '0',
      '0 / 0',
      '0',
      '0',
    ])
    expect(wrapper.text()).not.toContain('unavailable')
  })
})
