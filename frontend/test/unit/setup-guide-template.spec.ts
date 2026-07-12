import { describe, expect, it } from 'vitest'
import {
  renderSetupGuideTemplate,
  validateSetupGuideTemplate,
} from '../../app/utils/setup-guide-template'

const context = {
  token: 'token',
  baseUrl: 'base',
  openaiBaseUrl: 'openai',
  anthropicBaseUrl: 'anthropic',
  model: 'one',
  models: ['one', 'two'],
  providerName: 'provider',
  providerDisplayName: 'Provider',
}
describe('setup guide templates', () => {
  it('renders variables and model sections', () =>
    expect(
      renderSetupGuideTemplate(
        '{{providerDisplayName}}\n{{#models}}{{model}}\n{{/models}}',
        context,
      ),
    ).toBe('Provider\none\ntwo\n'))
  it('rejects unknown and malformed variables', () => {
    expect(validateSetupGuideTemplate('{{unknown}}')).toContain('Unknown')
    expect(validateSetupGuideTemplate('{{#models}}{{model}}')).toContain(
      'Unclosed',
    )
  })
})
