import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { computed, defineComponent, type PropType } from 'vue'

import PromptHistoryTable from '../../app/components/PromptHistory/PromptHistoryTable.vue'
import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '../../app/types/user-service'

type PromptHistoryTableItem = PromptHistoryItem | AdminPromptHistoryItem

const prompt: PromptHistoryItem = {
  id: 'prompt-id',
  interceptionId: 'interception-id',
  providerResponseId: 'response-id',
  provider: 'openai',
  providerType: 'openai',
  model: 'gpt-5',
  prompt: 'Alpha prompt',
  inputTokens: 10,
  outputTokens: 20,
  totalTokens: 30,
  durationMs: 1250,
  createdAt: '2026-01-01T00:00:00Z',
}

const adminPrompt: AdminPromptHistoryItem = {
  ...prompt,
  id: 'admin-prompt-id',
  userId: 'user-id',
  userName: '',
  userEmail: 'ada@example.com',
  userPreferredUsername: 'ada',
  clientIp: '198.51.100.7',
}

const longMultilinePrompt: PromptHistoryItem = {
  ...prompt,
  id: 'long-multiline-prompt-id',
  prompt: `First line
${'second segment '.repeat(24)}
Third line`,
}

function mountTable(options: {
  items: PromptHistoryTableItem[]
  showUser?: boolean
}) {
  return mount(PromptHistoryTable, {
    props: {
      items: options.items,
      loading: false,
      page: 1,
      pageSize: 10,
      showUser: options.showUser ?? false,
      sortBy: 'createdAt',
      sortDir: 'desc',
      total: options.items.length,
    },
    global: {
      stubs: {
        AppEmptyState: { template: '<section />' },
        AppSectionCard: {
          template: '<section><slot name="actions" /><slot /></section>',
        },
        AppServerDataTable: defineComponent({
          props: {
            headers: {
              type: Array as PropType<{ title: string; key: string }[]>,
              required: true,
            },
            items: {
              type: Array as PropType<PromptHistoryTableItem[]>,
              required: true,
            },
          },
          setup(props) {
            const headerTitles = computed(() =>
              props.headers.map((header) => header.title).join('|'),
            )

            return { headerTitles }
          },
          template: `
            <section>
              <span data-test="headers">{{ headerTitles }}</span>
              <div v-for="item in items" :key="item.id">
                <slot name="item.prompt" :item="item" />
                <slot name="item.userName" :item="item" />
                <slot name="item.actions" :item="item" />
              </div>
            </section>
          `,
        }),
        PromptHistoryRequestGraphDialog: {
          props: ['actorLabel', 'modelValue', 'prompt'],
          template: `
            <section v-if="modelValue" data-test="request-dialog">
              <span data-test="dialog-actor">{{ actorLabel }}</span>
              <span data-test="dialog-provider">{{ prompt.provider }}</span>
              <span data-test="dialog-model">{{ prompt.model }}</span>
            </section>
          `,
        },
        VBtn: {
          emits: ['click'],
          props: ['icon'],
          template:
            '<button type="button" :aria-label="$attrs[\'aria-label\']" :data-icon="icon" @click="$emit(\'click\')"><slot /></button>',
        },
        VChip: { template: '<span><slot /></span>' },
        VTooltip: {
          props: ['text'],
          template:
            '<span><slot name="activator" :props="{}" /><span data-test="tooltip">{{ text }}</span></span>',
        },
      },
    },
  })
}

describe('PromptHistoryTable', () => {
  it('renders long multiline prompts as compact previews', () => {
    const wrapper = mountTable({ items: [longMultilinePrompt] })
    const preview = wrapper.get('[data-test="prompt-preview"]').text()

    expect(preview).toContain('First line second segment')
    expect(preview).not.toContain('\n')
    expect(preview.length).toBeLessThanOrEqual(223)
    expect(preview.endsWith('...')).toBe(true)
  })

  it('renders a user request graph action', async () => {
    const wrapper = mountTable({ items: [prompt] })

    expect(wrapper.get('[data-test="headers"]').text()).toContain('Actions')
    expect(
      wrapper.get('[aria-label="View request graph"]').attributes(),
    ).toMatchObject({
      'data-icon': 'mdi-transit-connection-variant',
    })

    await wrapper.get('[aria-label="View request graph"]').trigger('click')

    expect(wrapper.get('[data-test="dialog-actor"]').text()).toBe('You')
    expect(wrapper.get('[data-test="dialog-provider"]').text()).toBe('openai')
    expect(wrapper.get('[data-test="dialog-model"]').text()).toBe('gpt-5')
  })

  it('uses admin user identity in the request graph action', async () => {
    const wrapper = mountTable({ items: [adminPrompt], showUser: true })

    expect(wrapper.get('[data-test="headers"]').text()).not.toContain(
      'Client IP',
    )

    await wrapper.get('[aria-label="View request graph"]').trigger('click')

    expect(wrapper.get('[data-test="dialog-actor"]').text()).toBe('ada')
  })
})
