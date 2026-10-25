import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'

import AppUserMenu from '../../app/components/App/AppUserMenu.vue'
import { useAuthStore } from '../../app/stores/auth'

const { navigateToMock } = vi.hoisted(() => ({
  navigateToMock: vi.fn(),
}))

mockNuxtImport('navigateTo', () => navigateToMock)

const MenuStub = defineComponent({
  template: `
    <div>
      <slot name="activator" :props="{ 'aria-expanded': 'false' }" />
      <div data-test="user-menu-content"><slot /></div>
    </div>
  `,
})

const ButtonStub = defineComponent({
  template: `
    <button v-bind="$attrs" type="button">
      <slot name="prepend" />
      <slot />
      <slot name="append" />
    </button>
  `,
})

const ListItemStub = defineComponent({
  emits: ['click'],
  props: ['title'],
  template: `
    <button v-bind="$attrs" type="button" @click="$emit('click')">
      <slot name="prepend" />
      <slot />
      {{ title }}
      <slot name="append" />
    </button>
  `,
})

function mountMenu() {
  return mount(AppUserMenu, {
    global: {
      stubs: {
        VAvatar: { template: '<span><slot /></span>' },
        VBtn: ButtonStub,
        VCard: { template: '<section><slot /></section>' },
        VDivider: { template: '<hr />' },
        VIcon: true,
        VList: { template: '<div><slot /></div>' },
        VListItem: ListItemStub,
        VMenu: MenuStub,
      },
    },
  })
}

describe('AppUserMenu', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    navigateToMock.mockReset()

    const authStore = useAuthStore()
    authStore.user = {
      email: 'ada@example.com',
      id: 'user-id',
      isActive: true,
      lastLoginAt: '2026-10-23T00:00:00Z',
      name: 'Ada Lovelace',
      preferredUsername: 'ada',
      role: 'user',
      sub: 'oidc-subject',
    }
  })

  it('exposes the signed-in identity through an accessible trigger', () => {
    const wrapper = mountMenu()
    const trigger = wrapper.get('[data-test="user-menu-trigger"]')

    expect(trigger.attributes('aria-label')).toBe(
      'Open user menu for Ada Lovelace',
    )
    expect(trigger.text()).toContain('Ada Lovelace')
    expect(wrapper.get('[data-test="user-menu-content"]').text()).toContain(
      'ada@example.com',
    )
    expect(wrapper.text()).toContain('AL')
  })

  it('opens the profile from the themed menu action', async () => {
    const wrapper = mountMenu()

    await wrapper.get('[data-test="user-menu-profile"]').trigger('click')

    expect(navigateToMock).toHaveBeenCalledWith('/profile')
  })

  it('logs out through the auth store with the login redirect', async () => {
    const authStore = useAuthStore()
    const logout = vi.spyOn(authStore, 'logout').mockResolvedValue()
    const wrapper = mountMenu()

    await wrapper.get('[data-test="user-menu-logout"]').trigger('click')

    expect(logout).toHaveBeenCalledWith('/login')
  })
})
