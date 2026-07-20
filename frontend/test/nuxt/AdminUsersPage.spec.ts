import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, shallowRef } from 'vue'

import AdminUsersPage from '../../app/pages/admin/users.vue'
import type { AdminUser } from '../../app/types/users'

const { useAdminSubscriptionsMock, useAdminUsersMock } = vi.hoisted(() => ({
  useAdminSubscriptionsMock: vi.fn(),
  useAdminUsersMock: vi.fn(),
}))

mockNuxtImport('useAdminSubscriptions', () => useAdminSubscriptionsMock)
mockNuxtImport('useAdminUsers', () => useAdminUsersMock)

const user: AdminUser = {
  id: 'user-id',
  sub: 'oidc-sub',
  preferredUsername: 'ada',
  email: 'ada@example.com',
  name: 'Ada Lovelace',
  role: 'user',
  note: '',
  isActive: true,
  firewallOverrideEnabled: false,
  lastLoginAt: '2026-01-02T00:00:00Z',
  inputTokens: 123,
  outputTokens: 456,
  expiresAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function createAdminUsersMock() {
  return {
    createFirewallRule: vi.fn(),
    deleteFirewallRule: vi.fn(),
    deleteUser: vi.fn(),
    firewallLoading: shallowRef(false),
    firewallPage: shallowRef(1),
    firewallPageSize: shallowRef(10),
    firewallRules: shallowRef([]),
    firewallSortBy: shallowRef('priority'),
    firewallSortDir: shallowRef<'asc' | 'desc'>('asc'),
    firewallTotal: shallowRef(0),
    groupLoading: shallowRef(false),
    groupOptions: shallowRef([]),
    listError: shallowRef(null),
    loadFirewallRules: vi.fn(),
    loadGroups: vi.fn(),
    loadTokens: vi.fn(),
    loadUser: vi.fn(),
    loadUserGroups: vi.fn(),
    loading: shallowRef(false),
    moveFirewallRulePriority: vi.fn(),
    nextFirewallPriority: shallowRef(1),
    page: shallowRef(1),
    pageSize: shallowRef(10),
    reload: vi.fn(),
    replaceUserGroups: vi.fn(),
    revokeUserToken: vi.fn(),
    role: shallowRef('all'),
    saving: shallowRef(false),
    search: shallowRef(''),
    selectedUser: shallowRef<AdminUser | null>(null),
    setFirewallPage: vi.fn(),
    setFirewallPageSize: vi.fn(),
    setFirewallSort: vi.fn(),
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    setRole: vi.fn(),
    setSearch: vi.fn(),
    setSort: vi.fn(),
    setStatus: vi.fn(),
    setTokenPage: vi.fn(),
    setTokenPageSize: vi.fn(),
    setTokenSort: vi.fn(),
    simulateFirewallIp: vi.fn(),
    sortBy: shallowRef('lastLoginAt'),
    sortDir: shallowRef<'asc' | 'desc'>('desc'),
    status: shallowRef('all'),
    tokenLoading: shallowRef(false),
    tokenPage: shallowRef(1),
    tokenPageSize: shallowRef(10),
    tokens: shallowRef([]),
    tokenSortBy: shallowRef('createdAt'),
    tokenSortDir: shallowRef<'asc' | 'desc'>('desc'),
    tokenTotal: shallowRef(0),
    total: shallowRef(1),
    updateFirewallRule: vi.fn(),
    updateUser: vi.fn(),
    updateUserAccess: vi.fn(),
    updateUserNote: vi.fn(),
    userGroups: shallowRef([]),
    users: shallowRef([user]),
  }
}

const UsersTableStub = defineComponent({
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
    modelValue: Boolean,
    user: {
      type: Object,
      required: true,
    },
  },
  emits: ['update:modelValue'],
  setup() {
    usageDialogMounts += 1
  },
  template: `
    <section v-if="modelValue" data-test="usage-dialog">
      <h2>{{ user.name }} usage statistics</h2>
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
  return mount(AdminUsersPage, {
    global: {
      stubs: {
        AdminAccountNoteDialog: true,
        AdminServiceAccountFirewallDialog: true,
        AdminUserDeleteDialog: true,
        AdminUserEditDialog: true,
        AdminUserGroupsDialog: true,
        AdminUsersFilters: true,
        AdminUsersTable: UsersTableStub,
        AdminUserTokensDialog: true,
        AdminUserUsageDialog: UsageDialogStub,
        AppConfirmDialog: true,
        AppPageHero: true,
        VAlert: { template: '<div><slot /></div>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<main><slot /></main>' },
        VRow: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('Admin users page usage statistics dialog', () => {
  beforeEach(() => {
    usageDialogMounts = 0
    useAdminUsersMock.mockReset()
    useAdminUsersMock.mockReturnValue(createAdminUsersMock())
    useAdminSubscriptionsMock.mockReset()
    useAdminSubscriptionsMock.mockReturnValue({
      loadAllPlans: vi.fn(),
      plans: shallowRef([]),
    })
  })

  it('mounts for the selected user and unmounts on every close', async () => {
    const wrapper = mountPage()

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(false)
    expect(usageDialogMounts).toBe(0)

    await wrapper.get('[data-test="open-usage"]').trigger('click')

    expect(wrapper.get('[data-test="usage-dialog"]').text()).toContain(
      'Ada Lovelace usage statistics',
    )
    expect(usageDialogMounts).toBe(1)

    await wrapper.get('[data-test="close-usage"]').trigger('click')

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(false)

    await wrapper.get('[data-test="open-usage"]').trigger('click')

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(true)
    expect(usageDialogMounts).toBe(2)
  })
})
