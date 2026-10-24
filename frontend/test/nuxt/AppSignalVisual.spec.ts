import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppSignalVisual from '../../app/components/App/AppSignalVisual.vue'
import type { AppSignalVisualName } from '../../app/types/theme'

const signalNames: AppSignalVisualName[] = [
  'book',
  'cables',
  'gate',
  'heartbeat',
  'identity',
  'key',
  'ledger',
  'network',
  'people',
  'radar',
  'robot',
]

describe('AppSignalVisual', () => {
  it.each(signalNames)('renders the %s signal as decorative SVG', (name) => {
    const wrapper = mount(AppSignalVisual, { props: { name } })
    const svg = wrapper.get('svg')

    expect(svg.attributes('data-signal')).toBe(name)
    expect(svg.attributes('aria-hidden')).toBe('true')
  })

  it('becomes a labelled image when a label is provided', () => {
    const wrapper = mount(AppSignalVisual, {
      props: { label: 'Identity signal', name: 'identity' },
    })
    const svg = wrapper.get('svg')

    expect(svg.attributes('aria-label')).toBe('Identity signal')
    expect(svg.attributes('aria-hidden')).toBeUndefined()
    expect(svg.attributes('role')).toBe('img')
  })
})
