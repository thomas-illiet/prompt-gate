import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminUsersFilters from '../../app/components/AdminUsers/AdminUsersFilters.vue'

function mountFilters() {
  return mount(AdminUsersFilters, {
    props: {
      ipValidation: 'all',
      role: 'all',
      search: '',
      status: 'all',
    },
    global: {
      stubs: {
        AppFilterCard: { template: '<section><slot /></section>' },
        VSelect: {
          emits: ['update:modelValue'],
          props: ['items', 'label', 'modelValue'],
          template: `
            <section :data-test="'select-' + label">
              <button
                v-for="item in items"
                :key="item.value"
                :data-test="label + '-' + item.value"
                @click="$emit('update:modelValue', item.value)"
              >
                {{ item.title }}
              </button>
            </section>
          `,
        },
        VTextField: {
          inheritAttrs: false,
          props: ['modelValue'],
          template: '<input :value="modelValue" />',
        },
      },
    },
  })
}

describe('AdminUsersFilters', () => {
  it('emits both IP validation exception filters', async () => {
    const wrapper = mountFilters()
    await wrapper.get('[data-test="IP validation-true"]').trigger('click')
    await wrapper.get('[data-test="IP validation-false"]').trigger('click')

    expect(wrapper.emitted('update:ipValidation')).toEqual([
      ['true'],
      ['false'],
    ])
  })

  it('labels administrative state as access while preserving API filter values', async () => {
    const wrapper = mountFilters()

    expect(wrapper.get('[data-test="select-Access"]').text()).toContain(
      'All access',
    )
    expect(wrapper.get('[data-test="select-Access"]').text()).toContain(
      'Enabled',
    )
    expect(wrapper.get('[data-test="select-Access"]').text()).toContain(
      'Disabled',
    )

    await wrapper.get('[data-test="Access-inactive"]').trigger('click')

    expect(wrapper.emitted('update:status')).toEqual([['inactive']])
  })
})
