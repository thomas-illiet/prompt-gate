import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminUserUsageDialog from '../../app/components/AdminUsers/AdminUserUsageDialog.vue'
import type { AdminUser } from '../../app/types/users'

const user: AdminUser = {
  activeInLast30Days: true,
  quotaResetCredits: 1,
  id: 'user/id',
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

function mountDialog() {
  return mount(AdminUserUsageDialog, {
    props: {
      modelValue: true,
      user,
    },
    global: {
      stubs: {
        AdminAccountUsageDialog: {
          props: [
            'costCaption',
            'emptyUsageText',
            'endpoint',
            'modelValue',
            'subtitle',
            'title',
          ],
          emits: ['update:modelValue'],
          template: `
            <section
              :data-cost-caption="costCaption"
              :data-empty-usage-text="emptyUsageText"
              :data-endpoint="endpoint"
              :data-subtitle="subtitle"
              :data-title="title"
            >
              <button
                data-test="close"
                @click="$emit('update:modelValue', false)"
              >
                Close
              </button>
            </section>
          `,
        },
      },
    },
  })
}

describe('AdminUserUsageDialog', () => {
  it('adapts a user into the shared usage dialog contract', () => {
    const wrapper = mountDialog()
    const dialog = wrapper.get('section')

    expect(dialog.attributes('data-title')).toBe(
      'Ada Lovelace usage statistics',
    )
    expect(dialog.attributes('data-endpoint')).toBe(
      '/api/v1/admin/users/user%2Fid/statistics',
    )
    expect(dialog.attributes('data-subtitle')).toBe(
      'Usage attributed to this user across the selected period.',
    )
    expect(dialog.attributes('data-cost-caption')).toBe('User usage estimate')
    expect(dialog.attributes('data-empty-usage-text')).toBe(
      'This user has no usage in the selected window.',
    )
  })

  it('forwards close events to its model', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="close"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })
})
