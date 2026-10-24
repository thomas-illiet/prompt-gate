import type {
  ModelLabProtocol,
  ModelLabStreamUpdate,
} from '~/types/model-lab'

export interface SSEFrame {
  data: string
  event: string
}

export interface SSEBufferResult {
  frames: SSEFrame[]
  remainder: string
}

export class ModelLabStreamError extends Error {
  constructor(
    message: string,
    readonly code = 'model_lab_stream_error',
    readonly status = 0,
  ) {
    super(message)
    this.name = 'ModelLabStreamError'
  }
}

const MODEL_LAB_MIN_STREAM_BYTES = 1 << 20
const MODEL_LAB_DEFAULT_STREAM_BYTES = 2 << 20
const MODEL_LAB_HARD_MAX_STREAM_BYTES = 16 << 20
const MODEL_LAB_BYTES_PER_OUTPUT_TOKEN = 512

export function modelLabStreamByteLimit(maxOutputTokens: number) {
  if (!Number.isFinite(maxOutputTokens) || maxOutputTokens <= 0) {
    return MODEL_LAB_DEFAULT_STREAM_BYTES
  }
  return Math.min(
    MODEL_LAB_HARD_MAX_STREAM_BYTES,
    Math.max(
      MODEL_LAB_MIN_STREAM_BYTES,
      Math.trunc(maxOutputTokens) * MODEL_LAB_BYTES_PER_OUTPUT_TOKEN,
    ),
  )
}

// consumeSSEBuffer parses complete SSE frames and retains a fragmented tail.
export function consumeSSEBuffer(buffer: string): SSEBufferResult {
  const normalized = buffer.replaceAll('\r\n', '\n').replaceAll('\r', '\n')
  const blocks = normalized.split('\n\n')
  const remainder = blocks.pop() ?? ''
  const frames: SSEFrame[] = []

  for (const block of blocks) {
    let event = 'message'
    const data: string[] = []

    for (const line of block.split('\n')) {
      if (line.startsWith('event:')) {
        event = line.slice(6).trim()
      } else if (line.startsWith('data:')) {
        data.push(line.slice(5).trimStart())
      }
    }

    if (data.length > 0) {
      frames.push({ data: data.join('\n'), event })
    }
  }

  return { frames, remainder }
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null
    ? (value as Record<string, unknown>)
    : null
}

function finiteNumber(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function parseOpenAIEvent(payload: Record<string, unknown>): ModelLabStreamUpdate {
  if (asRecord(payload.error)) {
    throw new ModelLabStreamError(
      'The provider stopped the stream.',
      'provider_stream_error',
    )
  }
  const choices = Array.isArray(payload.choices) ? payload.choices : []
  const firstChoice = asRecord(choices[0])
  const delta = asRecord(firstChoice?.delta)
  const content = typeof delta?.content === 'string' ? delta.content : ''
  const usage = asRecord(payload.usage)

  return {
    output: content,
    usage: {
      inputTokens: finiteNumber(usage?.prompt_tokens),
      outputTokens: finiteNumber(usage?.completion_tokens),
    },
  }
}

function parseAnthropicEvent(
  frame: SSEFrame,
  payload: Record<string, unknown>,
): ModelLabStreamUpdate {
  const eventType =
    typeof payload.type === 'string' ? payload.type : frame.event
  const delta = asRecord(payload.delta)
  const message = asRecord(payload.message)
  const messageUsage = asRecord(message?.usage)
  const usage = asRecord(payload.usage)

  if (eventType === 'error') {
    throw new ModelLabStreamError(
      'The provider stopped the stream.',
      'provider_stream_error',
    )
  }

  return {
    output:
      eventType === 'content_block_delta' && typeof delta?.text === 'string'
        ? delta.text
        : '',
    usage: {
      inputTokens:
        finiteNumber(messageUsage?.input_tokens) ??
        finiteNumber(usage?.input_tokens),
      outputTokens:
        finiteNumber(usage?.output_tokens) ??
        finiteNumber(messageUsage?.output_tokens),
    },
  }
}

// parseModelLabSSEFrame extracts text and usage without retaining the raw event.
export function parseModelLabSSEFrame(
  protocol: ModelLabProtocol,
  frame: SSEFrame,
): ModelLabStreamUpdate | null {
  if (frame.data === '[DONE]') return null

  let parsed: unknown
  try {
    parsed = JSON.parse(frame.data)
  } catch {
    throw new ModelLabStreamError(
      'The provider returned a malformed stream.',
      'invalid_provider_stream',
    )
  }

  const payload = asRecord(parsed)
  if (!payload) return null

  return protocol === 'anthropic-messages'
    ? parseAnthropicEvent(frame, payload)
    : parseOpenAIEvent(payload)
}

function isTerminalModelLabSSEFrame(
  protocol: ModelLabProtocol,
  frame: SSEFrame,
) {
  if (protocol === 'openai-chat') return frame.data === '[DONE]'
  if (frame.event === 'message_stop') return true

  try {
    const payload = asRecord(JSON.parse(frame.data))
    return payload?.type === 'message_stop'
  } catch {
    // parseModelLabSSEFrame reports malformed JSON before this helper runs.
    return false
  }
}

function streamErrorMessage(status: number) {
  if (status === 401) return 'This run ticket expired or was already used.'
  if (status === 403) return 'Policy or group access denied this model.'
  if (status === 429) return 'Your quota or parallel run limit was reached.'
  if (status === 502 || status === 503) {
    return 'The model provider is temporarily unavailable.'
  }
  return 'The model request could not be completed.'
}

export interface ReadModelLabStreamOptions {
  body: unknown
  endpoint: string
  onUpdate: (update: ModelLabStreamUpdate) => void
  protocol: ModelLabProtocol
  signal: AbortSignal
  token: string
  maxStreamBytes?: number
}

// readModelLabStream performs a no-store bearer request and incrementally reads SSE.
export async function readModelLabStream(options: ReadModelLabStreamOptions) {
  const response = await fetch(options.endpoint, {
    body: JSON.stringify(options.body),
    cache: 'no-store',
    credentials: 'omit',
    headers: {
      Accept: 'text/event-stream',
      Authorization: `Bearer ${options.token}`,
      'Content-Type': 'application/json',
    },
    method: 'POST',
    redirect: 'error',
    referrerPolicy: 'no-referrer',
    signal: options.signal,
  })

  if (!response.ok) {
    throw new ModelLabStreamError(
      streamErrorMessage(response.status),
      'model_lab_request_failed',
      response.status,
    )
  }
  if (!response.body) {
    throw new ModelLabStreamError(
      'The provider returned an empty stream.',
      'empty_provider_stream',
      response.status,
    )
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let completed = false
  let receivedTerminalFrame = false
  let receivedBytes = 0
  const maxStreamBytes = Math.min(
    MODEL_LAB_HARD_MAX_STREAM_BYTES,
    Math.max(1, options.maxStreamBytes ?? MODEL_LAB_DEFAULT_STREAM_BYTES),
  )

  try {
    while (true) {
      const { done, value } = await reader.read()
      receivedBytes += value?.byteLength ?? 0
      if (receivedBytes > maxStreamBytes) {
        throw new ModelLabStreamError(
          'The provider stream exceeded the safe display limit.',
          'provider_stream_too_large',
        )
      }
      buffer += decoder.decode(value, { stream: !done })
      if (done && buffer.trim()) buffer += '\n\n'

      const consumed = consumeSSEBuffer(buffer)
      buffer = consumed.remainder

      for (const frame of consumed.frames) {
        const update = parseModelLabSSEFrame(options.protocol, frame)
        if (update) options.onUpdate(update)
        if (isTerminalModelLabSSEFrame(options.protocol, frame)) {
          receivedTerminalFrame = true
        }
      }

      if (done) break
    }
    if (!receivedTerminalFrame) {
      throw new ModelLabStreamError(
        'The provider ended the stream before completion.',
        'incomplete_provider_stream',
      )
    }
    completed = true
  } finally {
    if (!completed) {
      try {
        await reader.cancel()
      } catch {
        // Preserve the original stream/parser failure.
      }
    }
    reader.releaseLock()
  }
}
