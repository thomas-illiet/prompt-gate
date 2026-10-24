import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { defineComponent, shallowRef } from 'vue'

import AppMonitoringBanner from '../../app/components/App/AppMonitoringBanner.vue'
import DefaultLayout from '../../app/layouts/default.vue'
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
    authStore: { user: null as unknown },
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

function mountLayout() {
  return mount(DefaultLayout, {
    slots: { default: '<div data-test="page-content">Page content</div>' },
    global: {
      components: { AppMonitoringBanner },
      stubs: {
        AppBar: true,
        AppDrawer: true,
        AppFooter: true,
        QuotaExhaustedDialog: QuotaExhaustedDialogStub,
        VApp: { template: '<div><slot /></div>' },
        VMain: { template: '<main><slot /></main>' },
      },
    },
  })
}

describe('DefaultLayout', () => {
  beforeEach(() => {
    authStore.user = null
    apiFetch.mockReset()
    apiFetch.mockResolvedValue({ status: 'ok', services: [] })
    navigateToMock.mockReset()
    useApiFetchMock.mockClear()
    useQuotaExhaustionDialogMock.mockReset()
    useQuotaExhaustionDialogMock.mockReturnValue(createQuotaDialogMock())
  })

  it('does not mount or load monitoring for a limited user', async () => {
    authStore.user = authenticatedUser('limited')

    const wrapper = mountLayout()
    await flushPromises()

    expect(wrapper.findComponent(AppMonitoringBanner).exists()).toBe(false)
    expect(useApiFetchMock).not.toHaveBeenCalled()
    expect(apiFetch).not.toHaveBeenCalled()
    expect(navigateToMock).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="page-content"]').text()).toBe(
      'Page content',
    )
    wrapper.unmount()
  })

  it.each(['user', 'admin'] as const)(
    'keeps monitoring enabled for the %s role',
    async (role) => {
      authStore.user = authenticatedUser(role)

      const wrapper = mountLayout()
      await flushPromises()

      expect(wrapper.findComponent(AppMonitoringBanner).exists()).toBe(true)
      expect(apiFetch).toHaveBeenCalledWith('/api/v1/monitoring/status')
      expect(navigateToMock).not.toHaveBeenCalled()
      wrapper.unmount()
    },
  )

  it('forwards quota dialog state and actions through the authenticated layout', async () => {
    authStore.user = authenticatedUser('user')
    const quotaDialog = createQuotaDialogMock()
    quotaDialog.isOpen.value = true
    quotaDialog.canReset.value = true
    useQuotaExhaustionDialogMock.mockReturnValue(quotaDialog)

    const wrapper = mountLayout()
    await flushPromises()

    expect(wrapper.find('[data-test="quota-dialog"]').exists()).toBe(true)
    await wrapper.get('[data-test="quota-dialog-reset"]').trigger('click')
    expect(quotaDialog.useReset).toHaveBeenCalledOnce()

    await wrapper.get('[data-test="quota-dialog-close"]').trigger('click')
    expect(quotaDialog.isOpen.value).toBe(false)
    wrapper.unmount()
  })
})
