import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ProfileGroupsCard from '../../app/components/Profile/ProfileGroupsCard.vue'

describe('ProfileGroupsCard', () => {
  it('exposes a retry action when groups fail to load', async () => {
    const wrapper = mount(ProfileGroupsCard, {
      props: {
        error: 'Groups unavailable.',
        groups: [],
        loading: false,
      },
      global: {
        stubs: {
          ProfileInfoCard: {
            template: '<section><slot name="actions" /><slot /></section>',
          },
          VAlert: { template: '<div><slot /></div>' },
          VBtn: { template: '<button><slot /></button>' },
          VChip: { template: '<span><slot /></span>' },
        },
      },
    })

    await wrapper.get('[data-test="profile-groups-retry"]').trigger('click')

    expect(wrapper.emitted('retry')).toHaveLength(1)
  })
})
