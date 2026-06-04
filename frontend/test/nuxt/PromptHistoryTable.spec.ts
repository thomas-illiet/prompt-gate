import { mount, type DOMWrapper } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { computed, defineComponent, type PropType } from 'vue'

import PromptHistoryTable from '../../app/components/PromptHistory/PromptHistoryTable.vue'
import type {
  AdminPromptHistoryItem,
  PromptHistoryItem,
} from '../../app/types/user-service'

type PromptHistoryTableItem = PromptHistoryItem | AdminPromptHistoryItem
const localStorageValues = new Map<string, string>()
const localStorageMock = {
  get length() {
    return localStorageValues.size
  },
  clear() {
    localStorageValues.clear()
  },
  getItem(key: string) {
    return localStorageValues.get(key) ?? null
  },
  key(index: number) {
    return [...localStorageValues.keys()][index] ?? null
  },
  removeItem(key: string) {
    localStorageValues.delete(key)
  },
  setItem(key: string, value: string) {
    localStorageValues.set(key, value)
  },
} satisfies Storage

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
  columnPreferencesKey?: string
  enableColumnPicker?: boolean
  items: PromptHistoryTableItem[]
  showUser?: boolean
  sortBy?: string
}) {
  return mount(PromptHistoryTable, {
    props: {
      columnPreferencesKey:
        options.columnPreferencesKey ?? 'prompt-history-table-test-columns',
      enableColumnPicker: options.enableColumnPicker ?? true,
      items: options.items,
      loading: false,
      page: 1,
      pageSize: 10,
      showUser: options.showUser ?? false,
      sortBy: options.sortBy ?? 'createdAt',
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
            const headerKeys = computed(() =>
              props.headers.map((header) => header.key).join('|'),
            )

            return { headerKeys, headerTitles }
          },
          template: `
            <section>
              <span data-test="headers">{{ headerTitles }}</span>
              <span data-test="header-keys">{{ headerKeys }}</span>
              <div v-for="item in items" :key="item.id">
                <slot
                  v-for="header in headers"
                  :key="header.key"
                  :name="'item.' + header.key"
                  :item="item"
                />
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
          props: ['icon', 'prependIcon'],
          template:
            '<button type="button" :aria-label="$attrs[\'aria-label\']" :data-icon="icon || prependIcon" @click="$emit(\'click\')"><slot /></button>',
        },
        VCard: { template: '<section><slot /></section>' },
        VCardActions: { template: '<div><slot /></div>' },
        VCheckbox: {
          emits: ['update:modelValue'],
          props: ['disabled', 'label', 'modelValue'],
          template: `
            <label>
              <input
                type="checkbox"
                data-test="column-checkbox"
                :aria-label="label"
                :checked="modelValue"
                :disabled="disabled"
                @change="$emit('update:modelValue', $event.target.checked)"
              />
              <span>{{ label }}</span>
            </label>
          `,
        },
        VChip: { template: '<span><slot /></span>' },
        VDivider: { template: '<hr />' },
        VList: { template: '<div><slot /></div>' },
        VListItem: { template: '<div><slot /></div>' },
        VMenu: {
          template: `
            <section>
              <slot name="activator" :props="{}" />
              <slot />
            </section>
          `,
        },
        VSpacer: { template: '<span />' },
        VTooltip: {
          props: ['text'],
          template:
            '<span><slot name="activator" :props="{}" /><span data-test="tooltip">{{ text }}</span></span>',
        },
      },
    },
  })
}

function headers(wrapper: ReturnType<typeof mountTable>) {
  return wrapper.get('[data-test="headers"]').text()
}

function columnCheckbox(
  wrapper: ReturnType<typeof mountTable>,
  label: string,
) {
  return wrapper.get(
    `[data-test="column-checkbox"][aria-label="${label}"]`,
  ) as DOMWrapper<HTMLInputElement>
}

async function setColumnChecked(
  wrapper: ReturnType<typeof mountTable>,
  label: string,
  checked: boolean,
) {
  const checkbox = columnCheckbox(wrapper, label)
  checkbox.element.checked = checked
  await checkbox.trigger('change')
}

function resetButton(wrapper: ReturnType<typeof mountTable>) {
  const button = wrapper
    .findAll('button')
    .find((candidate) => candidate.text() === 'Reset')

  if (!button) {
    throw new Error('Reset button not found')
  }

  return button
}

describe('PromptHistoryTable', () => {
  beforeEach(() => {
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: localStorageMock,
    })
    Object.defineProperty(window, 'localStorage', {
      configurable: true,
      value: localStorageMock,
    })
    localStorage.clear()
  })

  it('renders long multiline prompts as compact previews', () => {
    const wrapper = mountTable({ items: [longMultilinePrompt] })
    const preview = wrapper.get('[data-test="prompt-preview"]').text()

    expect(preview).toContain('First line second segment')
    expect(preview).not.toContain('\n')
    expect(preview.length).toBeLessThanOrEqual(223)
    expect(preview.endsWith('...')).toBe(true)
  })

  it('uses compact default columns for user prompt history', () => {
    const wrapper = mountTable({ items: [prompt] })

    expect(headers(wrapper)).toBe('Prompt|Provider|Model|Created|Actions')
    expect(headers(wrapper)).not.toContain('Input tokens')
    expect(headers(wrapper)).not.toContain('User')
  })

  it('uses compact default columns for admin prompt history', () => {
    const wrapper = mountTable({ items: [adminPrompt], showUser: true })

    expect(headers(wrapper)).toBe('Prompt|User|Provider|Model|Created|Actions')
    expect(headers(wrapper)).not.toContain('Client IP')
    expect(headers(wrapper)).not.toContain('Input tokens')
  })

  it('shows and hides optional columns', async () => {
    const wrapper = mountTable({ items: [prompt] })

    await setColumnChecked(wrapper, 'Input tokens', true)

    expect(headers(wrapper)).toContain('Input tokens')
    expect(wrapper.text()).toContain('10')

    await setColumnChecked(wrapper, 'Input tokens', false)

    expect(headers(wrapper)).not.toContain('Input tokens')
  })

  it('keeps required columns locked on', () => {
    const wrapper = mountTable({ items: [prompt] })

    expect(columnCheckbox(wrapper, 'Prompt').attributes('disabled')).toBe('')
    expect(columnCheckbox(wrapper, 'Created').attributes('disabled')).toBe('')
    expect(columnCheckbox(wrapper, 'Actions').attributes('disabled')).toBe('')
  })

  it('resets visible columns to the scope defaults', async () => {
    const wrapper = mountTable({ items: [prompt] })

    await setColumnChecked(wrapper, 'Total tokens', true)
    expect(headers(wrapper)).toContain('Total tokens')

    await resetButton(wrapper).trigger('click')

    expect(headers(wrapper)).toBe('Prompt|Provider|Model|Created|Actions')
  })

  it('persists user and admin column preferences separately', async () => {
    const userKey = 'promptgate.promptHistory.columns.test'
    const adminKey = 'promptgate.adminPromptHistory.columns.test'
    const userWrapper = mountTable({
      columnPreferencesKey: userKey,
      items: [prompt],
    })

    await setColumnChecked(userWrapper, 'Input tokens', true)
    userWrapper.unmount()

    const persistedUserWrapper = mountTable({
      columnPreferencesKey: userKey,
      items: [prompt],
    })
    const adminWrapper = mountTable({
      columnPreferencesKey: adminKey,
      items: [adminPrompt],
      showUser: true,
    })

    expect(headers(persistedUserWrapper)).toContain('Input tokens')
    expect(headers(adminWrapper)).toBe(
      'Prompt|User|Provider|Model|Created|Actions',
    )
  })

  it('resets sorting when the currently sorted column is hidden', async () => {
    const columnPreferencesKey = 'prompt-history-table-sort-columns'
    localStorage.setItem(
      columnPreferencesKey,
      JSON.stringify([
        'prompt',
        'provider',
        'model',
        'inputTokens',
        'createdAt',
        'actions',
      ]),
    )
    const wrapper = mountTable({
      columnPreferencesKey,
      items: [prompt],
      sortBy: 'inputTokens',
    })

    expect(headers(wrapper)).toContain('Input tokens')

    await setColumnChecked(wrapper, 'Input tokens', false)

    expect(wrapper.emitted('update:sort')).toEqual([['createdAt', 'desc']])
  })

  it('renders a user request graph action', async () => {
    const wrapper = mountTable({ items: [prompt] })

    expect(headers(wrapper)).toContain('Actions')
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

    expect(headers(wrapper)).not.toContain('Client IP')

    await wrapper.get('[aria-label="View request graph"]').trigger('click')

    expect(wrapper.get('[data-test="dialog-actor"]').text()).toBe('ada')
  })
})
