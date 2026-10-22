import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminServiceAccountUsageDialog from '../../app/components/AdminServiceAccounts/AdminServiceAccountUsageDialog.vue'
import type { ServiceAccount } from '../../app/types/service-accounts'

const account: ServiceAccount = {
  id: 'service/account',
  identifier: 'ci_runner',
  name: 'CI runner',
  role: 'user',
  note: '',
  isActive: true,
  inputTokens: 123,
  outputTokens: 456,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function mountDialog() {
  return mount(AdminServiceAccountUsageDialog, {
    props: {
      account,
      modelValue: true,
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

describe('AdminServiceAccountUsageDialog', () => {
  it('adapts a service account into the shared usage dialog contract', () => {
    const wrapper = mountDialog()
    const dialog = wrapper.get('section')

    expect(dialog.attributes('data-title')).toBe('CI runner usage statistics')
    expect(dialog.attributes('data-endpoint')).toBe(
      '/api/v1/admin/service-accounts/service%2Faccount/statistics',
    )
    expect(dialog.attributes('data-subtitle')).toBe(
      'Usage attributed to this service account across the selected period.',
    )
    expect(dialog.attributes('data-cost-caption')).toBe(
      'Service account usage estimate',
    )
    expect(dialog.attributes('data-empty-usage-text')).toBe(
      'This service account has no usage in the selected window.',
    )
  })

  it('forwards close events to its model', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="close"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[false]])
  })
})
