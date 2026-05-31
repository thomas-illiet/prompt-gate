import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminServiceAccountTokensDialog from '../../app/components/AdminServiceAccounts/AdminServiceAccountTokensDialog.vue'
import type {
  ServiceAccount,
  TokenResponse,
} from '../../app/types/service-accounts'

const account: ServiceAccount = {
  id: 'service-account-id',
  identifier: 'ci_runner',
  name: 'CI runner',
  role: 'user',
  isActive: true,
  firewallOverrideEnabled: false,
  inputTokens: 1234,
  outputTokens: 5678,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const activeToken: TokenResponse = {
  id: 'active-token',
  userId: account.id,
  name: 'ci_token',
  description: 'CI access',
  expiresAt: '2099-12-31T00:00:00Z',
  createdAt: '2026-01-01T00:00:00Z',
}

function mountDialog(tokens: TokenResponse[]) {
  return mount(AdminServiceAccountTokensDialog, {
    props: {
      account,
      loading: false,
      modelValue: true,
      page: 1,
      pageSize: 10,
      saving: false,
      showRevoked: false,
      sortBy: 'createdAt',
      sortDir: 'desc',
      tokens,
      total: tokens.length,
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
        AppTokenCreateForm: {
          template: '<form data-test="inline-create-form" />',
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
        VIcon: { template: '<i />' },
        VSpacer: { template: '<span />' },
        VSwitch: {
          emits: ['update:modelValue'],
          props: ['modelValue'],
          template:
            '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
        },
      },
    },
  })
}

describe('AdminServiceAccountTokensDialog', () => {
  it('opens token creation from a toolbar action instead of rendering an inline form', async () => {
    const wrapper = mountDialog([activeToken])

    expect(wrapper.find('[data-test="inline-create-form"]').exists()).toBe(
      false,
    )

    const newKeyButton = wrapper
      .findAll('button')
      .find((button) => button.text() === 'New key')

    expect(newKeyButton).toBeDefined()

    await newKeyButton?.trigger('click')

    expect(wrapper.emitted('create')).toEqual([[]])
  })
})
