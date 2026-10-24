import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { createVuetify } from 'vuetify'

import AppNotification from '../../app/components/App/AppNotification.vue'
import { useNotificationStore } from '../../app/stores/notification'

function mountNotifications() {
  const vuetify = createVuetify()

  return mount(AppNotification, {
    global: {
      plugins: [vuetify],
      directives: {
        tooltip: {},
      },
      stubs: {
        AppEmptyState: {
          props: ['title', 'text'],
          template:
            '<section data-test="notification-empty"><strong>{{ title }}</strong><span>{{ text }}</span></section>',
        },
        AppNotificationItem: {
          props: ['notification'],
          template:
            '<article data-test="notification-item">{{ notification.text }}</article>',
        },
        ClientOnly: { template: '<div><slot /></div>' },
        teleport: true,
        VBtn: {
          emits: ['click'],
          template:
            '<button v-bind="$attrs" type="button" @click="$emit(\'click\')"><slot /></button>',
        },
        VCard: {
          template: '<section v-bind="$attrs"><slot /></section>',
        },
        VSlideYReverseTransition: {
          template: '<div><slot /></div>',
        },
        VToolbar: { template: '<header><slot /></header>' },
        VToolbarTitle: {
          props: ['text'],
          template: '<h2>{{ text }}</h2>',
        },
      },
    },
  })
}

describe('AppNotification', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('opens an accessible empty control-room state', async () => {
    const wrapper = mountNotifications()
    const trigger = wrapper.get('[data-test="notification-trigger"]')
    const panel = wrapper.get('[data-test="notification-panel"]')

    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(trigger.attributes('aria-label')).toBe('Show notifications')
    expect(trigger.attributes('aria-controls')).toBe(panel.attributes('id'))
    expect(panel.attributes('aria-hidden')).toBe('true')

    await trigger.trigger('click')

    expect(trigger.attributes('aria-expanded')).toBe('true')
    const expandedPanel = wrapper.get('[data-test="notification-panel"]')
    expect(expandedPanel.attributes('aria-hidden')).toBe('false')
    expect(expandedPanel.attributes('role')).toBe('region')
    expect(wrapper.get('[data-test="notification-empty"]').text()).toContain(
      'Nothing on the wire',
    )
  })

  it('announces the count and clears the current notifications', async () => {
    const store = useNotificationStore()
    store.addNotification('Quota signal', 'warning')
    store.addNotification('Provider restored', 'success')
    const wrapper = mountNotifications()
    const trigger = wrapper.get('[data-test="notification-trigger"]')
    const panel = wrapper.get('[data-test="notification-panel"]')

    expect(trigger.attributes('aria-label')).toBe('Show 2 notifications')
    expect(panel.attributes('aria-hidden')).toBe('false')
    await trigger.trigger('click')

    expect(wrapper.text()).toContain('2 notifications')
    expect(wrapper.findAll('[data-test="notification-item"]')).toHaveLength(2)

    await wrapper.get('[data-test="notification-clear"]').trigger('click')

    expect(store.notifications).toHaveLength(0)
    expect(wrapper.get('[data-test="notification-empty"]').text()).toContain(
      'New control-room signals will land here.',
    )
  })
})
