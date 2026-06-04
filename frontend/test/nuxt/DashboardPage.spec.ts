import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardPage from '../../app/pages/dashboard.vue'

const {
  authUser,
  initializeMock,
  refreshDashboardMock,
  useAuthStoreMock,
  useDashboardRefreshMock,
  waitUntilReadyMock,
} = vi.hoisted(() => {
  const authUser = {
    value: null as null | { role: string },
  }
  const initializeMock = vi.fn(async () => undefined)
  const refreshDashboardMock = vi.fn()
  const waitUntilReadyMock = vi.fn(async () => undefined)

  return {
    authUser,
    initializeMock,
    refreshDashboardMock,
    useAuthStoreMock: vi.fn(() => ({
      initialize: initializeMock,
      waitUntilReady: waitUntilReadyMock,
      isAuthenticated: true,
      get user() {
        return authUser.value
      },
    })),
    useDashboardRefreshMock: vi.fn(() => ({
      refresh: refreshDashboardMock,
      version: { value: 0 },
    })),
    waitUntilReadyMock,
  }
})

mockNuxtImport('useAuthStore', () => useAuthStoreMock)
mockNuxtImport('useDashboardRefresh', () => useDashboardRefreshMock)

function mountPage() {
  return mount(DashboardPage, {
    global: {
      stubs: {
        DashboardActivityChart: {
          props: ['scope', 'window'],
          template:
            '<div data-test="activity" :data-scope="scope" :data-window="window" />',
        },
        DashboardAdoptionKpis: {
          props: ['window'],
          template: '<div data-test="adoption" :data-window="window" />',
        },
        DashboardDurationKpi: {
          props: ['scope', 'window'],
          template:
            '<div data-test="duration" :data-scope="scope" :data-window="window" />',
        },
        DashboardMessagesKpi: {
          props: ['scope', 'window'],
          template:
            '<div data-test="messages" :data-scope="scope" :data-window="window" />',
        },
        DashboardScopeSelect: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: `
            <button
              data-test="scope-select"
              @click="$emit('update:modelValue', 'global')"
            >
              {{ modelValue }}
            </button>
          `,
        },
        DashboardTimeRangeSelect: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<div data-test="window-select">{{ modelValue }}</div>',
        },
        DashboardTokensKpi: {
          props: ['scope', 'window'],
          template:
            '<div data-test="tokens" :data-scope="scope" :data-window="window" />',
        },
        DashboardUsageCostKpi: {
          props: ['scope', 'window'],
          template:
            '<div data-test="usage-cost" :data-scope="scope" :data-window="window" />',
        },
        DashboardTopIdentitiesChart: {
          props: ['window'],
          template: '<div data-test="top-identities" :data-window="window" />',
        },
        DashboardTopModelsChart: {
          props: ['scope', 'window'],
          template:
            '<div data-test="top-models" :data-scope="scope" :data-window="window" />',
        },
        DashboardTopProviderNamesChart: true,
        DashboardTopProviderTypesChart: true,
        VBtn: {
          emits: ['click'],
          template:
            '<button data-test="dashboard-refresh" @click="$emit(\'click\')"><slot /></button>',
        },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<div><slot /></div>' },
        VRow: { template: '<div><slot /></div>' },
        VTooltip: {
          template:
            '<span><slot name="activator" :props="{}" /><slot /></span>',
        },
      },
      directives: {
        tooltip: {},
      },
    },
  })
}

describe('DashboardPage', () => {
  beforeEach(() => {
    authUser.value = null
    initializeMock.mockClear()
    refreshDashboardMock.mockClear()
    useAuthStoreMock.mockClear()
    useDashboardRefreshMock.mockClear()
    waitUntilReadyMock.mockClear()
  })

  it('shows scope control for admins and defaults to my usage', () => {
    authUser.value = { role: 'admin' }

    const wrapper = mountPage()

    expect(wrapper.find('[data-test="scope-select"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="scope-select"]').text()).toBe('self')
    expect(wrapper.get('[data-test="tokens"]').attributes('data-scope')).toBe(
      'self',
    )
    expect(
      wrapper.get('[data-test="usage-cost"]').attributes('data-scope'),
    ).toBe('self')
    expect(wrapper.find('[data-test="adoption"]').exists()).toBe(false)
  })

  it('shows global admin KPIs when admins select global scope', async () => {
    authUser.value = { role: 'admin' }
    const wrapper = mountPage()

    await wrapper.get('[data-test="scope-select"]').trigger('click')

    expect(wrapper.get('[data-test="tokens"]').attributes('data-scope')).toBe(
      'global',
    )
    expect(
      wrapper.get('[data-test="usage-cost"]').attributes('data-scope'),
    ).toBe('global')
    expect(wrapper.find('[data-test="adoption"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="top-identities"]').exists()).toBe(true)
  })

  it('hides scope control and global KPIs for non-admin users', () => {
    authUser.value = { role: 'user' }

    const wrapper = mountPage()

    expect(wrapper.find('[data-test="scope-select"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="tokens"]').attributes('data-scope')).toBe(
      'self',
    )
    expect(
      wrapper.get('[data-test="usage-cost"]').attributes('data-scope'),
    ).toBe('self')
    expect(wrapper.find('[data-test="adoption"]').exists()).toBe(false)
  })

  it('refreshes dashboard widgets from the header control', async () => {
    authUser.value = { role: 'user' }

    const wrapper = mountPage()
    const refreshButton = wrapper.get('[data-test="dashboard-refresh"]')

    expect(refreshButton.text()).toContain('Refresh')

    await refreshButton.trigger('click')

    expect(refreshDashboardMock).toHaveBeenCalledTimes(1)
  })
})
