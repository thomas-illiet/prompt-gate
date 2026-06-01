import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import AdminServiceAccountsTable from '../../app/components/AdminServiceAccounts/AdminServiceAccountsTable.vue'
import type { AppRowAction } from '../../app/types/row-actions'
import type { ServiceAccount } from '../../app/types/service-accounts'

const account: ServiceAccount = {
  id: 'service-account-id',
  identifier: 'ci_runner',
  name: 'CI runner',
  role: 'user',
  note: '',
  isActive: true,
  firewallOverrideEnabled: false,
  inputTokens: 1234,
  outputTokens: 5678,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountTable() {
  return mount(AdminServiceAccountsTable, {
    props: {
      items: [account],
      loading: false,
      page: 1,
      pageSize: 10,
      sortBy: 'createdAt',
      sortDir: 'desc',
      total: 1,
    },
    global: {
      stubs: {
        AppRowActionMenu: defineComponent({
          props: {
            actions: {
              type: Array as PropType<AppRowAction<ServiceAccount>[]>,
              required: true,
            },
            item: {
              type: Object as PropType<ServiceAccount>,
              required: true,
            },
          },
          setup(props) {
            function resolveTitle(action: AppRowAction<ServiceAccount>) {
              return typeof action.title === 'function'
                ? action.title(props.item)
                : action.title
            }

            function selectAction(action: AppRowAction<ServiceAccount>) {
              action.onSelect?.(props.item)
            }

            return { resolveTitle, selectAction }
          },
          template:
            '<div><button v-for="action in actions" :key="action.key" :data-test="\'row-action-\' + action.key" @click="selectAction(action)">{{ resolveTitle(action) }}</button></div>',
        }),
        AppSectionCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        AppServerDataTable: {
          props: ['items'],
          template:
            '<div><div v-for="item in items" :key="item.id"><slot name="item.name" :item="item" /><slot name="item.identifier" :item="item" /><slot name="item.actions" :item="item" /></div></div>',
        },
        VBtn: { template: '<button><slot /></button>' },
        VChip: { template: '<span><slot /></span>' },
      },
    },
  })
}

describe('AdminServiceAccountsTable', () => {
  it('exposes a notes row action', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="row-action-notes"]').text()).toBe('Notes')

    await wrapper.get('[data-test="row-action-notes"]').trigger('click')

    expect(wrapper.emitted('notes')).toEqual([[account]])
  })
})
