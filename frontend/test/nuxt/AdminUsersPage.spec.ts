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
  activeInLast30Days: true,
  quotaResetCredits: 1,
  type: 'user',
  id: 'user-id',
  sub: 'oidc-sub',
  preferredUsername: 'ada',
  email: 'ada@example.com',
  name: 'Ada Lovelace',
  role: 'user',
  subscriptionPlanId: null,
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
    createUser: vi.fn().mockResolvedValue(user),
    createUserToken: vi.fn(),
    deleteUser: vi.fn(),
    exporting: shallowRef(false),
    exportUsers: vi.fn(),
    groupLoading: shallowRef(false),
    groupOptions: shallowRef([]),
    grantQuotaReset: vi.fn(),
    ipAddresses: shallowRef([]),
    ipLoading: shallowRef(false),
    ipPage: shallowRef(1),
    ipPageSize: shallowRef(10),
    ipSortBy: shallowRef('lastSeen'),
    ipSortDir: shallowRef<'asc' | 'desc'>('desc'),
    ipTotal: shallowRef(0),
    listError: shallowRef(null),
    loadGroups: vi.fn(),
    loadIPAddresses: vi.fn().mockResolvedValue([]),
    loadTokens: vi.fn().mockResolvedValue([]),
    loadUser: vi.fn().mockResolvedValue(user),
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
  emits: [
    'create',
    'manageIps',
    'manageTokens',
    'offerQuotaReset',
    'usageStatistics',
  ],
  template: `
    <div>
      <button data-test="open-create-user" @click="$emit('create')">
        Pre-create user
      </button>
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
      <button
        data-test="offer-quota-reset"
        @click="$emit('offerQuotaReset', items[0])"
      >
        Offer quota reset
      </button>
    </div>
  `,
})

const CreateDialogStub = defineComponent({
  props: { modelValue: Boolean },
  emits: ['create', 'update:modelValue'],
  template: `
    <section v-if="modelValue" data-test="create-user-dialog">
      <button
        data-test="submit-create-user"
        @click="$emit('create', { preferredUsername: 'Alice.Example' })"
      >
        Create
      </button>
    </section>
  `,
})

const EditDialogStub = defineComponent({
  props: { modelValue: Boolean },
  emits: ['save', 'update:modelValue'],
  template:
    '<section v-if="modelValue" data-test="edit-user-dialog">Edit access</section>',
})

const ConfirmDialogStub = defineComponent({
  props: {
    confirmLabel: String,
    message: String,
    modelValue: Boolean,
    title: String,
  },
  emits: ['cancel', 'confirm', 'update:modelValue'],
  template: `
    <section v-if="modelValue" data-test="confirm-dialog">
      <h2>{{ title }}</h2>
      <p>{{ message }}</p>
      <button data-test="confirm-action" @click="$emit('confirm')">
        {{ confirmLabel }}
      </button>
    </section>
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
        AdminUserCreateDialog: CreateDialogStub,
        AdminUserDeleteDialog: true,
        AdminUserEditDialog: EditDialogStub,
        AdminUserGroupsDialog: true,
        AdminUsersFilters: true,
        AdminUsersTable: UsersTableStub,
        AdminUserTokensDialog: UserTokensDialogStub,
        AdminUserUsageDialog: UsageDialogStub,
        AppConfirmDialog: ConfirmDialogStub,
        AppPageHeader: true,
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

  it('pre-creates a user and opens access settings after success', async () => {
    const adminUsers = createAdminUsersMock()
    useAdminUsersMock.mockReturnValue(adminUsers)
    const loadAllPlans = vi.fn()
    useAdminSubscriptionsMock.mockReturnValue({
      loadAllPlans,
      plans: shallowRef([]),
    })
    const wrapper = mountPage()

    await wrapper.get('[data-test="open-create-user"]').trigger('click')
    await wrapper.get('[data-test="submit-create-user"]').trigger('click')

    await vi.waitFor(() => {
      expect(adminUsers.createUser).toHaveBeenCalledWith({
        preferredUsername: 'Alice.Example',
      })
      expect(wrapper.find('[data-test="create-user-dialog"]').exists()).toBe(
        false,
      )
      expect(wrapper.find('[data-test="edit-user-dialog"]').exists()).toBe(true)
    })
    expect(adminUsers.loadUser).toHaveBeenCalledWith(user.id)
    expect(loadAllPlans).toHaveBeenCalledOnce()
  })

  it('keeps the pre-create dialog open when creation fails', async () => {
    const adminUsers = createAdminUsersMock()
    adminUsers.createUser.mockRejectedValueOnce(new Error('conflict'))
    useAdminUsersMock.mockReturnValue(adminUsers)
    const wrapper = mountPage()

    await wrapper.get('[data-test="open-create-user"]').trigger('click')
    await wrapper.get('[data-test="submit-create-user"]').trigger('click')

    await vi.waitFor(() => {
      expect(wrapper.find('[data-test="create-user-dialog"]').exists()).toBe(
        true,
      )
    })
    expect(wrapper.find('[data-test="edit-user-dialog"]').exists()).toBe(false)
  })

  it('offers recovery without recreating the user when post-create loading fails', async () => {
    const adminUsers = createAdminUsersMock()
    adminUsers.loadUser
      .mockRejectedValueOnce(new Error('user reload interrupted'))
      .mockResolvedValueOnce(user)
    useAdminUsersMock.mockReturnValue(adminUsers)
    const loadAllPlans = vi.fn().mockResolvedValue(undefined)
    useAdminSubscriptionsMock.mockReturnValue({
      loadAllPlans,
      plans: shallowRef([]),
    })
    const wrapper = mountPage()

    await wrapper.get('[data-test="open-create-user"]').trigger('click')
    await wrapper.get('[data-test="submit-create-user"]').trigger('click')

    await vi.waitFor(() => {
      expect(wrapper.find('[data-test="create-user-dialog"]').exists()).toBe(
        false,
      )
      expect(
        wrapper.find('[data-test="created-user-setup-warning"]').exists(),
      ).toBe(true)
    })
    expect(wrapper.text()).toContain('The account already exists')
    expect(wrapper.find('[data-test="edit-user-dialog"]').exists()).toBe(false)
    expect(adminUsers.createUser).toHaveBeenCalledOnce()

    await wrapper.get('[data-test="retry-created-user-setup"]').trigger('click')

    await vi.waitFor(() => {
      expect(wrapper.find('[data-test="edit-user-dialog"]').exists()).toBe(true)
      expect(
        wrapper.find('[data-test="created-user-setup-warning"]').exists(),
      ).toBe(false)
    })
    expect(adminUsers.createUser).toHaveBeenCalledOnce()
    expect(adminUsers.loadUser).toHaveBeenCalledTimes(2)
    expect(loadAllPlans).toHaveBeenCalledTimes(2)
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

  it('confirms and grants a quota reset from the selected user row', async () => {
    const adminUsers = createAdminUsersMock()
    useAdminUsersMock.mockReturnValue(adminUsers)
    const wrapper = mountPage()

    await wrapper.get('[data-test="offer-quota-reset"]').trigger('click')

    const dialog = wrapper.get('[data-test="confirm-dialog"]')
    expect(dialog.text()).toContain('Offer quota reset')
    expect(dialog.text()).toContain('Ada Lovelace')
    expect(dialog.text()).toContain('2/2')

    await dialog.get('[data-test="confirm-action"]').trigger('click')

    expect(adminUsers.grantQuotaReset).toHaveBeenCalledWith(user.id)
    await vi.waitFor(() => {
      expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(false)
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
