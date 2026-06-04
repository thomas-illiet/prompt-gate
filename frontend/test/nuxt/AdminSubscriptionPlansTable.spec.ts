import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminSubscriptionPlansTable from '../../app/components/AdminSubscriptions/AdminSubscriptionPlansTable.vue'
import type { SubscriptionPlan } from '../../app/types/subscriptions'

const plan: SubscriptionPlan = {
  id: 'plan-id',
  name: 'Pro',
  description: 'Production access',
  quota5hTokens: 1000,
  quota7dTokens: null,
  isDefault: false,
  assignedUsersCount: 1,
  assignedServiceAccountsCount: 1,
  assignedDirectAccountsCount: 2,
  assignedIndirectAccountsCount: 0,
  assignedAccountsCount: 2,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountTable(items: SubscriptionPlan[] = [plan]) {
  return mount(AdminSubscriptionPlansTable, {
    props: {
      items,
      loading: false,
      page: 1,
      pageSize: 10,
      sortBy: 'name',
      sortDir: 'asc',
      total: items.length,
    },
    global: {
      stubs: {
        AppRowActionMenu: { template: '<div />' },
        AppSectionCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        AppServerDataTable: {
          props: ['items'],
          template:
            '<div><div v-for="item in items" :key="item.id"><slot name="item.assignedAccountsCount" :item="item" /></div></div>',
        },
        VBtn: { template: '<button><slot /></button>' },
      },
    },
  })
}

describe('AdminSubscriptionPlansTable', () => {
  it('shows assigned account counts for each plan', () => {
    const wrapper = mountTable()

    expect(wrapper.text()).toContain('2 accounts')
    expect(wrapper.text()).toContain(
      '2 direct assignments, 0 indirect assignments',
    )
  })

  it('shows direct and indirect assignment counts when no direct accounts are attached', () => {
    const wrapper = mountTable([
      {
        ...plan,
        assignedUsersCount: 0,
        assignedServiceAccountsCount: 0,
        assignedDirectAccountsCount: 0,
        assignedIndirectAccountsCount: 3,
        assignedAccountsCount: 3,
      },
    ])

    expect(wrapper.text()).toContain('3 accounts')
    expect(wrapper.text()).toContain(
      '0 direct assignments, 3 indirect assignments',
    )
  })
})
