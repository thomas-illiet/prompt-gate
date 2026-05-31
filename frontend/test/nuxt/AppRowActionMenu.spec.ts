import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AppRowActionMenu from '../../app/components/App/AppRowActionMenu.vue'
import type { AppRowAction } from '../../app/types/row-actions'

interface TestRow {
  id: string
  locked: boolean
  name: string
}

function mountMenu(actions: AppRowAction<TestRow>[], item: TestRow) {
  return mount(AppRowActionMenu<TestRow>, {
    props: {
      actions,
      item,
    },
    global: {
      stubs: {
        VBtn: {
          props: ['ariaLabel'],
          template:
            '<button data-test="activator" :aria-label="ariaLabel"><slot /><slot name="append" /></button>',
        },
        VIcon: { template: '<i />' },
        VList: { template: '<div><slot /></div>' },
        VListItem: {
          emits: ['click'],
          props: ['disabled', 'title'],
          template:
            '<button data-test="action" :disabled="disabled" @click="$emit(\'click\')">{{ title }}</button>',
        },
        VMenu: {
          template: '<div><slot name="activator" :props="{}" /><slot /></div>',
        },
      },
    },
  })
}

describe('AppRowActionMenu', () => {
  it('runs item-aware action callbacks and emits selected context', async () => {
    const item = { id: 'row-1', locked: false, name: 'Alpha' }
    const onSelect = vi.fn()
    const actions: AppRowAction<TestRow>[] = [
      {
        icon: 'mdi-pencil-outline',
        key: 'edit',
        onSelect,
        title: (row) => `Edit ${row.name}`,
      },
    ]

    const wrapper = mountMenu(actions, item)

    expect(wrapper.get('[data-test="action"]').text()).toBe('Edit Alpha')

    await wrapper.get('[data-test="action"]').trigger('click')

    expect(onSelect).toHaveBeenCalledWith(item)
    expect(wrapper.emitted('select')).toEqual([
      ['edit', { action: actions[0], item }],
    ])
  })

  it('keeps disabled item-aware actions inert', async () => {
    const item = { id: 'row-1', locked: true, name: 'Alpha' }
    const onSelect = vi.fn()
    const actions: AppRowAction<TestRow>[] = [
      {
        disabled: (row) => row.locked,
        icon: 'mdi-delete-outline',
        key: 'delete',
        onSelect,
        title: 'Delete',
      },
    ]

    const wrapper = mountMenu(actions, item)

    expect(wrapper.get('[data-test="action"]').attributes('disabled')).toBe('')

    await wrapper.get('[data-test="action"]').trigger('click')

    expect(onSelect).not.toHaveBeenCalled()
    expect(wrapper.emitted('select')).toBeUndefined()
  })
})
