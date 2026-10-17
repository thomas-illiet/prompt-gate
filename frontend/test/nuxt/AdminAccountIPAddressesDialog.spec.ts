import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminAccountIPAddressesDialog from '../../app/components/AdminAccounts/AdminAccountIPAddressesDialog.vue'

describe('AdminAccountIPAddressesDialog', () => {
  it('renders addresses and forwards table interactions', async () => {
    const wrapper = mount(AdminAccountIPAddressesDialog, {
      props: {
        modelValue: true,
        accountName: 'Ada',
        items: [
          {
            ip: '2001:db8::1',
            lastSeen: '2026-09-15T08:00:00Z',
            lastDeniedReason: 'network_not_allowed',
            lastDeniedAt: '2026-09-15T07:55:00Z',
          },
          {
            ip: '192.0.2.5',
            lastSeen: '2026-09-15T07:00:00Z',
            lastDeniedReason: null,
            lastDeniedAt: null,
          },
        ],
        loading: false,
        page: 1,
        pageSize: 10,
        sortBy: 'lastSeen',
        sortDir: 'desc',
        total: 1,
      },
      global: {
        stubs: {
          AppDialogCard: {
            template: '<section><slot /><slot name="actions" /></section>',
          },
          AppDialogCloseButton: {
            template:
              '<button data-test="close" @click="$emit(\'click\')">Close</button>',
          },
          AppServerDataTable: {
            props: ['items'],
            emits: ['update:page', 'update:page-size', 'update:sort'],
            template:
              '<div><button data-test="page" @click="$emit(\'update:page\', 2)" /><div v-for="item in items" :key="item.ip"><slot name="item.ip" :item="item" /><slot name="item.lastSeen" :item="item" /><slot name="item.lastDeniedAt" :item="item" /><slot name="item.lastDeniedReason" :item="item" /></div></div>',
          },
          VBtn: {
            template:
              "<button :data-test=\"$attrs['prepend-icon'] === 'mdi-refresh' ? 'refresh' : 'clear'\" @click=\"$emit('click')\"><slot /></button>",
          },
          VChip: { template: '<span><slot /></span>' },
        },
      },
    })

    expect(wrapper.text()).toContain('2001:db8::1')
    expect(wrapper.text()).toContain('Sep 15, 2026')
    expect(wrapper.text()).toContain('network_not_allowed')
    expect(wrapper.text()).toContain('—')
    await wrapper.get('[data-test="refresh"]').trigger('click')
    await wrapper.get('[data-test="clear"]').trigger('click')
    await wrapper.get('[data-test="page"]').trigger('click')
    await wrapper.get('[data-test="close"]').trigger('click')
    expect(wrapper.emitted('refresh')?.length).toBeGreaterThanOrEqual(1)
    expect(wrapper.emitted('clear')?.length).toBeGreaterThanOrEqual(1)
    expect(wrapper.emitted('update:page')).toEqual([[2]])
    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })
})
