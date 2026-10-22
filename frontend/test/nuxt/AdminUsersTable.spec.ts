import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import AdminUsersTable from '../../app/components/AdminUsers/AdminUsersTable.vue'
import type { AppRowAction } from '../../app/types/row-actions'
import type { AdminUser } from '../../app/types/users'

const user: AdminUser = {
  activeInLast30Days: true,
  quotaResetCredits: 1,
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

function mountTable(item: AdminUser = user) {
  return mount(AdminUsersTable, {
    props: {
      items: [item],
      loading: false,
      page: 1,
      pageSize: 10,
      sortBy: 'lastLoginAt',
      sortDir: 'desc',
      total: 1,
    },
    global: {
      stubs: {
        AppRowActionMenu: defineComponent({
          props: {
            actions: {
              type: Array as PropType<AppRowAction<AdminUser>[]>,
              required: true,
            },
            item: {
              type: Object as PropType<AdminUser>,
              required: true,
            },
          },
          setup(props) {
            function resolveTitle(action: AppRowAction<AdminUser>) {
              return typeof action.title === 'function'
                ? action.title(props.item)
                : action.title
            }

            function isDisabled(action: AppRowAction<AdminUser>) {
              return typeof action.disabled === 'function'
                ? action.disabled(props.item)
                : Boolean(action.disabled)
            }

            function selectAction(action: AppRowAction<AdminUser>) {
              if (isDisabled(action)) return
              action.onSelect?.(props.item)
            }

            return { isDisabled, resolveTitle, selectAction }
          },
          template:
            '<div><button v-for="action in actions" :key="action.key" :data-test="\'row-action-\' + action.key" :disabled="isDisabled(action)" @click="selectAction(action)">{{ resolveTitle(action) }}</button></div>',
        }),
        AppSectionCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        AppServerDataTable: {
          props: ['headers', 'items'],
          template:
            '<div><span v-for="header in headers" :key="header.key" :data-header="header.title" :data-sortable="header.sortable">{{ header.title }}</span><div v-for="item in items" :key="item.id"><slot name="item.name" :item="item" /><slot name="item.email" :item="item" /><slot name="item.isActive" :item="item" /><slot name="item.activeInLast30Days" :item="item" /><slot name="item.actions" :item="item" /></div></div>',
        },
        AppStatusToggleButton: {
          props: ['active', 'activeLabel', 'inactiveLabel'],
          template:
            '<button data-test="access-toggle">{{ active ? activeLabel : inactiveLabel }}</button>',
        },
        VBtn: {
          emits: ['click'],
          template:
            '<button @click="$emit(\'click\', $event)"><slot /></button>',
        },
        VChip: { template: '<span><slot /></span>' },
      },
    },
  })
}

describe('AdminUsersTable', () => {
  it('emits create from the primary pre-create action', async () => {
    const wrapper = mountTable()

    await wrapper.get('[data-test="pre-create-user"]').trigger('click')

    expect(wrapper.emitted('create')).toEqual([[]])
  })

  it('exposes an IP addresses row action', async () => {
    const wrapper = mountTable()
    expect(wrapper.get('[data-test="row-action-manageIps"]').text()).toBe(
      'IP addresses',
    )
    await wrapper.get('[data-test="row-action-manageIps"]').trigger('click')
    expect(wrapper.emitted('manageIps')).toEqual([[user]])
  })

  it('shows username and OIDC subject details to disambiguate duplicate identities', () => {
    const wrapper = mountTable()

    expect(wrapper.text()).toContain('@ada')
    expect(wrapper.text()).toContain('sub: oidc-sub')
  })

  it('renders readable fallbacks for a profile that has not linked through SSO', () => {
    const wrapper = mountTable({
      ...user,
      sub: null,
      email: '',
      name: '',
      lastLoginAt: null,
    })

    expect(wrapper.text()).toContain('ada')
    expect(wrapper.text()).toContain('No email')
    expect(wrapper.text()).toContain('Not linked')
  })

  it('shows access and 30-day activity as independent states', () => {
    const recentlyActiveDisabledUser = {
      ...user,
      isActive: false,
      activeInLast30Days: true,
    }
    const wrapper = mountTable(recentlyActiveDisabledUser)

    expect(wrapper.get('[data-test="access-toggle"]').text()).toBe('Disabled')
    expect(wrapper.text()).toContain('Active (30d)')
    expect(
      wrapper
        .get('[data-header="Platform activity"]')
        .attributes('data-sortable'),
    ).toBe('false')

    const enabledInactiveWrapper = mountTable({
      ...user,
      isActive: true,
      activeInLast30Days: false,
    })
    expect(
      enabledInactiveWrapper.get('[data-test="access-toggle"]').text(),
    ).toBe('Enabled')
    expect(enabledInactiveWrapper.text()).toContain('Inactive (30d)')
  })

  it('exposes a manage-virtual-keys row action', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="row-action-tokens"]').text()).toBe(
      'Manage virtual keys',
    )

    await wrapper.get('[data-test="row-action-tokens"]').trigger('click')

    expect(wrapper.emitted('manageTokens')).toEqual([[user]])
  })

  it('exposes a manage-groups row action', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="row-action-groups"]').text()).toBe(
      'Manage groups',
    )

    await wrapper.get('[data-test="row-action-groups"]').trigger('click')

    expect(wrapper.emitted('manageGroups')).toEqual([[user]])
  })

  it('exposes a notes row action', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="row-action-notes"]').text()).toBe('Notes')

    await wrapper.get('[data-test="row-action-notes"]').trigger('click')

    expect(wrapper.emitted('notes')).toEqual([[user]])
  })

  it('emits the selected user from the usage-statistics action', async () => {
    const wrapper = mountTable()

    expect(wrapper.get('[data-test="row-action-usageStatistics"]').text()).toBe(
      'Usage statistics',
    )

    await wrapper
      .get('[data-test="row-action-usageStatistics"]')
      .trigger('click')

    expect(wrapper.emitted('usageStatistics')).toEqual([[user]])
  })

  it('offers one quota reset and includes the current balance in the label', async () => {
    const wrapper = mountTable()
    const action = wrapper.get('[data-test="row-action-offerQuotaReset"]')

    expect(action.text()).toBe('Offer quota reset (1/2)')
    expect(action.attributes('disabled')).toBeUndefined()

    await action.trigger('click')

    expect(wrapper.emitted('offerQuotaReset')).toEqual([[user]])
  })

  it('disables only the quota reset action when the balance is full', async () => {
    const fullBalanceUser = { ...user, quotaResetCredits: 2 }
    const wrapper = mountTable(fullBalanceUser)
    const action = wrapper.get('[data-test="row-action-offerQuotaReset"]')

    expect(action.text()).toBe('Offer quota reset (2/2)')
    expect(action.attributes('disabled')).toBe('')

    await action.trigger('click')

    expect(wrapper.emitted('offerQuotaReset')).toBeUndefined()
    expect(
      wrapper
        .get('[data-test="row-action-usageStatistics"]')
        .attributes('disabled'),
    ).toBeUndefined()
  })
})
