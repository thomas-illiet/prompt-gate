import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, shallowRef } from 'vue'

import ProfilePage from '../../app/pages/profile.vue'
import { useAuthStore } from '../../app/stores/auth'

const { useProfileGroupsMock, useProfileQuotaResetsMock } = vi.hoisted(() => ({
  useProfileGroupsMock: vi.fn(),
  useProfileQuotaResetsMock: vi.fn(),
}))

mockNuxtImport('useProfileGroups', () => useProfileGroupsMock)
mockNuxtImport('useProfileQuotaResets', () => useProfileQuotaResetsMock)

function createProfileGroupsMock() {
  return {
    error: shallowRef<string | null>(null),
    groups: shallowRef([]),
    loading: shallowRef(false),
    reload: vi.fn(),
  }
}

function createProfileQuotaResetsMock() {
  const historyOpen = shallowRef(false)

  return {
    error: shallowRef(null),
    history: shallowRef([]),
    historyError: shallowRef(null),
    historyLoading: shallowRef(false),
    historyOpen,
    historyPage: shallowRef(1),
    historyPageSize: shallowRef(20),
    historyTotal: shallowRef(0),
    loadHistory: vi.fn(),
    loading: shallowRef(false),
    openHistory: vi.fn(() => {
      historyOpen.value = true
    }),
    reload: vi.fn(),
    setHistoryPage: vi.fn(),
    setHistoryPageSize: vi.fn(),
    summary: shallowRef({
      available: 1,
      maximum: 2,
      hasResettableQuota: true,
      pendingReset: false,
    }),
    useReset: vi.fn().mockResolvedValue({ status: 'applied' }),
    using: shallowRef(false),
  }
}

const QuotaResetCardStub = defineComponent({
  emits: ['showHistory', 'use'],
  template: `
    <section data-test="quota-reset-card">
      <button data-test="use-quota-reset" @click="$emit('use')">Use reset</button>
      <button data-test="show-quota-reset-history" @click="$emit('showHistory')">
        View history
      </button>
    </section>
  `,
})

const QuotaResetHistoryDialogStub = defineComponent({
  props: { modelValue: Boolean },
  emits: ['update:modelValue'],
  template:
    '<section v-if="modelValue" data-test="quota-reset-history-dialog">History</section>',
})

const ConfirmDialogStub = defineComponent({
  props: { message: String, modelValue: Boolean },
  emits: ['confirm', 'update:modelValue'],
  template: `
    <section v-if="modelValue">
      <p>{{ message }}</p>
      <button data-test="confirm-quota-reset" @click="$emit('confirm')">
        Confirm reset
      </button>
    </section>
  `,
})

const ProfileGroupsCardStub = defineComponent({
  props: ['error', 'groups', 'loading'],
  emits: ['retry'],
  template: `
    <div data-test="groups-card">
      <button data-test="groups-retry" @click="$emit('retry')">Retry</button>
    </div>
  `,
})

function mountPage() {
  return mount(ProfilePage, {
    global: {
      stubs: {
        AppConfirmDialog: ConfirmDialogStub,
        AppPageHero: {
          props: ['statValue', 'visual'],
          template:
            '<div data-test="profile-hero" :data-visual="visual">{{ statValue }}</div>',
        },
        ProfileGroupsCard: ProfileGroupsCardStub,
        ProfileIdentityCard: {
          props: ['roleLabel'],
          template: '<div data-test="identity-card" :data-role="roleLabel" />',
        },
        ProfileInfoCard: { template: '<section><slot /></section>' },
        ProfileQuotaResetCard: QuotaResetCardStub,
        ProfileQuotaResetHistoryDialog: QuotaResetHistoryDialogStub,
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
    useProfileGroupsMock.mockReset()
    useProfileGroupsMock.mockReturnValue(createProfileGroupsMock())
    useProfileQuotaResetsMock.mockReset()
    useProfileQuotaResetsMock.mockReturnValue(createProfileQuotaResetsMock())
  })

  it('keeps the profile focused on identity, groups, and technical access details', () => {
    const wrapper = mountPage()

    expect(wrapper.find('[data-test="identity-card"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="profile-hero"]').text()).toBe('Active')
    expect(
      wrapper.get('[data-test="profile-hero"]').attributes('data-visual'),
    ).toBe('identity')
    expect(wrapper.find('[data-test="groups-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="quota-reset-card"]').exists()).toBe(true)
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

    expect(
      wrapper.get('[data-test="identity-card"]').attributes('data-role'),
    ).toBe('Limited')
  })

  it('retries loading profile groups from the groups card', async () => {
    const groups = createProfileGroupsMock()
    groups.error.value = 'Groups unavailable.'
    useProfileGroupsMock.mockReturnValue(groups)
    const wrapper = mountPage()

    await wrapper.get('[data-test="groups-retry"]').trigger('click')

    expect(groups.reload).toHaveBeenCalledOnce()
  })

  it('opens quota reset history from the dedicated profile card', async () => {
    const quotaResets = createProfileQuotaResetsMock()
    useProfileQuotaResetsMock.mockReturnValue(quotaResets)
    const wrapper = mountPage()

    await wrapper.get('[data-test="show-quota-reset-history"]').trigger('click')

    expect(quotaResets.openHistory).toHaveBeenCalledOnce()
    expect(
      wrapper.find('[data-test="quota-reset-history-dialog"]').exists(),
    ).toBe(true)
  })

  it('asks for confirmation before consuming a quota reset', async () => {
    const quotaResets = createProfileQuotaResetsMock()
    useProfileQuotaResetsMock.mockReturnValue(quotaResets)
    const wrapper = mountPage()

    await wrapper.get('[data-test="use-quota-reset"]').trigger('click')

    expect(quotaResets.useReset).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain(
      'This clears the live 5-hour and 7-day quota windows while preserving usage analytics.',
    )
    await wrapper.get('[data-test="confirm-quota-reset"]').trigger('click')

    await vi.waitFor(() => expect(quotaResets.useReset).toHaveBeenCalledOnce())
    expect(wrapper.find('[data-test="confirm-quota-reset"]').exists()).toBe(
      false,
    )
  })
})
