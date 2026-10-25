import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it } from 'vitest'

import AdminActivityAdoptionKpis from '../../app/components/AdminDashboards/AdminActivityAdoptionKpis.vue'

const VAvatarStub = defineComponent({
  template: '<span data-test="avatar"><slot /></span>',
})

describe('AdminActivityAdoptionKpis', () => {
  it('renders the three adoption cards with the current period values', () => {
    const wrapper = mount(AdminActivityAdoptionKpis, {
      props: {
        adoption: {
          activeServiceAccounts: 23,
          activeUsers: 1_234,
          activeVirtualKeys: 45,
        },
      },
      global: {
        stubs: {
          VAvatar: VAvatarStub,
          VIcon: true,
        },
      },
    })

    const cards = wrapper.findAll('.admin-activity-adoption-kpis__card')
    expect(cards).toHaveLength(3)
    expect(cards.map((card) => card.text())).toEqual([
      'Active users1,234Used this period',
      'Active service accounts23Used this period',
      'Active virtual keys45Used this period',
    ])
    expect(wrapper.attributes('aria-busy')).toBe('false')
  })

  it('shows placeholders while the adoption summary is loading', () => {
    const wrapper = mount(AdminActivityAdoptionKpis, {
      props: { loading: true },
      global: {
        stubs: {
          VAvatar: VAvatarStub,
          VIcon: true,
        },
      },
    })

    expect(
      wrapper.findAll('.admin-activity-adoption-kpis__skeleton'),
    ).toHaveLength(3)
    expect(wrapper.text()).not.toContain('—')
    expect(wrapper.attributes('aria-busy')).toBe('true')
  })
})
