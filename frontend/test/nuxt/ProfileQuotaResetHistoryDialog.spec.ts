import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent } from 'vue'

import ProfileQuotaResetHistoryDialog from '../../app/components/Profile/ProfileQuotaResetHistoryDialog.vue'
import type { QuotaResetHistoryItem } from '../../app/types/quota-resets'

const items: QuotaResetHistoryItem[] = [
  {
    id: 'monthly-event',
    type: 'monthly_grant',
    delta: 1,
    balanceAfter: 2,
    createdAt: '2026-10-01T08:00:00Z',
  },
  {
    id: 'used-event',
    type: 'quota_reset_used',
    delta: -1,
    balanceAfter: 1,
    createdAt: '2026-10-02T08:00:00Z',
  },
  {
    id: 'admin-event',
    type: 'admin_grant',
    delta: 1,
    balanceAfter: 2,
    createdAt: '2026-10-03T08:00:00Z',
  },
]

const ServerTableStub = defineComponent({
  props: {
    headers: { type: Array, required: true },
    items: { type: Array, required: true },
    sortBy: { type: String, required: true },
    sortDir: { type: String, required: true },
  },
  emits: ['update:page', 'update:page-size', 'update:sort'],
  template: `
    <div>
      <div v-for="item in items" :key="item.id">
        <slot name="item.createdAt" :item="item" />
        <slot name="item.type" :item="item" />
        <slot name="item.delta" :item="item" />
        <slot name="item.balanceAfter" :item="item" />
      </div>
      <button data-test="next-history-page" @click="$emit('update:page', 2)">
        Next page
      </button>
      <button data-test="change-history-page-size" @click="$emit('update:page-size', 50)">
        50 rows
      </button>
      <button data-test="sort-history" @click="$emit('update:sort', 'delta', 'asc')">
        Sort
      </button>
    </div>
  `,
})

function mountDialog(overrides: Record<string, unknown> = {}) {
  return mount(ProfileQuotaResetHistoryDialog, {
    props: {
      error: null,
      items,
      loading: false,
      maximum: 2,
      modelValue: true,
      page: 1,
      pageSize: 20,
      sortBy: 'createdAt',
      sortDir: 'desc',
      total: 3,
      ...overrides,
    },
    global: {
      stubs: {
        AppDialogCard: {
          props: ['modelValue', 'subtitle', 'title'],
          template:
            '<section v-if="modelValue"><h2>{{ title }}</h2><p>{{ subtitle }}</p><slot /><slot name="actions" /></section>',
        },
        AppDialogCloseButton: { template: '<button>Close</button>' },
        AppServerDataTable: ServerTableStub,
        VAlert: { template: '<div><slot /></div>' },
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\')"><slot /></button>',
        },
        VChip: { template: '<span><slot /></span>' },
      },
    },
  })
}

describe('ProfileQuotaResetHistoryDialog', () => {
  it('renders grant and consumption entries with resulting balances', () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).toContain('3 history entries.')
    expect(wrapper.text()).toContain('Monthly credit')
    expect(wrapper.text()).toContain('Granted credit')
    expect(wrapper.text()).not.toContain('Administrator')
    expect(wrapper.text()).toContain('Quota reset used')
    expect(wrapper.text()).toContain('+1')
    expect(wrapper.text()).toContain('-1')
    expect(wrapper.text()).toContain('2 / 2')
    expect(wrapper.text()).toContain('1 / 2')
  })

  it('makes every data column sortable and centered', () => {
    const wrapper = mountDialog()
    const table = wrapper.findComponent(ServerTableStub)

    expect(table.props('headers')).toEqual([
      expect.objectContaining({ key: 'createdAt', align: 'center' }),
      expect.objectContaining({ key: 'type', align: 'center' }),
      expect.objectContaining({ key: 'delta', align: 'center' }),
      expect.objectContaining({ key: 'balanceAfter', align: 'center' }),
    ])
    expect(
      (table.props('headers') as Array<{ sortable?: boolean }>).every(
        (header) => header.sortable !== false,
      ),
    ).toBe(true)
    expect(table.props('sortBy')).toBe('createdAt')
    expect(table.props('sortDir')).toBe('desc')
  })

  it('forwards server pagination and sorting changes', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="next-history-page"]').trigger('click')
    await wrapper.get('[data-test="change-history-page-size"]').trigger('click')
    await wrapper.get('[data-test="sort-history"]').trigger('click')

    expect(wrapper.emitted('update:page')).toEqual([[2]])
    expect(wrapper.emitted('update:page-size')).toEqual([[50]])
    expect(wrapper.emitted('update:sort')).toEqual([['delta', 'asc']])
  })

  it('shows history errors and emits refresh', async () => {
    const wrapper = mountDialog({
      error: 'Unable to load quota reset history.',
      items: [],
      total: 0,
    })

    expect(wrapper.text()).toContain('Unable to load quota reset history.')
    await wrapper.findAll('button')[0]?.trigger('click')

    expect(wrapper.emitted('refresh')).toEqual([[]])
  })
})
