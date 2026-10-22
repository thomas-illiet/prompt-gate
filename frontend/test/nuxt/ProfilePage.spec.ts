import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ProfilePage from '../../app/pages/profile.vue'
import { useAuthStore } from '../../app/stores/auth'

const { useProfileGroupsMock } = vi.hoisted(() => ({
  useProfileGroupsMock: vi.fn(() => ({
    error: { value: null },
    groups: { value: [] },
    loading: { value: false },
  })),
}))

mockNuxtImport('useProfileGroups', () => useProfileGroupsMock)

function mountPage() {
  return mount(ProfilePage, {
    global: {
      stubs: {
        ProfileGroupsCard: {
          props: ['groups', 'loading'],
          template: '<div data-test="groups-card" />',
        },
        ProfileIdentityCard: {
          props: ['roleLabel'],
          template: '<div data-test="identity-card" :data-role="roleLabel" />',
        },
        ProfileInfoCard: { template: '<section><slot /></section>' },
        VAvatar: { template: '<span><slot /></span>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<div><slot /></div>' },
        VIcon: true,
        VRow: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('ProfilePage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    const authStore = useAuthStore()
    authStore.user = {
      email: 'ada@example.com',
      id: 'user-id',
      isActive: true,
      lastLoginAt: '2026-06-03T08:00:00Z',
      name: 'Ada Lovelace',
      preferredUsername: 'ada',
      role: 'user',
      sub: 'oidc-subject',
    }
    useProfileGroupsMock.mockClear()
  })

  it('keeps the profile focused on identity, groups, and technical access details', () => {
    const wrapper = mountPage()

    expect(wrapper.find('[data-test="identity-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="groups-card"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('User ID')
    expect(wrapper.text()).toContain('OIDC subject')
    expect(wrapper.text()).not.toContain('Subscription quota')
    expect(wrapper.text()).not.toContain('Token activity')
    expect(wrapper.find('[data-test="quick-actions"]').exists()).toBe(false)
  })

  it('preserves the limited role in the identity card', () => {
    const authStore = useAuthStore()
    authStore.user = { ...authStore.user!, role: 'limited' }

    const wrapper = mountPage()

    expect(wrapper.get('[data-test="identity-card"]').attributes('data-role')).toBe(
      'Limited',
    )
  })
})
