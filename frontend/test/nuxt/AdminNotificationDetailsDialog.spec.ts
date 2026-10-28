import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it } from 'vitest'

import AdminNotificationDetailsDialog from '../../app/components/AdminNotifications/AdminNotificationDetailsDialog.vue'
import type {
  NotificationDelivery,
  NotificationMessageDetail,
} from '../../app/types/notifications'

const sentDelivery: NotificationDelivery = {
  attempts: 1,
  id: 'delivery-sent',
  lastAttemptAt: '2026-10-28T10:00:00Z',
  lastError: '',
  nextAttemptAt: '',
  recipientEmail: 'sent@example.test',
  recipientName: 'Sent User',
  sentAt: '2026-10-28T10:00:00Z',
  status: 'sent',
  userId: 'user-sent',
}

const failedDelivery: NotificationDelivery = {
  attempts: 8,
  id: 'delivery-failed',
  lastAttemptAt: '2026-10-28T10:00:00Z',
  lastError: 'SMTP 550 recipient rejected',
  nextAttemptAt: '',
  recipientEmail: 'failed@example.test',
  recipientName: 'Failed User',
  sentAt: null,
  status: 'failed',
  userId: 'user-failed',
}

const message: NotificationMessageDetail = {
  audience: 'users',
  createdAt: '2026-10-28T10:00:00Z',
  deliveries: [sentDelivery, failedDelivery],
  failed: 1,
  htmlBody: '<p>Hello</p>',
  id: 'message-1',
  kind: 'manual',
  markdownBody: 'Hello',
  pending: 0,
  processing: 0,
  recipients: 2,
  sent: 1,
  subject: 'Operational update',
  textBody: 'Hello',
}

function mountDialog() {
  return mount(AdminNotificationDetailsDialog, {
    props: {
      loading: false,
      message,
      modelValue: true,
      retrying: false,
    },
    global: {
      stubs: {
        AppDialogActionButton: {
          name: 'AppDialogActionButton',
          props: ['label'],
          template: '<button>{{ label }}</button>',
        },
        AppDialogCard: {
          template:
            '<section><slot /><footer><slot name="actions" /></footer></section>',
        },
        AppDialogCloseButton: true,
        VBadge: true,
        VBtn: { template: '<button><slot /></button>' },
        VBtnToggle: { template: '<div><slot /></div>' },
        VChip: { template: '<span><slot /></span>' },
        VDataTable: {
          name: 'VDataTable',
          props: ['headers', 'items'],
          template: '<div />',
        },
        VSkeletonLoader: true,
        VTab: { template: '<button><slot /></button>' },
        VTabs: {
          name: 'VTabs',
          props: ['modelValue'],
          template: '<div><slot /></div>',
        },
        VWindow: {
          name: 'VWindow',
          props: ['modelValue'],
          template: '<div><slot /></div>',
        },
        VWindowItem: { template: '<div><slot /></div>' },
      },
    },
  })
}

describe('AdminNotificationDetailsDialog', () => {
  it('opens delivery diagnostics first and prioritizes failures', async () => {
    const wrapper = mountDialog()
    await nextTick()

    expect(wrapper.getComponent({ name: 'VWindow' }).props('modelValue')).toBe(
      'deliveries',
    )
    expect(
      wrapper.getComponent({ name: 'VDataTable' }).props('items')[0].status,
    ).toBe('failed')
  })

  it('offers one retry action with the failed delivery count', () => {
    const wrapper = mountDialog()

    expect(
      wrapper.getComponent({ name: 'AppDialogActionButton' }).props('label'),
    ).toBe('Retry 1 failed')
  })

  it('isolates the stored email document in a sandboxed client preview', () => {
    const wrapper = mountDialog()
    const preview = wrapper.get('iframe[title="Stored email preview"]')

    expect(preview.attributes('srcdoc')).toBe(message.htmlBody)
    expect(preview.attributes('sandbox')).toBe('')
  })
})
