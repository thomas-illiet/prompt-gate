import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  ModelLabStreamError,
  consumeSSEBuffer,
  parseModelLabSSEFrame,
  readModelLabStream,
} from '../../app/utils/model-lab-stream'

describe('Model Lab stream utilities', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('retains a fragmented SSE frame and parses it after completion', () => {
    const first = consumeSSEBuffer(
      'event: content_block_delta\ndata: {"type":"content_block_',
    )

    expect(first.frames).toEqual([])
    expect(first.remainder).toContain('content_block_')

    const second = consumeSSEBuffer(
      `${first.remainder}delta","delta":{"text":"hello"}}\n\n`,
    )

    expect(second.frames).toEqual([
      {
        event: 'content_block_delta',
        data: '{"type":"content_block_delta","delta":{"text":"hello"}}',
      },
    ])
    expect(second.remainder).toBe('')
  })

  it('extracts OpenAI text and usage without retaining the raw payload', () => {
    expect(
      parseModelLabSSEFrame('openai-chat', {
        event: 'message',
        data: JSON.stringify({
          choices: [{ delta: { content: '<script>alert(1)</script>' } }],
          usage: { prompt_tokens: 12, completion_tokens: 7 },
        }),
      }),
    ).toEqual({
      output: '<script>alert(1)</script>',
      usage: { inputTokens: 12, outputTokens: 7 },
    })
  })

  it('extracts Anthropic text and final usage', () => {
    expect(
      parseModelLabSSEFrame('anthropic-messages', {
        event: 'content_block_delta',
        data: JSON.stringify({
          type: 'content_block_delta',
          delta: { type: 'text_delta', text: 'Bonjour' },
        }),
      }),
    ).toMatchObject({ output: 'Bonjour' })

    expect(
      parseModelLabSSEFrame('anthropic-messages', {
        event: 'message_delta',
        data: JSON.stringify({
          type: 'message_delta',
          usage: { output_tokens: 9 },
        }),
      }),
    ).toMatchObject({ usage: { outputTokens: 9 } })
  })

  it('treats malformed provider data as a bounded error', () => {
    expect(() =>
      parseModelLabSSEFrame('openai-chat', {
        event: 'message',
        data: '{not-json',
      }),
    ).toThrow(ModelLabStreamError)
    expect(
      parseModelLabSSEFrame('openai-chat', {
        event: 'message',
        data: '[DONE]',
      }),
    ).toBeNull()
  })

  it('does not expose provider error details', () => {
    for (const protocol of ['openai-chat', 'anthropic-messages'] as const) {
      expect(() =>
        parseModelLabSSEFrame(protocol, {
          event: 'error',
          data: JSON.stringify({
            type: 'error',
            error: { message: 'upstream-secret-detail' },
          }),
        }),
      ).toThrow('The provider stopped the stream.')
    }
  })

  it('cancels the response reader when parsing fails before the stream ends', async () => {
    let cancelled = false
    const encoder = new TextEncoder()
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoder.encode('data: {not-json\n\n'))
      },
      cancel() {
        cancelled = true
      },
    })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(stream, {
          headers: { 'Content-Type': 'text/event-stream' },
          status: 200,
        }),
      ),
    )

    await expect(
      readModelLabStream({
        body: {},
        endpoint: 'https://proxy.example/provider/v1/chat/completions',
        onUpdate: vi.fn(),
        protocol: 'openai-chat',
        signal: new AbortController().signal,
        token: 'pgml_test',
      }),
    ).rejects.toThrow('malformed stream')
    expect(cancelled).toBe(true)
  })

  it.each([
    {
      protocol: 'openai-chat' as const,
      stream:
        'data: {"choices":[{"delta":{"content":"partial"}}]}\n\n',
    },
    {
      protocol: 'anthropic-messages' as const,
      stream:
        'event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"partial"}}\n\n',
    },
  ])('rejects a premature $protocol EOF', async ({ protocol, stream }) => {
    const encoder = new TextEncoder()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          new ReadableStream<Uint8Array>({
            start(controller) {
              controller.enqueue(encoder.encode(stream))
              controller.close()
            },
          }),
          {
            headers: { 'Content-Type': 'text/event-stream' },
            status: 200,
          },
        ),
      ),
    )

    const onUpdate = vi.fn()
    await expect(
      readModelLabStream({
        body: {},
        endpoint: 'https://proxy.example/provider/v1/chat/completions',
        onUpdate,
        protocol,
        signal: new AbortController().signal,
        token: 'pgml_test',
      }),
    ).rejects.toMatchObject({ code: 'incomplete_provider_stream' })
    expect(onUpdate).toHaveBeenCalledWith(
      expect.objectContaining({ output: 'partial' }),
    )
  })

  it('accepts an Anthropic stream only after message_stop', async () => {
    const encoder = new TextEncoder()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          new ReadableStream<Uint8Array>({
            start(controller) {
              controller.enqueue(
                encoder.encode(
                  'event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"complete"}}\n\n',
                ),
              )
              controller.enqueue(
                encoder.encode(
                  'event: message_stop\ndata: {"type":"message_stop"}\n\n',
                ),
              )
              controller.close()
            },
          }),
          {
            headers: { 'Content-Type': 'text/event-stream' },
            status: 200,
          },
        ),
      ),
    )

    await expect(
      readModelLabStream({
        body: {},
        endpoint: 'https://proxy.example/provider/v1/messages',
        onUpdate: vi.fn(),
        protocol: 'anthropic-messages',
        signal: new AbortController().signal,
        token: 'pgml_test',
      }),
    ).resolves.toBeUndefined()
  })

  it('cancels a stream that exceeds the cumulative browser limit', async () => {
    let cancelled = false
    const encoder = new TextEncoder()
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(
          encoder.encode(
            'data: {"choices":[{"delta":{"content":"safe"}}]}\n\n',
          ),
        )
        controller.enqueue(encoder.encode('data: {"choices":[]}\n\n'))
      },
      cancel() {
        cancelled = true
      },
    })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(stream, {
          headers: { 'Content-Type': 'text/event-stream' },
          status: 200,
        }),
      ),
    )

    await expect(
      readModelLabStream({
        body: {},
        endpoint: 'https://proxy.example/provider/v1/chat/completions',
        maxStreamBytes: 64,
        onUpdate: vi.fn(),
        protocol: 'openai-chat',
        signal: new AbortController().signal,
        token: 'pgml_test',
      }),
    ).rejects.toThrow('safe display limit')
    expect(cancelled).toBe(true)
  })
})
