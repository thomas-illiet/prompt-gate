import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminUserTokensDialog from '../../app/components/AdminUsers/AdminUserTokensDialog.vue'
import type { UserToken } from '../../app/types/user-service'
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

const activeToken: UserToken = {
  id: 'active-token',
  userId: user.id,
  name: 'active_cli',
  description: 'Active access',
  expiresAt: '2099-12-31T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
}

const revokedToken: UserToken = {
  ...activeToken,
  id: 'revoked-token',
  name: 'revoked_cli',
  revokedAt: '2026-02-01T00:00:00Z',
}

const expiredToken: UserToken = {
  ...activeToken,
  id: 'expired-token',
  name: 'expired_cli',
  expiresAt: '2020-01-01T00:00:00Z',
}

function mountDialog(tokens: UserToken[]) {
  return mount(AdminUserTokensDialog, {
    props: {
      loading: false,
      modelValue: true,
      page: 1,
      pageSize: 10,
      saving: false,
      sortBy: 'createdAt',
      sortDir: 'desc',
      tokens,
      total: tokens.length,
      user,
    },
    global: {
      stubs: {
        AppDialogCloseButton: {
          template: '<button type="button">Close</button>',
        },
        AppServerDataTable: {
          props: ['items'],
          template:
            '<div><div v-for="item in items" :key="item.id"><slot name="item.actions" :item="item" /></div></div>',
        },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          emits: ['click'],
          props: ['disabled', 'loading'],
          template:
            '<button type="button" :disabled="disabled || loading" @click="$emit(\'click\')"><slot /></button>',
        },
        VCard: { template: '<section><slot /></section>' },
        VCardActions: { template: '<div><slot /></div>' },
        VCardItem: {
          template: '<div><slot name="prepend" /><slot /></div>',
        },
        VCardSubtitle: { template: '<p><slot /></p>' },
        VCardText: { template: '<div><slot /></div>' },
        VCardTitle: { template: '<h2><slot /></h2>' },
        VChip: { template: '<span><slot /></span>' },
        VDialog: {
          props: ['modelValue'],
          template: '<div v-if="modelValue"><slot /></div>',
        },
        VEmptyState: { template: '<div />' },
        VIcon: { template: '<i />' },
        VSpacer: { template: '<span />' },
      },
    },
  })
}

describe('AdminUserTokensDialog', () => {
  it('emits revoke for active tokens and disables inactive token actions', async () => {
    const wrapper = mountDialog([activeToken, revokedToken, expiredToken])
    const revokeButtons = wrapper
      .findAll('button')
      .filter((button) => button.text() === 'Revoke')

    expect(revokeButtons).toHaveLength(3)
    expect(revokeButtons[0]?.attributes('disabled')).toBeUndefined()
    expect(revokeButtons[1]?.attributes('disabled')).toBe('')
    expect(revokeButtons[2]?.attributes('disabled')).toBe('')

    await revokeButtons[0]?.trigger('click')

    expect(wrapper.emitted('revoke')).toEqual([[activeToken]])
  })
})
