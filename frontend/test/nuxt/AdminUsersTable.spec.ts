import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import AdminUsersTable from '../../app/components/AdminUsers/AdminUsersTable.vue'
import type { AppRowAction } from '../../app/types/row-actions'
import type { AdminUser } from '../../app/types/users'

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

function mountTable() {
  return mount(AdminUsersTable, {
    props: {
      items: [user],
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

            function selectAction(action: AppRowAction<AdminUser>) {
              action.onSelect?.(props.item)
            }

            return { resolveTitle, selectAction }
          },
          template:
            '<div><button v-for="action in actions" :key="action.key" :data-test="\'row-action-\' + action.key" @click="selectAction(action)">{{ resolveTitle(action) }}</button></div>',
        }),
        AppSectionCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        AppServerDataTable: {
          props: ['items'],
          template:
            '<div><div v-for="item in items" :key="item.id"><slot name="item.name" :item="item" /><slot name="item.email" :item="item" /><slot name="item.actions" :item="item" /></div></div>',
        },
        VBtn: { template: '<button><slot /></button>' },
      },
    },
  })
}

describe('AdminUsersTable', () => {
  it('shows username and OIDC subject details to disambiguate duplicate identities', () => {
    const wrapper = mountTable()

    expect(wrapper.text()).toContain('@ada')
    expect(wrapper.text()).toContain('sub: oidc-sub')
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
})
