import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it } from 'vitest'

import AdminEmailComposer from '../../app/components/AdminNotifications/AdminEmailComposer.vue'

function mountComposer(
  props: Partial<InstanceType<typeof AdminEmailComposer>['$props']> = {},
) {
  return mount(AdminEmailComposer, {
    props: {
      audience: null,
      groups: [],
      loadingOptions: false,
      preview: null,
      previewing: false,
      resolving: false,
      saving: false,
      users: [],
      ...props,
    },
    global: {
      stubs: {
        AdminEmailAudiencePanel: {
          name: 'AdminEmailAudiencePanel',
          props: [
            'audience',
            'canResolve',
            'externalAddress',
            'groupIds',
            'mode',
            'userIds',
          ],
          emits: [
            'resolve',
            'update:externalAddress',
            'update:groupIds',
            'update:mode',
            'update:userIds',
          ],
          template:
            '<button data-test="resolve" type="button" @click="$emit(\'resolve\')">Resolve</button>',
        },
        AdminEmailReviewPanel: {
          name: 'AdminEmailReviewPanel',
          props: ['audience', 'canSend', 'preview', 'saving', 'subject'],
          emits: ['edit', 'send'],
          template:
            '<div data-test="review"><button data-test="edit" type="button" @click="$emit(\'edit\')">Edit</button></div>',
        },
        AppSectionCard: { template: '<section><slot /></section>' },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: {
          props: ['disabled', 'loading'],
          emits: ['click'],
          template:
            '<button data-test="continue" type="button" :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
        },
        VDivider: true,
        VIcon: true,
        VTextField: true,
        VTextarea: true,
      },
    },
  })
}

describe('AdminEmailComposer', () => {
  it('lets the audience be checked before message content is written', async () => {
    const wrapper = mountComposer()
    const audiencePanel = wrapper.getComponent({
      name: 'AdminEmailAudiencePanel',
    })

    expect(audiencePanel.props('canResolve')).toBe(false)

    await audiencePanel.vm.$emit('update:userIds', ['user-1'])
    await nextTick()

    expect(audiencePanel.props('canResolve')).toBe(true)
    await audiencePanel.vm.$emit('resolve')

    expect(wrapper.emitted('resolve')).toEqual([
      [
        {
          externalAddress: '',
          groupIds: [],
          mode: 'users',
          userIds: ['user-1'],
        },
      ],
    ])
  })

  it('invalidates only the audience when its selection changes', async () => {
    const wrapper = mountComposer()
    const audiencePanel = wrapper.getComponent({
      name: 'AdminEmailAudiencePanel',
    })

    await audiencePanel.vm.$emit('update:mode', 'all')
    await nextTick()

    expect(wrapper.emitted('invalidate:audience')).toHaveLength(1)
    expect(wrapper.emitted('invalidate:preview')).toBeUndefined()
  })

  it('shows the preview only after composing and requesting confirmation', async () => {
    const audience = {
      recipients: [{ userId: 'user-1', email: 'ada@example.com', name: 'Ada' }],
      valid: 1,
      ignored: 0,
    }
    const wrapper = mountComposer({ audience })

    expect(wrapper.find('[data-test="review"]').exists()).toBe(false)

    await wrapper
      .getComponent({ name: 'VTextField' })
      .vm.$emit('update:modelValue', 'Maintenance completed')
    await wrapper
      .getComponent({ name: 'VTextarea' })
      .vm.$emit('update:modelValue', '# Maintenance completed')
    await nextTick()

    await wrapper.get('[data-test="continue"]').trigger('click')

    expect(wrapper.emitted('preview')).toEqual([
      [
        {
          subject: 'Maintenance completed',
          markdown: '# Maintenance completed',
        },
      ],
    ])
    expect(wrapper.find('[data-test="review"]').exists()).toBe(false)

    await wrapper.setProps({
      preview: {
        htmlBody: '<h1>Maintenance completed</h1>',
        textBody: 'Maintenance completed',
      },
    })
    await nextTick()

    const review = wrapper.getComponent({ name: 'AdminEmailReviewPanel' })
    expect(review.props('subject')).toBe('Maintenance completed')
    expect(review.props('audience')).toEqual(audience)
    expect(wrapper.findComponent({ name: 'VTextField' }).exists()).toBe(false)

    await wrapper.get('[data-test="edit"]').trigger('click')
    expect(wrapper.findComponent({ name: 'VTextField' }).exists()).toBe(true)
  })
})
