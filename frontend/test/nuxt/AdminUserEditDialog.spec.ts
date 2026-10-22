import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminUserEditDialog from '../../app/components/AdminUsers/AdminUserEditDialog.vue'
import type { AdminUser } from '../../app/types/users'

const pendingUser: AdminUser = {
  activeInLast30Days: false,
  quotaResetCredits: 0,
  id: 'pending-user-id',
  sub: null,
  preferredUsername: 'alice',
  email: '',
  name: '',
  role: 'none',
  note: '',
  isActive: true,
  lastLoginAt: null,
  inputTokens: 0,
  outputTokens: 0,
  expiresAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

describe('AdminUserEditDialog', () => {
  it('renders pending SSO profile fallbacks and access terminology', () => {
    const wrapper = mount(AdminUserEditDialog, {
      props: {
        loading: false,
        modelValue: true,
        subscriptionPlans: [],
        user: pendingUser,
      },
      global: {
        stubs: {
          AppDialogActionButton: true,
          AppDialogCard: {
            props: ['modelValue'],
            template:
              '<section v-if="modelValue"><slot /><slot name="actions" /></section>',
          },
          AppDialogCloseButton: true,
          VChip: { template: '<span><slot /></span>' },
          VCol: { template: '<div><slot /></div>' },
          VList: { template: '<div><slot /></div>' },
          VListItem: {
            props: ['subtitle', 'title'],
            template: '<div>{{ title }}: {{ subtitle }}<slot name="append" /></div>',
          },
          VRow: { template: '<div><slot /></div>' },
          VSelect: true,
          VSheet: { template: '<section><slot /></section>' },
          VSwitch: {
            props: ['label'],
            template: '<label>{{ label }}</label>',
          },
        },
      },
    })

    expect(wrapper.text()).toContain('Email: No email')
    expect(wrapper.text()).toContain('OIDC subject: Not linked')
    expect(wrapper.text()).toContain('Access enabled')
    expect(wrapper.text()).toContain('Last login: Never')
  })
})
