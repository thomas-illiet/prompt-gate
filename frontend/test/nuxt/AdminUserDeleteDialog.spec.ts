import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminUserDeleteDialog from '../../app/components/AdminUsers/AdminUserDeleteDialog.vue'
import type { AdminUser } from '../../app/types/users'

const user: AdminUser = {
  id: 'user-id',
  sub: 'oidc-subject',
  preferredUsername: 'alice',
  email: 'alice@example.com',
  name: 'Alice',
  type: 'user',
  role: 'limited',
  subscriptionPlanId: null,
  quotaResetCredits: 0,
  note: '',
  isActive: true,
  activeInLast30Days: false,
  inputTokens: 0,
  outputTokens: 0,
  expiresAt: null,
  lastLoginAt: null,
  createdAt: '2026-10-23T00:00:00Z',
  updatedAt: '2026-10-23T00:00:00Z',
}

describe('AdminUserDeleteDialog', () => {
  it('warns that deletion does not block OIDC reprovisioning', () => {
    const wrapper = mount(AdminUserDeleteDialog, {
      props: {
        loading: false,
        modelValue: true,
        user,
      },
      global: {
        stubs: {
          AppDialogCard: {
            props: ['modelValue', 'subtitle'],
            template:
              '<section v-if="modelValue"><p>{{ subtitle }}</p><slot /><slot name="actions" /></section>',
          },
          AppDialogActionButton: true,
          AppDialogCloseButton: true,
          VAlert: {
            template: '<aside><slot /></aside>',
          },
          VChip: true,
          VList: {
            template: '<div><slot /></div>',
          },
          VListItem: true,
          VSheet: {
            template: '<div><slot /></div>',
          },
          VTextField: true,
        },
      },
    })

    expect(wrapper.text()).toContain('This does not block the OIDC identity.')
    expect(wrapper.text()).toContain(
      'A later sign-in will create a new limited account.',
    )
    expect(wrapper.text()).toContain('assign role none')
  })
})
