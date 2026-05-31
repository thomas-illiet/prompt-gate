import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { computed, shallowRef } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { UserTokenStatusFilter } from '../../app/types/user-service'
import TokensPage from '../../app/pages/tokens.vue'

const {
  createUserTokensState,
  useTargetDialogMock,
  useUserTokensMock,
} = vi.hoisted(() => {
  function createUserTokensState() {
    const search = shallowRef('')
    const statusFilter = shallowRef<UserTokenStatusFilter>('active')
    const total = shallowRef(3)

    return {
      createToken: vi.fn(),
      createdToken: shallowRef(null),
      listError: shallowRef<string | null>(null),
      loading: shallowRef(false),
      page: shallowRef(1),
      pageSize: shallowRef(10),
      reload: vi.fn(),
      revokeToken: vi.fn(),
      saving: shallowRef(false),
      search,
      setPage: vi.fn(),
      setPageSize: vi.fn(),
      setSearch: vi.fn((value: string) => {
        search.value = value
      }),
      setSort: vi.fn(),
      setStatusFilter: vi.fn((value: UserTokenStatusFilter) => {
        statusFilter.value = value

        if (value === 'active') {
          total.value = 3
          return
        }

        if (value === 'expired') {
          total.value = 1
          return
        }

        if (value === 'revoked') {
          total.value = 2
          return
        }

        total.value = 6
      }),
      sortBy: shallowRef('createdAt'),
      sortDir: shallowRef<'asc' | 'desc'>('desc'),
      statusFilter,
      tokenStats: computed(() => ({
        active: statusFilter.value === 'active' ? total.value : 0,
        all: total.value,
        expired: statusFilter.value === 'expired' ? total.value : 0,
        revoked: statusFilter.value === 'revoked' ? total.value : 0,
      })),
      tokens: shallowRef([]),
      total,
    }
  }

  return {
    createUserTokensState,
    useTargetDialogMock: vi.fn(() => ({
      close: vi.fn(),
      isOpen: shallowRef(false),
      open: vi.fn(),
      target: shallowRef(null),
    })),
    useUserTokensMock: vi.fn(),
  }
})

mockNuxtImport('useTargetDialog', () => useTargetDialogMock)
mockNuxtImport('useUserTokens', () => useUserTokensMock)

function mountPage() {
  return mount(TokensPage, {
    global: {
      stubs: {
        AppTokenCreateDialog: true,
        AppConfirmDialog: true,
        UserTokenCreatedDialog: true,
        UserTokenTable: true,
        UserTokenFilters: {
          props: ['search', 'statusFilter'],
          emits: ['update:search', 'update:status-filter'],
          template: `
            <div>
              <button
                data-test="set-expired"
                @click="$emit('update:status-filter', 'expired')"
              >
                expired
              </button>
              <button
                data-test="set-revoked"
                @click="$emit('update:status-filter', 'revoked')"
              >
                revoked
              </button>
              <button
                data-test="set-all"
                @click="$emit('update:status-filter', 'all')"
              >
                all
              </button>
            </div>
          `,
        },
        AppPageHero: {
          props: ['statValue'],
          template: '<div data-test="hero-stat">{{ statValue }}</div>',
        },
        VAlert: { template: '<div><slot /></div>' },
        VCol: { template: '<div><slot /></div>' },
        VContainer: { template: '<div><slot /></div>' },
        VRow: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('TokensPage', () => {
  beforeEach(() => {
    useTargetDialogMock.mockClear()
    useUserTokensMock.mockReset()
    useUserTokensMock.mockReturnValue(createUserTokensState())
  })

  it('updates the hero stat label when the selected status filter changes', async () => {
    const wrapper = mountPage()

    expect(wrapper.get('[data-test="hero-stat"]').text()).toBe('3 active')

    await wrapper.get('[data-test="set-expired"]').trigger('click')
    expect(wrapper.get('[data-test="hero-stat"]').text()).toBe('1 expired')

    await wrapper.get('[data-test="set-revoked"]').trigger('click')
    expect(wrapper.get('[data-test="hero-stat"]').text()).toBe('2 revoked')

    await wrapper.get('[data-test="set-all"]').trigger('click')
    expect(wrapper.get('[data-test="hero-stat"]').text()).toBe('6 all')
  })
})
