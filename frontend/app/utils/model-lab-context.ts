import type {
  ModelLabContextWindow,
  ModelLabConversationMessage,
  ModelLabRunSettings,
  ModelLabTarget,
  ModelLabTurn,
} from '~/types/model-lab'
import { modelLabPayloadByteLength } from './model-lab-payload'

interface BuildModelLabContextWindowOptions {
  maxBytes: number
  prompt: string
  settings: ModelLabRunSettings
  targets: ModelLabTarget[]
  turns: ModelLabTurn[]
}

function cloneMessages(messages: ModelLabConversationMessage[]) {
  return messages.map((message) => ({ ...message }))
}

function completedConversationPairs(turns: ModelLabTurn[]) {
  return turns
    .filter(
      (turn) =>
        turn.primary.status === 'completed' &&
        turn.primary.output.trim().length > 0,
    )
    .map<ModelLabConversationMessage[]>((turn) => [
      { content: turn.prompt, role: 'user' },
      { content: turn.primary.output, role: 'assistant' },
    ])
}

export function modelLabConversationByteLength(
  targets: ModelLabTarget[],
  messages: ModelLabConversationMessage[],
  settings: ModelLabRunSettings,
) {
  if (targets.length === 0) return Number.POSITIVE_INFINITY

  return Math.max(
    ...targets.map((target) =>
      modelLabPayloadByteLength(target, messages, settings),
    ),
  )
}

export function modelLabContextFitsTargets(
  targets: ModelLabTarget[],
  messages: ModelLabConversationMessage[],
  settings: ModelLabRunSettings,
  maxBytes: number,
) {
  return (
    modelLabConversationByteLength(targets, messages, settings) <= maxBytes
  )
}

// buildModelLabContextWindow drops only complete, oldest user/assistant pairs.
// The current user message is never truncated or silently omitted.
export function buildModelLabContextWindow(
  options: BuildModelLabContextWindowOptions,
): ModelLabContextWindow {
  const pairs = completedConversationPairs(options.turns)
  const current: ModelLabConversationMessage = {
    content: options.prompt.trim(),
    role: 'user',
  }
  let omittedTurns = 0

  while (omittedTurns <= pairs.length) {
    const messages = [
      ...pairs.slice(omittedTurns).flatMap(cloneMessages),
      current,
    ]
    const byteLength = modelLabConversationByteLength(
      options.targets,
      messages,
      options.settings,
    )

    if (byteLength <= options.maxBytes) {
      return {
        byteLength,
        fits: true,
        messages,
        omittedTurns,
      }
    }

    if (omittedTurns === pairs.length) {
      return {
        byteLength,
        fits: false,
        messages,
        omittedTurns,
      }
    }
    omittedTurns += 1
  }

  // The loop always returns; keep a closed fallback for exhaustive callers.
  return {
    byteLength: Number.POSITIVE_INFINITY,
    fits: false,
    messages: [current],
    omittedTurns: pairs.length,
  }
}
