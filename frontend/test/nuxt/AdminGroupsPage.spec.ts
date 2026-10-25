import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, shallowRef } from 'vue'

import AdminGroupsPage from '../../app/pages/admin/groups.vue'
import type { AccessGroup } from '../../app/types/groups'

const { useAdminGroupsMock } = vi.hoisted(() => ({
  useAdminGroupsMock: vi.fn(),
}))

mockNuxtImport('useAdminGroups', () => useAdminGroupsMock)

const group: AccessGroup = {
  id: 'group-id',
  name: 'engineering',
  displayName: 'Engineering',
  description: 'Engineering access',
  providers: [],
  modelPatterns: ['^gpt-5'],
  excludedModelPatterns: [],
  members: [],
  providerCount: 0,
  modelPatternCount: 1,
  memberCount: 0,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function createAdminGroupsMock() {
  return {
    addMember: vi.fn(),
    clearModelValidation: vi.fn(),
    createGroup: vi.fn(),
    deleteGroup: vi.fn(),
    groups: shallowRef([group]),
    listError: shallowRef(null),
    loadGroup: vi.fn(),
    loadMemberOptions: vi.fn(),
    loadProviderOptions: vi.fn(),
    loading: shallowRef(false),
    memberLoading: shallowRef(false),
    memberOptions: shallowRef([]),
    modelValidation: shallowRef(null),
    modelValidationError: shallowRef(null),
    modelValidationLoading: shallowRef(false),
    page: shallowRef(1),
    pageSize: shallowRef(10),
    providerOptions: shallowRef([]),
    reload: vi.fn(),
    removeMember: vi.fn(),
    saving: shallowRef(false),
    search: shallowRef(''),
    selectedGroup: shallowRef<AccessGroup | null>(null),
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    setSearch: vi.fn(),
    setSort: vi.fn(),
    sortBy: shallowRef('name'),
    sortDir: shallowRef<'asc' | 'desc'>('asc'),
    total: shallowRef(1),
    updateGroup: vi.fn(),
    validateModelPatterns: vi.fn(),
  }
}

const GroupsTableStub = defineComponent({
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
    group: {
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
      <h2>{{ group.displayName }} usage statistics</h2>
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
  return mount(AdminGroupsPage, {
    global: {
      stubs: {
        AdminGroupDialog: true,
        AdminGroupMembersDialog: true,
        AdminGroupsFilters: true,
        AdminGroupsTable: GroupsTableStub,
        AdminGroupUsageDialog: UsageDialogStub,
        AppConfirmDialog: true,
        AppPageHeader: true,
        VAlert: { template: '<div><slot /></div>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<main><slot /></main>' },
        VRow: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('Admin groups page usage statistics dialog', () => {
  beforeEach(() => {
    usageDialogMounts = 0
    useAdminGroupsMock.mockReset()
    useAdminGroupsMock.mockReturnValue(createAdminGroupsMock())
  })

  it('mounts for the selected group and unmounts on every close', async () => {
    const wrapper = mountPage()

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(false)
    expect(usageDialogMounts).toBe(0)

    await wrapper.get('[data-test="open-usage"]').trigger('click')

    expect(wrapper.get('[data-test="usage-dialog"]').text()).toContain(
      'Engineering usage statistics',
    )
    expect(usageDialogMounts).toBe(1)

    await wrapper.get('[data-test="close-usage"]').trigger('click')

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(false)

    await wrapper.get('[data-test="open-usage"]').trigger('click')

    expect(wrapper.find('[data-test="usage-dialog"]').exists()).toBe(true)
    expect(usageDialogMounts).toBe(2)
  })
})
