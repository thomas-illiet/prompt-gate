import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it } from 'vitest'

import AdminSetupGuidesTable from '../../app/components/AdminSetupGuides/AdminSetupGuidesTable.vue'

const DataTableStub = defineComponent({
  name: 'VDataTable',
  props: {
    headers: { type: Array, required: true },
    items: { type: Array, required: true },
  },
  template: '<div />',
})

describe('AdminSetupGuidesTable', () => {
  it('centers compact enum columns while preserving manual ordering', () => {
    const wrapper = mount(AdminSetupGuidesTable, {
      props: { guides: [], loading: false },
      global: { stubs: { VDataTable: DataTableStub } },
    })
    const headers = wrapper
      .getComponent(DataTableStub)
      .props('headers') as Array<{
      align?: string
      cellProps?: { class?: string }
      headerProps?: { class?: string }
      key: string
      sortable?: boolean
    }>

    expect(
      headers.map((header) => ({
        centered: header.align === 'center',
        key: header.key,
        sortable: header.sortable !== false,
      })),
    ).toEqual([
      { centered: true, key: 'position', sortable: false },
      { centered: false, key: 'title', sortable: false },
      { centered: true, key: 'compatibility', sortable: false },
      { centered: true, key: 'modelMode', sortable: false },
      { centered: true, key: 'enabled', sortable: false },
      { centered: true, key: 'actions', sortable: false },
    ])
    expect(
      headers
        .filter((header) => header.align === 'center')
        .every(
          (header) =>
            header.cellProps?.class === 'text-center' &&
            header.headerProps?.class === 'text-center',
        ),
    ).toBe(true)
  })
})
