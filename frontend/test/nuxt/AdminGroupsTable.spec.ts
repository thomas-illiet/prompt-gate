import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import AdminGroupsTable from '../../app/components/AdminGroups/AdminGroupsTable.vue'
import type { AdminGroupListItem } from '../../app/types/groups'
import type { AppRowAction } from '../../app/types/row-actions'

const group: AdminGroupListItem = {
  id: 'group-id',
  name: 'engineering',
  displayName: 'Engineering',
  description: 'Engineering access',
  providers: [],
  modelPatterns: ['^gpt-5'],
  excludedModelPatterns: [],
  members: [],
  providerCount: 0,
  modelPatternCount: 1,
  memberCount: 0,
  inputTokens: 1_234_567,
  outputTokens: 0,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountTable() {
  return mount(AdminGroupsTable, {
    props: {
      items: [group],
      loading: false,
      page: 1,
      pageSize: 10,
      sortBy: 'name',
      sortDir: 'asc',
      total: 1,
    },
    global: {
      stubs: {
        AppRowActionMenu: defineComponent({
          props: {
            actions: {
              type: Array as PropType<AppRowAction<AdminGroupListItem>[]>,
              required: true,
            },
            item: {
              type: Object as PropType<AdminGroupListItem>,
              required: true,
            },
          },
          setup(props) {
            function resolveTitle(action: AppRowAction<AdminGroupListItem>) {
              return typeof action.title === 'function'
                ? action.title(props.item)
                : action.title
            }

            function selectAction(action: AppRowAction<AdminGroupListItem>) {
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
        AppServerDataTable: defineComponent({
          name: 'AppServerDataTable',
          props: ['headers', 'items'],
          template:
            '<div><div v-for="item in items" :key="item.id"><slot name="item.name" :item="item" /><span data-test="input-tokens"><slot name="item.inputTokens" :item="item" /></span><span data-test="output-tokens"><slot name="item.outputTokens" :item="item" /></span><slot name="item.actions" :item="item" /></div></div>',
        }),
        VBtn: { template: '<button><slot /></button>' },
      },
    },
  })
}

describe('AdminGroupsTable', () => {
  it('shows sortable total-token columns after members', () => {
    const wrapper = mountTable()
    const table = wrapper.getComponent({ name: 'AppServerDataTable' })

    expect(
      table
        .props('headers')
        .map(({ title, key }: { title: string; key: string }) => ({
          title,
          key,
        })),
    ).toEqual([
      { title: 'Name', key: 'name' },
      { title: 'Providers', key: 'providerCount' },
      { title: 'Model rules', key: 'modelPatternCount' },
      { title: 'Members', key: 'memberCount' },
      { title: 'Total input tokens', key: 'inputTokens' },
      { title: 'Total output tokens', key: 'outputTokens' },
      { title: 'Actions', key: 'actions' },
    ])
    expect(table.props('headers')[4]).not.toMatchObject({ sortable: false })
    expect(table.props('headers')[5]).not.toMatchObject({ sortable: false })
  })

  it('formats large and zero token totals', () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="input-tokens"]').text()).toBe('1,234,567')
    expect(wrapper.get('[data-test="output-tokens"]').text()).toBe('0')
  })

  it('emits the selected group from the usage-statistics action', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="row-action-usageStatistics"]').text()).toBe(
      'Usage statistics',
    )

    await wrapper
      .get('[data-test="row-action-usageStatistics"]')
      .trigger('click')

    expect(wrapper.emitted('usageStatistics')).toEqual([[group]])
  })
})
