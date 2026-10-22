import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, shallowRef } from 'vue'

import AdminServiceAccountsPage from '../../app/pages/admin/service-accounts.vue'
import type { ServiceAccount } from '../../app/types/service-accounts'

const { useAdminServiceAccountsMock, useAdminSubscriptionsMock } = vi.hoisted(
  () => ({
    useAdminServiceAccountsMock: vi.fn(),
    useAdminSubscriptionsMock: vi.fn(),
  }),
)

mockNuxtImport('useAdminServiceAccounts', () => useAdminServiceAccountsMock)
mockNuxtImport('useAdminSubscriptions', () => useAdminSubscriptionsMock)

const account: ServiceAccount = {
  id: 'service-account-id',
  identifier: 'ci_runner',
  name: 'CI runner',
  role: 'user',
  note: '',
  isActive: true,
  inputTokens: 123,
  outputTokens: 456,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function createAdminServiceAccountsMock() {
  return {
    accounts: shallowRef([account]),
    activeAccountsCount: shallowRef(1),
    createAccountWithSubscription: vi.fn(),
    createToken: vi.fn(),
    createdToken: shallowRef(null),
    deleteAccount: vi.fn(),
    ipAddresses: shallowRef([]),
    ipLoading: shallowRef(false),
    ipPage: shallowRef(1),
    ipPageSize: shallowRef(10),
    ipSortBy: shallowRef('lastSeen'),
    ipSortDir: shallowRef<'asc' | 'desc'>('desc'),
    ipTotal: shallowRef(0),
    listError: shallowRef(null),
    loadAccount: vi.fn(),
    loadIPAddresses: vi.fn(),
    loadTokens: vi.fn(),
    loading: shallowRef(false),
    page: shallowRef(1),
    pageSize: shallowRef(10),
    reload: vi.fn(),
    revokeToken: vi.fn(),
    saving: shallowRef(false),
    search: shallowRef(''),
    selectedAccount: shallowRef<ServiceAccount | null>(null),
    setIPPage: vi.fn(),
    setIPPageSize: vi.fn(),
    setIPSort: vi.fn(),
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    setSearch: vi.fn(),
    setSort: vi.fn(),
    setTokenPage: vi.fn(),
    setTokenPageSize: vi.fn(),
    setTokenSort: vi.fn(),
    sortBy: shallowRef('createdAt'),
    sortDir: shallowRef<'asc' | 'desc'>('desc'),
    tokenLoading: shallowRef(false),
    tokenPage: shallowRef(1),
    tokenPageSize: shallowRef(10),
    tokenSortBy: shallowRef('createdAt'),
    tokenSortDir: shallowRef<'asc' | 'desc'>('desc'),
    tokens: shallowRef([]),
    tokenTotal: shallowRef(0),
    total: shallowRef(1),
    updateAccount: vi.fn(),
    updateAccountNote: vi.fn(),
    updateAccountWithSubscription: vi.fn(),
  }
}

const ServiceAccountsTableStub = defineComponent({
  props: {
    items: {
      type: Array,
      required: true,
    },
  },
  emits: ['usageStatistics'],
  template: `
    <button
      data-test="open-usage"
      @click="$emit('usageStatistics', items[0])"
    >
      Usage statistics
    </button>
  `,
})

let usageDialogMounts = 0
const UsageDialogStub = defineComponent({
  props: {
    account: {
      type: Object,
      required: true,
    },
    modelValue: Boolean,
  },
  emits: ['update:modelValue'],
  setup() {
    usageDialogMounts += 1
  },
  template: `
    <section v-if="modelValue" data-test="usage-dialog">
      <h2>{{ account.name }} usage statistics</h2>
      <button
        data-test="close-usage"
        @click="$emit('update:modelValue', false)"
      >
        Close
      </button>
    </section>
  `,
})

function mountPage() {
  return mount(AdminServiceAccountsPage, {
    global: {
      stubs: {
        AdminAccountIPAddressesDialog: true,
        AdminAccountNoteDialog: true,
        AdminServiceAccountDialog: true,
        AdminServiceAccountsTable: ServiceAccountsTableStub,
        AdminServiceAccountTokenCreatedDialog: true,
        AdminServiceAccountTokensDialog: true,
        AdminServiceAccountUsageDialog: UsageDialogStub,
        AppConfirmDialog: true,
        AppFilterCard: { template: '<section><slot /></section>' },
        AppPageHero: true,
        AppTokenCreateDialog: true,
        VAlert: { template: '<div><slot /></div>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<main><slot /></main>' },
        VRow: { template: '<div><slot /></div>' },
        VTextField: true,
      },
    },
  })
}

describe('Admin service accounts page usage statistics dialog', () => {
  beforeEach(() => {
    usageDialogMounts = 0
    useAdminServiceAccountsMock.mockReset()
    useAdminServiceAccountsMock.mockReturnValue(
      createAdminServiceAccountsMock(),
    )
    useAdminSubscriptionsMock.mockReset()
    useAdminSubscriptionsMock.mockReturnValue({
      loadAllPlans: vi.fn(),
      plans: shallowRef([]),
    })
  })

  it('mounts for the selected account and unmounts on every close', async () => {
    const wrapper = mountPage()

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(false)
    expect(usageDialogMounts).toBe(0)

    await wrapper.get('[data-test="open-usage"]').trigger('click')

    expect(wrapper.get('[data-test="usage-dialog"]').text()).toContain(
      'CI runner usage statistics',
    )
    expect(usageDialogMounts).toBe(1)

    await wrapper.get('[data-test="close-usage"]').trigger('click')

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(false)

    await wrapper.get('[data-test="open-usage"]').trigger('click')

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(true)
    expect(usageDialogMounts).toBe(2)
  })
})
