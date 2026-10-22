import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import AdminGroupsTable from '../../app/components/AdminGroups/AdminGroupsTable.vue'
import type { AccessGroup } from '../../app/types/groups'
import type { AppRowAction } from '../../app/types/row-actions'

const group: AccessGroup = {
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
              type: Array as PropType<AppRowAction<AccessGroup>[]>,
              required: true,
            },
            item: {
              type: Object as PropType<AccessGroup>,
              required: true,
            },
          },
          setup(props) {
            function resolveTitle(action: AppRowAction<AccessGroup>) {
              return typeof action.title === 'function'
                ? action.title(props.item)
                : action.title
            }

            function selectAction(action: AppRowAction<AccessGroup>) {
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
            '<div><div v-for="item in items" :key="item.id"><slot name="item.name" :item="item" /><slot name="item.actions" :item="item" /></div></div>',
        },
        VBtn: { template: '<button><slot /></button>' },
      },
    },
  })
}

describe('AdminGroupsTable', () => {
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
