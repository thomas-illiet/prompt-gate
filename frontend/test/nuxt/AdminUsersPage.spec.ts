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
  lastLoginAt: '2026-01-02T00:00:00Z',
  inputTokens: 123,
  outputTokens: 456,
  expiresAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function createAdminUsersMock() {
  return {
    clearIPAddresses: vi.fn(),
    createdToken: shallowRef(null),
    createUserToken: vi.fn(),
    deleteUser: vi.fn(),
    exporting: shallowRef(false),
    exportUsers: vi.fn(),
    groupLoading: shallowRef(false),
    groupOptions: shallowRef([]),
    ipAddresses: shallowRef([]),
    ipLoading: shallowRef(false),
    ipPage: shallowRef(1),
    ipPageSize: shallowRef(10),
    ipSortBy: shallowRef('lastSeen'),
    ipSortDir: shallowRef<'asc' | 'desc'>('desc'),
    ipTotal: shallowRef(0),
    listError: shallowRef(null),
    loadGroups: vi.fn(),
    loadIPAddresses: vi.fn(),
    loadTokens: vi.fn(),
    loadUser: vi.fn(),
    loadUserGroups: vi.fn(),
    loading: shallowRef(false),
    page: shallowRef(1),
    pageSize: shallowRef(10),
    reload: vi.fn(),
    replaceUserGroups: vi.fn(),
    revokeUserToken: vi.fn(),
    role: shallowRef('all'),
    saving: shallowRef(false),
    search: shallowRef(''),
    selectedUser: shallowRef<AdminUser | null>(null),
    setIPPage: vi.fn(),
    setIPPageSize: vi.fn(),
    setIPSort: vi.fn(),
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    setRole: vi.fn(),
    setSearch: vi.fn(),
    setSort: vi.fn(),
    setStatus: vi.fn(),
    setTokenPage: vi.fn(),
    setTokenPageSize: vi.fn(),
    setTokenSort: vi.fn(),
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
  emits: ['manageIps', 'manageTokens', 'usageStatistics'],
  template: `
    <div>
      <button
        data-test="open-ips"
        @click="$emit('manageIps', items[0])"
      >
        IP addresses
      </button>
      <button
        data-test="open-usage"
        @click="$emit('usageStatistics', items[0])"
      >
        Usage statistics
      </button>
      <button
        data-test="open-tokens"
        @click="$emit('manageTokens', items[0])"
      >
        Virtual keys
      </button>
    </div>
  `,
})

const IPAddressesDialogStub = defineComponent({
  props: {
    accountName: String,
    modelValue: Boolean,
  },
  emits: ['update:modelValue'],
  template: `
    <section v-if="modelValue" data-test="ip-addresses-dialog">
      {{ accountName }} IP addresses
    </section>
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

const UserTokensDialogStub = defineComponent({
  props: ['modelValue'],
  emits: ['create'],
  template:
    '<button v-if="modelValue" data-test="create-user-token" @click="$emit(\'create\')">Create key</button>',
})

const TokenCreateDialogStub = defineComponent({
  props: ['modelValue'],
  emits: ['create'],
  template:
    '<button v-if="modelValue" data-test="submit-user-token" @click="$emit(\'create\', { name: \'admin_created\', description: \'Managed\' })">Submit key</button>',
})

function mountPage() {
  return mount(AdminUsersPage, {
    global: {
      stubs: {
        AdminAccountNoteDialog: true,
        AdminAccountIPAddressesDialog: IPAddressesDialogStub,
        AdminUserDeleteDialog: true,
        AdminUserEditDialog: true,
        AdminUserGroupsDialog: true,
        AdminUsersFilters: true,
        AdminUsersTable: UsersTableStub,
        AdminUserTokensDialog: UserTokensDialogStub,
        AdminUserUsageDialog: UsageDialogStub,
        AppConfirmDialog: true,
        AppPageHero: true,
        AppTokenCreateDialog: TokenCreateDialogStub,
        UserTokenCreatedDialog: true,
        VAlert: { template: '<div><slot /></div>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<main><slot /></main>' },
        VBtn: {
          emits: ['click'],
          template:
            '<button data-test="export-users" @click="$emit(\'click\')"><slot /></button>',
        },
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

  it('opens the IP addresses dialog from the table action', async () => {
    const adminUsers = createAdminUsersMock()
    useAdminUsersMock.mockReturnValue(adminUsers)
    const wrapper = mountPage()

    await wrapper.get('[data-test="open-ips"]').trigger('click')

    expect(wrapper.get('[data-test="ip-addresses-dialog"]').text()).toContain(
      'Ada Lovelace IP addresses',
    )
    expect(adminUsers.loadIPAddresses).toHaveBeenCalledWith(user.id)
  })

  it('exports users from the page action', async () => {
    const adminUsers = createAdminUsersMock()
    useAdminUsersMock.mockReturnValue(adminUsers)
    const wrapper = mountPage()

    await wrapper.get('[data-test="export-users"]').trigger('click')

    expect(adminUsers.exportUsers).toHaveBeenCalledOnce()
  })

  it('creates a token for the user selected from the table', async () => {
    const adminUsers = createAdminUsersMock()
    useAdminUsersMock.mockReturnValue(adminUsers)
    const wrapper = mountPage()

    await wrapper.get('[data-test="open-tokens"]').trigger('click')
    expect(adminUsers.loadTokens).toHaveBeenCalledWith(user.id)

    await wrapper.get('[data-test="create-user-token"]').trigger('click')
    await wrapper.get('[data-test="submit-user-token"]').trigger('click')

    expect(adminUsers.createUserToken).toHaveBeenCalledWith(user.id, {
      name: 'admin_created',
      description: 'Managed',
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
