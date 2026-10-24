import { mount } from '@vue/test-utils'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ErrorPage from '../../app/error.vue'

const { clearErrorMock } = vi.hoisted(() => ({
  clearErrorMock: vi.fn(),
}))

mockNuxtImport('clearError', () => clearErrorMock)

function mountPage(statusCode: number, message = 'private backend detail') {
  return mount(ErrorPage, {
    props: {
      error: {
        fatal: true,
        message,
        name: 'NuxtError',
        statusCode,
        statusMessage: message,
        toJSON: () => ({ message, statusCode, statusMessage: message }),
        unhandled: false,
      },
    },
    global: {
      stubs: {
        NuxtLayout: { template: '<div><slot /></div>' },
        VBtn: { template: '<button><slot /></button>' },
        VContainer: { template: '<main><slot /></main>' },
      },
    },
  })
}

describe('ErrorPage', () => {
  beforeEach(() => {
    clearErrorMock.mockReset()
    window.history.replaceState({}, '', '/')
  })

  it('renders a safe server message and retries the current route', async () => {
    window.history.replaceState({}, '', '/admin/users?window=7d')
    const wrapper = mountPage(500)

    expect(wrapper.text()).toContain('Something went off course')
    expect(wrapper.text()).not.toContain('private backend detail')

    await wrapper.get('button').trigger('click')

    expect(clearErrorMock).toHaveBeenCalledWith({
      redirect: '/admin/users?window=7d',
    })
  })

  it('routes a forbidden request back to the dashboard', async () => {
    const wrapper = mountPage(403)

    expect(wrapper.text()).toContain('Access denied')
    await wrapper.get('button').trigger('click')

    expect(clearErrorMock).toHaveBeenCalledWith({ redirect: '/dashboard' })
  })

  it('preserves the current route when a 401 requires a new sign-in', async () => {
    window.history.replaceState({}, '', '/tokens?tab=active')
    const wrapper = mountPage(401)

    expect(wrapper.text()).toContain('Session expired')
    await wrapper.get('button').trigger('click')

    expect(clearErrorMock).toHaveBeenCalledWith({
      redirect: '/login?redirect=%2Ftokens%3Ftab%3Dactive',
    })
  })
})
