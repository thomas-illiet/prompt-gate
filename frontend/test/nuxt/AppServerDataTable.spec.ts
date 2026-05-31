import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import AppServerDataTable from '../../app/components/App/AppServerDataTable.vue'

interface TestRow {
  id: string
  name: string
}

function mountTable(options: {
  props?: Partial<{
    headers: { title: string; key: string }[]
    items: TestRow[]
    loading: boolean
    loadingDelayMs: number
    pageSize: number
    refreshIndicatorMinMs: number
    total: number
  }>
  slots?: Record<string, string>
} = {}) {
  return mount(AppServerDataTable<TestRow>, {
    props: {
      defaultSortBy: 'createdAt',
      defaultSortDir: 'desc',
      headers: [{ title: 'Name', key: 'name' }],
      items: [{ id: 'row-1', name: 'Alpha' }],
      loading: false,
      page: 1,
      pageSize: 10,
      sortBy: 'createdAt',
      sortDir: 'desc',
      total: 1,
      ...options.props,
    },
    slots: {
      'item.name':
        '<template #default="{ item }"><span data-test="name">{{ item.name }}</span></template>',
      ...options.slots,
    },
    global: {
      stubs: {
        AppEmptyState: {
          props: ['compact', 'icon', 'title', 'text', 'tone'],
          template: `
            <section data-test="empty-state">
              <span data-test="empty-title">{{ title }}</span>
              <span data-test="empty-text">{{ text }}</span>
            </section>
          `,
        },
        VDataTableServer: {
          emits: ['update:items-per-page', 'update:page', 'update:sort-by'],
          props: [
            'headers',
            'items',
            'itemsLength',
            'itemsPerPage',
            'loading',
            'page',
            'sortBy',
          ],
          template: `
            <section>
              <slot v-if="loading" name="loading" />
              <slot v-else-if="items.length === 0" name="no-data" />
              <slot v-else name="item.name" :item="items[0]" />
              <button data-test="page" @click="$emit('update:page', 2)">page</button>
              <button data-test="page-size" @click="$emit('update:items-per-page', 25)">page size</button>
              <button data-test="sort" @click="$emit('update:sort-by', [{ key: 'name', order: 'asc' }])">sort</button>
              <button data-test="clear-sort" @click="$emit('update:sort-by', [])">clear sort</button>
            </section>
          `,
        },
      },
    },
  })
}

describe('AppServerDataTable', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('forwards item slots and pagination events', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="name"]').text()).toBe('Alpha')

    await wrapper.get('[data-test="page"]').trigger('click')
    await wrapper.get('[data-test="page-size"]').trigger('click')

    expect(wrapper.emitted('update:page')).toEqual([[2]])
    expect(wrapper.emitted('update:page-size')).toEqual([[25]])
  })

  it('normalizes Vuetify sort updates and restores default sort', async () => {
    const wrapper = mountTable()

    await wrapper.get('[data-test="sort"]').trigger('click')
    await wrapper.get('[data-test="clear-sort"]').trigger('click')

    expect(wrapper.emitted('update:sort')).toEqual([
      ['name', 'asc'],
      ['createdAt', 'desc'],
    ])
  })

  it('renders loading skeleton while loading', () => {
    const wrapper = mountTable({
      props: {
        items: [],
        loading: true,
        total: 0,
      },
    })

    expect(wrapper.find('[data-test="table-refresh-indicator"]').exists()).toBe(
      true,
    )
    expect(wrapper.find('[data-test="table-loading-skeleton"]').exists()).toBe(
      true,
    )
  })

  it('keeps existing rows visible during short refreshes', async () => {
    vi.useFakeTimers()

    const wrapper = mountTable({
      props: {
        loading: true,
      },
    })

    expect(wrapper.find('[data-test="table-loading-skeleton"]').exists()).toBe(
      false,
    )
    expect(wrapper.find('[data-test="table-refresh-indicator"]').exists()).toBe(
      true,
    )
    expect(wrapper.get('[data-test="name"]').text()).toBe('Alpha')

    await wrapper.setProps({ loading: false })
    await vi.advanceTimersByTimeAsync(180)

    expect(wrapper.find('[data-test="table-refresh-indicator"]').exists()).toBe(
      true,
    )
    expect(wrapper.find('[data-test="table-loading-skeleton"]').exists()).toBe(
      false,
    )
    expect(wrapper.get('[data-test="name"]').text()).toBe('Alpha')

    await vi.advanceTimersByTimeAsync(140)
    await wrapper.vm.$nextTick()

    expect(wrapper.find('[data-test="table-refresh-indicator"]').exists()).toBe(
      false,
    )
  })

  it('renders loading skeleton over existing rows after the refresh delay', async () => {
    vi.useFakeTimers()

    const wrapper = mountTable({
      props: {
        loading: true,
      },
    })

    expect(wrapper.find('[data-test="table-loading-skeleton"]').exists()).toBe(
      false,
    )

    await vi.advanceTimersByTimeAsync(180)
    await wrapper.vm.$nextTick()

    expect(wrapper.find('[data-test="table-loading-skeleton"]').exists()).toBe(
      true,
    )
    expect(wrapper.find('[data-test="name"]').exists()).toBe(false)
  })

  it('renders a default empty state when no rows are available', () => {
    const wrapper = mountTable({
      props: {
        items: [],
        total: 0,
      },
    })

    expect(wrapper.get('[data-test="empty-title"]').text()).toBe('No results')
    expect(wrapper.get('[data-test="empty-text"]').text()).toBe(
      'There is nothing to show for the current filters.',
    )
  })

  it('lets custom no-data content override the default empty state', () => {
    const wrapper = mountTable({
      props: {
        items: [],
        total: 0,
      },
      slots: {
        'no-data': '<div data-test="custom-empty">Custom empty</div>',
      },
    })

    expect(wrapper.get('[data-test="custom-empty"]').text()).toBe(
      'Custom empty',
    )
    expect(wrapper.find('[data-test="empty-state"]').exists()).toBe(false)
  })
})
