import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { describe, expect, it } from 'vitest'

import AdminQuotaResetEventsSection from '../../app/components/AdminQuotaResets/AdminQuotaResetEventsSection.vue'
import AdminQuotaStatesSection from '../../app/components/AdminQuotaResets/AdminQuotaStatesSection.vue'
import type {
  AdminQuotaResetEventItem,
  AdminQuotaStateItem,
} from '../../app/types/quota-resets'

const ServerTableStub = defineComponent({
  props: { items: { type: Array, required: true } },
  setup(props, { slots }) {
    return () =>
      h(
        'div',
        {
          'data-empty': String(props.items.length === 0),
          'data-test': 'table',
        },
        props.items.flatMap((item) =>
          Object.entries(slots)
            .filter(([name]) => name.startsWith('item.'))
            .flatMap(([, slot]) => slot?.({ item }) ?? []),
        ),
      )
  },
})

const NuxtLinkStub = defineComponent({
  props: { to: { type: Object, required: true } },
  setup(props, { slots }) {
    return () =>
      h(
        'a',
        {
          'data-test': 'account-link',
          'data-to': `${props.to.path}?search=${props.to.query.search}`,
        },
        slots.default?.(),
      )
  },
})

function stubs() {
  return {
    AppFilterCard: { template: '<section><slot /></section>' },
    AppSectionCard: { template: '<section><slot /></section>' },
    AppServerDataTable: ServerTableStub,
    NuxtLink: NuxtLinkStub,
    VAlert: { template: '<div data-test="error"><slot /></div>' },
    VBtn: true,
    VBtnToggle: { template: '<div><slot /></div>' },
    VChip: { template: '<span><slot /></span>' },
    VIcon: true,
    VSelect: true,
    VTextField: true,
  }
}

function stateItem(
  accountId: string,
  accountType: 'user' | 'service',
  status: AdminQuotaStateItem['status'],
): AdminQuotaStateItem {
  return {
    accountId,
    accountType,
    displayName: accountType === 'service' ? 'Build bot' : 'Alice',
    planName: status === 'unassigned' ? '' : 'Builder',
    hasSubscription: status !== 'unassigned',
    status,
    maximumPercent: status === 'unlimited' ? null : 85,
    mostSaturatedWindow: status === 'unassigned' ? null : '5h',
    nextResetAt: null,
    syncedAt: '2026-10-25T10:00:00Z',
    windows:
      status === 'unassigned'
        ? []
        : [
            {
              window: '5h',
              usedTokens: 10,
              quotaTokens: null,
              remainingTokens: null,
              percent: null,
              resetAt: null,
            },
            {
              window: '7d',
              usedTokens: 10,
              quotaTokens: null,
              remainingTokens: null,
              percent: null,
              resetAt: null,
            },
          ],
  }
}

function eventItem(
  id: string,
  accountType: 'user' | 'service',
  actorType: 'admin_session' | 'admin_api_key',
): AdminQuotaResetEventItem {
  return {
    id,
    target: {
      accountId: `${accountType}-1`,
      accountType,
      displayName: accountType === 'service' ? 'Build bot' : 'Alice',
    },
    type: 'admin_grant',
    status: 'applied',
    delta: 1,
    balanceAfter: 1,
    actor: { type: actorType, userId: null, displayName: null },
    grantMonth: '2026-10',
    resetCutoffAt: null,
    reconcileAttempts: 0,
    lastReconcileAttemptAt: null,
    appliedAt: '2026-10-25T10:00:00Z',
    createdAt: '2026-10-25T10:00:00Z',
  }
}

describe('admin quota reset list sections', () => {
  it('links user/service projections and distinguishes unassigned from unlimited', () => {
    const wrapper = mount(AdminQuotaStatesSection, {
      props: {
        accountType: 'all',
        error: 'Latest projection refresh failed.',
        items: [
          stateItem('user-1', 'user', 'unassigned'),
          stateItem('service-1', 'service', 'unlimited'),
        ],
        loading: false,
        page: 1,
        pageSize: 20,
        projectionSyncedAt: null,
        search: '',
        sortBy: 'saturation',
        sortDir: 'desc',
        total: 2,
        view: 'at-risk',
      },
      global: { stubs: stubs() },
    })

    expect(
      wrapper
        .findAll('[data-test="account-link"]')
        .map((link) => link.attributes('data-to')),
    ).toEqual([
      '/admin/users?search=user-1',
      '/admin/service-accounts?search=service-1',
    ])
    expect(wrapper.text()).toContain('Unassigned')
    expect(wrapper.text()).toContain('No active plan')
    expect(wrapper.text()).toContain('Unlimited')
    expect(wrapper.text()).toContain('No token ceiling')
    expect(wrapper.get('[data-test="error"]').text()).toContain(
      'Latest projection refresh failed.',
    )
  })

  it('links event targets and labels deleted sessions separately from API keys', () => {
    const wrapper = mount(AdminQuotaResetEventsSection, {
      props: {
        error: null,
        items: [
          eventItem('event-user', 'user', 'admin_session'),
          eventItem('event-service', 'service', 'admin_api_key'),
        ],
        loading: false,
        page: 1,
        pageSize: 20,
        search: '',
        sortBy: 'createdAt',
        sortDir: 'desc',
        status: 'all',
        total: 2,
        type: 'all',
        window: '30d',
      },
      global: { stubs: stubs() },
    })

    expect(
      wrapper
        .findAll('[data-test="account-link"]')
        .map((link) => link.attributes('data-to')),
    ).toEqual([
      '/admin/users?search=user-1',
      '/admin/service-accounts?search=service-1',
    ])
    expect(wrapper.text()).toContain('Deleted administrator')
    expect(wrapper.text()).toContain('Administrator API key')
    expect(wrapper.text()).not.toContain('idempotency')
  })
})
