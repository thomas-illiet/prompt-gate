import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, shallowRef } from 'vue'

import AppAuthenticatedShell from '../../app/components/App/AppAuthenticatedShell.vue'
import AppLimitedAccessBanner from '../../app/components/App/AppLimitedAccessBanner.vue'
import AppMonitoringBanner from '../../app/components/App/AppMonitoringBanner.vue'
import type { AppRole, AuthUser } from '../../app/types/auth'

const {
  apiFetch,
  authStore,
  navigateToMock,
  useApiFetchMock,
  useQuotaExhaustionDialogMock,
} = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return {
    apiFetch,
    authStore: {
      adminContactEmail: null as string | null,
      user: null as unknown,
    },
    navigateToMock: vi.fn(),
    useApiFetchMock: vi.fn(() => apiFetch),
    useQuotaExhaustionDialogMock: vi.fn(),
  }
})

mockNuxtImport('useAuthStore', () => () => authStore)
mockNuxtImport('useApiFetch', () => useApiFetchMock)
mockNuxtImport('navigateTo', () => navigateToMock)
mockNuxtImport('useQuotaExhaustionDialog', () => useQuotaExhaustionDialogMock)

function createQuotaDialogMock() {
  return {
    canReset: shallowRef(false),
    isOpen: shallowRef(false),
    quotaLoading: shallowRef(false),
    resetAvailable: shallowRef(0),
    resetError: shallowRef<string | null>(null),
    resetLoading: shallowRef(false),
    resetMaximum: shallowRef(2),
    resetPending: shallowRef(false),
    resetUsing: shallowRef(false),
    useReset: vi.fn(),
    windows: shallowRef([]),
  }
}

const QuotaExhaustedDialogStub = defineComponent({
  props: {
    canReset: Boolean,
    modelValue: Boolean,
    resetAvailable: Number,
    resetError: String,
    resetLoading: Boolean,
    resetMaximum: Number,
    resetPending: Boolean,
    resetUsing: Boolean,
    windows: Array,
  },
  emits: ['reset', 'update:modelValue'],
  template:
    '<section v-if="modelValue" data-test="quota-dialog"><button data-test="quota-dialog-reset" @click="$emit(\'reset\')">Reset</button><button data-test="quota-dialog-close" @click="$emit(\'update:modelValue\', false)">Close</button></section>',
})

const AppBarStub = defineComponent({
  props: { contextLabel: String },
  template: '<header data-test="app-bar" :data-context-label="contextLabel" />',
})

const AppDrawerStub = defineComponent({
  props: { mode: String },
  template: '<nav data-test="app-drawer" :data-mode="mode" />',
})

function authenticatedUser(role: AppRole): AuthUser {
  return {
    id: `${role}-id`,
    sub: `${role}-sub`,
    preferredUsername: `${role}-user`,
    email: `${role}@example.com`,
    name: `${role} user`,
    role,
    isActive: true,
    lastLoginAt: '2026-10-23T00:00:00Z',
  }
}

function mountShell(mode: 'admin' | 'user' = 'user') {
  return mount(AppAuthenticatedShell, {
    props: { mode },
    slots: { default: '<div data-test="page-content">Page content</div>' },
    global: {
      components: { AppLimitedAccessBanner, AppMonitoringBanner },
      stubs: {
        AppBar: AppBarStub,
        AppDrawer: AppDrawerStub,
        AppFooter: true,
        QuotaExhaustedDialog: QuotaExhaustedDialogStub,
        VAlert: { template: '<aside><slot /></aside>' },
        VApp: { template: '<div><slot /></div>' },
        VMain: { template: '<main><slot /></main>' },
      },
    },
  })
}

describe('AppAuthenticatedShell', () => {
  beforeEach(() => {
    authStore.adminContactEmail = null
    authStore.user = null
    apiFetch.mockReset()
    apiFetch.mockResolvedValue({ status: 'ok', services: [] })
    navigateToMock.mockReset()
    useApiFetchMock.mockClear()
    useQuotaExhaustionDialogMock.mockReset()
    useQuotaExhaustionDialogMock.mockReturnValue(createQuotaDialogMock())
  })

  it.each([
    ['user', 'Green room'],
    ['admin', 'Administration'],
  ] as const)('configures the %s navigation context', (mode, contextLabel) => {
    const wrapper = mountShell(mode)

    expect(
      wrapper.get('[data-test="app-drawer"]').attributes('data-mode'),
    ).toBe(mode)
    expect(
      wrapper.get('[data-test="app-bar"]').attributes('data-context-label'),
    ).toBe(contextLabel)
  })

  it('does not mount or load monitoring for a limited user', async () => {
    authStore.adminContactEmail = 'admin@example.com'
    authStore.user = authenticatedUser('limited')

    const wrapper = mountShell()
    await flushPromises()

    const banner = wrapper.findComponent(AppLimitedAccessBanner)
    expect(banner.exists()).toBe(true)
    expect(banner.props('contactEmail')).toBe('admin@example.com')
    expect(wrapper.findComponent(AppMonitoringBanner).exists()).toBe(false)
    expect(useApiFetchMock).not.toHaveBeenCalled()
    expect(apiFetch).not.toHaveBeenCalled()
    expect(navigateToMock).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="page-content"]').text()).toBe(
      'Page content',
    )
    wrapper.unmount()
  })

  it.each(['none', 'user', 'admin'] as const)(
    'does not show the limited-access banner for the %s role',
    async (role) => {
      authStore.user = authenticatedUser(role)

      const wrapper = mountShell()
      await flushPromises()

      expect(wrapper.findComponent(AppLimitedAccessBanner).exists()).toBe(false)
      wrapper.unmount()
    },
  )

  it('does not show the limited-access banner without a session user', async () => {
    const wrapper = mountShell()
    await flushPromises()

    expect(wrapper.findComponent(AppLimitedAccessBanner).exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not show the limited-access banner for an inactive limited user', async () => {
    authStore.user = {
      ...authenticatedUser('limited'),
      isActive: false,
    }

    const wrapper = mountShell()
    await flushPromises()

    expect(wrapper.findComponent(AppLimitedAccessBanner).exists()).toBe(false)
    wrapper.unmount()
  })

  it.each(['user', 'admin'] as const)(
    'keeps monitoring enabled for the %s role',
    async (role) => {
      authStore.user = authenticatedUser(role)

      const wrapper = mountShell()
      await flushPromises()

      expect(wrapper.findComponent(AppMonitoringBanner).exists()).toBe(true)
      expect(apiFetch).toHaveBeenCalledWith('/api/v1/monitoring/status')
      expect(navigateToMock).not.toHaveBeenCalled()
      wrapper.unmount()
    },
  )

  it('forwards quota dialog state and actions through the authenticated shell', async () => {
    authStore.user = authenticatedUser('user')
    const quotaDialog = createQuotaDialogMock()
    quotaDialog.isOpen.value = true
    quotaDialog.canReset.value = true
    useQuotaExhaustionDialogMock.mockReturnValue(quotaDialog)

    const wrapper = mountShell()
    await flushPromises()

    expect(wrapper.find('[data-test="quota-dialog"]').exists()).toBe(true)
    await wrapper.get('[data-test="quota-dialog-reset"]').trigger('click')
    expect(quotaDialog.useReset).toHaveBeenCalledOnce()

    await wrapper.get('[data-test="quota-dialog-close"]').trigger('click')
    expect(quotaDialog.isOpen.value).toBe(false)
    wrapper.unmount()
  })
})
