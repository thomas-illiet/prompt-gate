import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import AccessDeniedPage from '../../app/pages/access-denied.vue'
import { useAuthStore } from '../../app/stores/auth'

function mountPage() {
  return mount(AccessDeniedPage, {
    global: {
      stubs: {
        VAvatar: { template: '<div><slot /></div>' },
        VBtn: { template: '<button><slot /></button>' },
        VCard: { template: '<section><slot /></section>' },
        VCardText: { template: '<div><slot /></div>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<main><slot /></main>' },
        VIcon: true,
        VRow: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('AccessDeniedPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('describes the none role as explicit access removal', () => {
    const authStore = useAuthStore()
    authStore.user = {
      id: 'blocked-user',
      sub: 'blocked-subject',
      preferredUsername: 'blocked',
      email: 'blocked@example.com',
      name: 'Blocked user',
      role: 'none',
      isActive: true,
      lastLoginAt: '2026-10-23T00:00:00Z',
    }

    const wrapper = mountPage()

    expect(wrapper.text()).toContain('Your platform access has been removed.')
    expect(wrapper.text()).not.toContain('still needs an application role')
  })
})
