import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ProfilePage from '../../app/pages/profile.vue'
import { useAuthStore } from '../../app/stores/auth'
import type { ProfileTokenUsageSummary } from '../../app/composables/useProfileTokenUsage'

const {
  reloadProfileQuotaMock,
  reloadProfileUsageMock,
  useProfileGroupsMock,
  useProfileQuotaMock,
  useProfileTokenUsageMock,
} = vi.hoisted(() => {
  const reloadProfileQuotaMock = vi.fn()
  const reloadProfileUsageMock = vi.fn()

  return {
    reloadProfileQuotaMock,
    reloadProfileUsageMock,
    useProfileGroupsMock: vi.fn(() => ({
      error: { value: null },
      groups: { value: [] },
      loading: { value: false },
    })),
    useProfileQuotaMock: vi.fn(() => ({
      error: { value: null },
      loading: { value: false },
      quota: { value: null },
      reload: reloadProfileQuotaMock,
    })),
    useProfileTokenUsageMock: vi.fn(() => ({
      error: { value: null },
      loading: { value: false },
      reload: reloadProfileUsageMock,
      summary: {
        value: {
          activeDays: 1,
          currentStreakDays: 1,
          days: [],
          endsAt: '2026-06-03',
          longestStreakDays: 1,
          maxTokens: 42,
          peakDay: null,
          startsAt: '2025-06-04',
          totalTokens: 42,
        } satisfies ProfileTokenUsageSummary,
      },
    })),
  }
})

mockNuxtImport('useProfileGroups', () => useProfileGroupsMock)
mockNuxtImport('useProfileQuota', () => useProfileQuotaMock)
mockNuxtImport('useProfileTokenUsage', () => useProfileTokenUsageMock)

function mountPage() {
  return mount(ProfilePage, {
    global: {
      stubs: {
        ProfileGroupsCard: {
          template: '<div data-test="groups-card" />',
        },
        ProfileIdentityCard: {
          template: '<div data-test="identity-card" />',
        },
        ProfileInfoCard: {
          template: '<section><slot /></section>',
        },
        ProfileQuickActions: {
          emits: ['logout'],
          template: '<div data-test="quick-actions" />',
        },
        ProfileQuotaSection: {
          props: ['error', 'loading', 'quota'],
          emits: ['retry'],
          template:
            '<div data-test="profile-quota-section" :data-loading="loading" @click="$emit(\'retry\')" />',
        },
        ProfileTokenUsageSection: {
          props: ['error', 'loading', 'summary'],
          emits: ['retry'],
          template:
            '<div data-test="profile-token-section" :data-loading="loading" @click="$emit(\'retry\')">{{ summary.totalTokens }}</div>',
        },
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

    reloadProfileQuotaMock.mockClear()
    reloadProfileUsageMock.mockClear()
    useProfileGroupsMock.mockClear()
    useProfileQuotaMock.mockClear()
    useProfileTokenUsageMock.mockClear()
  })

  it('mounts the profile token usage section between existing profile cards', async () => {
    const wrapper = mountPage()

    expect(wrapper.find('[data-test="identity-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="quick-actions"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="profile-quota-section"]').exists()).toBe(
      true,
    )
    expect(wrapper.find('[data-test="profile-token-section"]').exists()).toBe(
      true,
    )
    expect(wrapper.get('[data-test="profile-token-section"]').text()).toBe('42')

    await wrapper.get('[data-test="profile-token-section"]').trigger('click')

    expect(reloadProfileUsageMock).toHaveBeenCalledTimes(1)
  })
})
