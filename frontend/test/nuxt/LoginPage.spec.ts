import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it } from 'vitest'

import LoginPage from '../../app/pages/login.vue'
import { useAuthStore } from '../../app/stores/auth'

mockNuxtImport('useRoute', () => () => ({ query: {} }))

function mountPage() {
  return mount(LoginPage, {
    global: {
      stubs: {
        VAlert: { template: '<div><slot /><slot name="append" /></div>' },
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: { template: '<button><slot /></button>' },
        VCard: { template: '<section><slot /></section>' },
        VCardText: { template: '<div><slot /></div>' },
        VContainer: { template: '<main><slot /></main>' },
        VIcon: true,
      },
    },
  })
}

describe('LoginPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('does not claim that the control room is online before auth is ready', async () => {
    const authStore = useAuthStore()
    const wrapper = mountPage()
    const status = wrapper.get('[data-test="login-visual-status"]')

    expect(status.attributes('data-tone')).toBe('checking')
    expect(status.text()).toContain('Checking control room')

    authStore.isReady = true
    await nextTick()

    expect(status.attributes('data-tone')).toBe('online')
    expect(status.text()).toContain('Control room online')

    authStore.initializationError = 'Private transport detail'
    await nextTick()

    expect(status.attributes('data-tone')).toBe('unavailable')
    expect(status.text()).toContain('Control room unavailable')
    expect(status.text()).not.toContain('Private transport detail')
  })

  it('uses the generated gateway artwork as a decorative login visual', () => {
    const wrapper = mountPage()
    const image = wrapper.get('[data-test="login-visual-image"]')

    expect(image.attributes('alt')).toBe('')
    expect(image.attributes('src')).toContain('login-gateway.jpg')
  })
})
