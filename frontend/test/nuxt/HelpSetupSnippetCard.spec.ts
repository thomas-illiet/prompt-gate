import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import HelpSetupSnippetCard from '../../app/components/HelpSetup/HelpSetupSnippetCard.vue'

const { notifySuccess } = vi.hoisted(() => ({
  notifySuccess: vi.fn(),
}))

vi.mock('~/stores/notification', () => ({
  Notify: {
    success: notifySuccess,
  },
}))

describe('HelpSetupSnippetCard', () => {
  beforeEach(() => {
    notifySuccess.mockClear()
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    })
  })

  it('copies the snippet to the clipboard', async () => {
    const wrapper = mount(HelpSetupSnippetCard, {
      props: {
        code: 'curl https://proxy.example.com',
        subtitle: 'Snippet',
        title: 'curl',
      },
      global: {
        stubs: {
          VBtn: {
            emits: ['click'],
            template:
              '<button aria-label="Copy snippet" @click="$emit(\'click\')"><slot /></button>',
          },
          VCard: { template: '<section><slot /></section>' },
          VCardItem: {
            template: '<header><slot /><slot name="append" /></header>',
          },
          VCardSubtitle: { template: '<p><slot /></p>' },
          VCardText: { template: '<main><slot /></main>' },
          VCardTitle: { template: '<h2><slot /></h2>' },
          VIcon: { template: '<span />' },
        },
      },
    })

    await wrapper.get('button[aria-label="Copy snippet"]').trigger('click')

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      'curl https://proxy.example.com',
    )
    expect(notifySuccess).toHaveBeenCalledWith('Snippet copied.')
  })

  it('copies slotted snippet content to the clipboard', async () => {
    const wrapper = mount(HelpSetupSnippetCard, {
      props: {
        subtitle: 'Snippet',
        title: 'Shell',
      },
      slots: {
        default:
          'export OPENAI_API_KEY=token<br />export OPENAI_BASE_URL=https://proxy.example.com',
      },
      global: {
        stubs: {
          VBtn: {
            emits: ['click'],
            template:
              '<button aria-label="Copy snippet" @click="$emit(\'click\')"><slot /></button>',
          },
          VCard: { template: '<section><slot /></section>' },
          VCardItem: {
            template: '<header><slot /><slot name="append" /></header>',
          },
          VCardSubtitle: { template: '<p><slot /></p>' },
          VCardText: { template: '<main><slot /></main>' },
          VCardTitle: { template: '<h2><slot /></h2>' },
          VIcon: { template: '<span />' },
        },
      },
    })

    await wrapper.get('button[aria-label="Copy snippet"]').trigger('click')

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      'export OPENAI_API_KEY=token\nexport OPENAI_BASE_URL=https://proxy.example.com',
    )
  })

  it('renders default config file paths', () => {
    const wrapper = mount(HelpSetupSnippetCard, {
      props: {
        code: 'schema: v1',
        filePaths: ['~/.continue/config.yaml'],
        subtitle: 'Snippet',
        title: 'Continue',
      },
      global: {
        stubs: {
          VBtn: { template: '<button><slot /></button>' },
          VCard: { template: '<section><slot /></section>' },
          VCardSubtitle: { template: '<p><slot /></p>' },
          VCardText: { template: '<main><slot /></main>' },
          VCardTitle: { template: '<h2><slot /></h2>' },
          VIcon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('Default file path')
    expect(wrapper.text()).toContain('~/.continue/config.yaml')
  })
})
