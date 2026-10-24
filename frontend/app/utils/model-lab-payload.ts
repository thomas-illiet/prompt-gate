import type {
  ModelLabConversationMessage,
  ModelLabRunSettings,
  ModelLabTarget,
} from '~/types/model-lab'

// buildModelLabPayload emits only the text conversation fields accepted by the
// proxy validator. It deliberately excludes tools and provider-specific extras.
export function buildModelLabPayload(
  target: ModelLabTarget,
  conversation: ModelLabConversationMessage[] | string,
  settings: ModelLabRunSettings,
) {
  const messages =
    typeof conversation === 'string'
      ? [{ content: conversation, role: 'user' as const }]
      : conversation.map((message) => ({ ...message }))

  return {
    max_tokens: settings.maxOutputTokens,
    messages,
    model: target.model,
    stream: true,
    temperature: settings.temperature,
  }
}

export function modelLabPayloadByteLength(
  target: ModelLabTarget,
  conversation: ModelLabConversationMessage[],
  settings: ModelLabRunSettings,
) {
  return new TextEncoder().encode(
    JSON.stringify(buildModelLabPayload(target, conversation, settings)),
  ).byteLength
}
