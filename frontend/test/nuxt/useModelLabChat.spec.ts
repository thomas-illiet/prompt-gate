import { effectScope, nextTick, shallowRef } from 'vue'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useModelLabChat } from '../../app/composables/useModelLabChat'
import type {
  ModelLabTarget,
  ModelLabTicketRequest,
  ModelLabTicketResponse,
} from '../../app/types/model-lab'

const { apiFetch, useApiFetchMock } = vi.hoisted(() => {
  const apiFetch = vi.fn()
  return { apiFetch, useApiFetchMock: vi.fn(() => apiFetch) }
})

mockNuxtImport('useApiFetch', () => useApiFetchMock)

const targets: ModelLabTarget[] = ['alpha', 'beta', 'gamma'].map((model) => ({
  model,
  providerDisplayName: 'OpenAI Test',
  providerName: `openai-${model}`,
  providerType: 'openai',
  protocol: 'openai-chat',
}))

function ticketResponse(
  request: ModelLabTicketRequest,
): ModelLabTicketResponse {
  return {
    comparisonId: 'comparison-id',
    expiresAt: '2026-10-24T10:01:00Z',
    tickets: request.targets.map((target, index) => ({
      endpoint: `https://proxy.example/${target.providerName}/v1/chat/completions`,
      method: 'POST',
      model: target.model,
      protocol: 'openai-chat',
      providerName: target.providerName,
      token: `pgml_${index}`,
    })),
  }
}

function successfulStream(text: string) {
  const encoder = new TextEncoder()
  return new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(
          encoder.encode(
            `data: ${JSON.stringify({ choices: [{ delta: { content: text } }] })}\n\n`,
          ),
        )
        controller.enqueue(encoder.encode('data: [DONE]\n\n'))
        controller.close()
      },
    }),
    { headers: { 'Content-Type': 'text/event-stream' }, status: 200 },
  )
}

describe('useModelLabChat', () => {
  beforeEach(() => {
    apiFetch.mockReset()
    apiFetch.mockImplementation(
      (_path: string, options: { body: ModelLabTicketRequest }) =>
        Promise.resolve(ticketResponse(options.body)),
    )
    useApiFetchMock.mockClear()
  })

  afterEach(() => vi.unstubAllGlobals())

  it('keeps an untouched composer neutral and exposes the byte-limit input cap', () => {
    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!

    expect(chat.draftError.value).toBe('')
    expect(chat.maxPromptLength.value).toBe(32_768)
    scope.stop()
  })

  it('sends completed primary turns as multi-turn context', async () => {
    let responseNumber = 0
    const fetchMock = vi.fn(
      (_input: RequestInfo | URL, _init?: RequestInit) => {
        responseNumber += 1
        return Promise.resolve(successfulStream(`answer-${responseNumber}`))
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!
    expect(chat.primaryTarget.value?.model).toBe('alpha')

    chat.setDraft('first question')
    await chat.send()
    expect(chat.turns.value[0]?.comparisons).toEqual([])
    expect(apiFetch).toHaveBeenCalledTimes(1)
    expect(
      (apiFetch.mock.calls[0]?.[1]?.body as ModelLabTicketRequest).targets,
    ).toEqual([{ model: 'alpha', providerName: 'openai-alpha' }])

    chat.setDraft('follow up')
    await chat.send()

    const secondBody = JSON.parse(
      String(fetchMock.mock.calls[1]?.[1]?.body),
    ) as { messages: unknown[] }
    expect(secondBody.messages).toEqual([
      { content: 'first question', role: 'user' },
      { content: 'answer-1', role: 'assistant' },
      { content: 'follow up', role: 'user' },
    ])
    expect(chat.turns.value).toHaveLength(2)
    expect(chat.turns.value[1]?.context).toEqual(secondBody.messages)
    scope.stop()
  })

  it('runs a selected second model in the same batch and keeps it out of later context', async () => {
    let responseNumber = 0
    const fetchMock = vi.fn(
      (_input: RequestInfo | URL, _init?: RequestInit) => {
        responseNumber += 1
        return Promise.resolve(successfulStream(`answer-${responseNumber}`))
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!
    chat.setSelectedTargets([targets[0]!, targets[1]!])
    chat.setDraft('compare me')
    await chat.send()
    const firstTurn = chat.turns.value[0]!
    expect(firstTurn.comparisons).toHaveLength(1)
    expect(apiFetch).toHaveBeenCalledTimes(1)
    expect(
      (apiFetch.mock.calls[0]?.[1]?.body as ModelLabTicketRequest).targets,
    ).toEqual([
      { model: 'alpha', providerName: 'openai-alpha' },
      { model: 'beta', providerName: 'openai-beta' },
    ])
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(firstTurn.primary.output).toBe('answer-1')
    expect(firstTurn.comparisons[0]?.output).toBe('answer-2')

    chat.setDraft('continue')
    await chat.send()
    expect(fetchMock).toHaveBeenCalledTimes(4)
    for (const call of fetchMock.mock.calls.slice(2)) {
      const continued = JSON.parse(String(call[1]?.body)) as {
        messages: Array<{ content: string; role: string }>
      }
      expect(continued.messages).toEqual([
        { content: 'compare me', role: 'user' },
        { content: 'answer-1', role: 'assistant' },
        { content: 'continue', role: 'user' },
      ])
      expect(
        continued.messages.map((message) => message.content),
      ).not.toContain('answer-2')
    }
    scope.stop()
  })

  it('honors a server limit of one selected model', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(successfulStream('answer')),
    )
    const configuredLimits = shallowRef({
      maxOutputTokens: 4096,
      maxPromptBytes: 32_768,
      maxTargets: 1,
    })

    const scope = effectScope()
    const chat = scope.run(() =>
      useModelLabChat(shallowRef(targets), configuredLimits),
    )!
    chat.setSelectedTargets(targets)
    chat.setDraft('one model only')
    await chat.send()

    expect(chat.maxSelectedTargets.value).toBe(1)
    expect(chat.selectedTargets.value).toEqual([targets[0]])
    expect(chat.turns.value[0]?.comparisons).toEqual([])
    expect(
      (apiFetch.mock.calls[0]?.[1]?.body as ModelLabTicketRequest).targets,
    ).toEqual([{ model: 'alpha', providerName: 'openai-alpha' }])
    scope.stop()
  })

  it('keeps at most two unique accessible models in selection order', () => {
    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!
    const unavailable = { ...targets[0]!, model: 'unavailable' }

    chat.setSelectedTargets([
      targets[2]!,
      targets[2]!,
      unavailable,
      targets[1]!,
      targets[0]!,
    ])

    expect(chat.selectedTargets.value).toEqual([targets[2], targets[1]])
    expect(chat.primaryTarget.value).toEqual(targets[2])
    scope.stop()
  })

  it('preserves valid selections when the available model list refreshes', async () => {
    const availableTargets = shallowRef(targets)
    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(availableTargets))!
    chat.setSelectedTargets([targets[1]!, targets[2]!])

    availableTargets.value = [targets[0]!, { ...targets[2]! }]
    await nextTick()
    expect(chat.selectedTargets.value.map((target) => target.model)).toEqual([
      'gamma',
    ])

    availableTargets.value = [targets[0]!]
    await nextTick()
    expect(chat.selectedTargets.value).toEqual([targets[0]])
    scope.stop()
  })

  it('retries with the stored context and settings', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response('{}', { status: 503 }))
      .mockResolvedValueOnce(successfulStream('recovered'))
    vi.stubGlobal('fetch', fetchMock)

    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!
    chat.setDraft('original prompt')
    await chat.send()
    const failed = chat.turns.value[0]!
    expect(failed.primary.status).toBe('error')

    chat.setDraft('unrelated draft')
    chat.setTemperature(1)
    chat.setMaxOutputTokens(2048)
    await chat.retryLane(failed.id, failed.primary.id)

    const retryBody = JSON.parse(
      String(fetchMock.mock.calls[1]?.[1]?.body),
    ) as Record<string, unknown>
    expect(retryBody).toMatchObject({
      max_tokens: 1024,
      messages: [{ content: 'original prompt', role: 'user' }],
      temperature: 0.7,
    })
    expect(chat.turns.value[0]?.primary.output).toBe('recovered')
    scope.stop()
  })

  it('marks a premature provider EOF as failed and excludes it from later context', async () => {
    const encoder = new TextEncoder()
    const incompleteStream = new Response(
      new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(
            encoder.encode(
              'data: {"choices":[{"delta":{"content":"partial answer"}}]}\n\n',
            ),
          )
          controller.close()
        },
      }),
      { headers: { 'Content-Type': 'text/event-stream' }, status: 200 },
    )
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(incompleteStream)
      .mockResolvedValueOnce(successfulStream('clean answer'))
    vi.stubGlobal('fetch', fetchMock)

    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!
    chat.setDraft('first question')
    await chat.send()

    expect(chat.turns.value[0]?.primary).toMatchObject({
      error: 'The provider ended the stream before completion.',
      output: 'partial answer',
      status: 'error',
    })

    chat.setDraft('fresh question')
    await chat.send()
    const nextBody = JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body)) as {
      messages: unknown[]
    }
    expect(nextBody.messages).toEqual([
      { content: 'fresh question', role: 'user' },
    ])
    scope.stop()
  })

  it('stops before provider transmission and resets only ephemeral conversation state', async () => {
    let resolveTickets!: (value: ModelLabTicketResponse) => void
    apiFetch.mockImplementationOnce(
      (_path: string, options: { body: ModelLabTicketRequest }) =>
        new Promise<ModelLabTicketResponse>((resolve) => {
          resolveTickets = (value) => resolve(value)
          void options
        }),
    )
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!
    chat.setSelectedTargets([targets[1]!])
    chat.setTemperature(0.3)
    chat.setDraft('do not transmit')
    const sendPromise = chat.send()
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledOnce())

    chat.stopAll()
    const request = apiFetch.mock.calls[0]?.[1]?.body as ModelLabTicketRequest
    resolveTickets(ticketResponse(request))
    await sendPromise
    await nextTick()

    expect(fetchMock).not.toHaveBeenCalled()
    expect(chat.turns.value[0]?.primary.status).toBe('stopped')
    chat.reset()
    expect(chat.turns.value).toEqual([])
    expect(chat.draft.value).toBe('')
    expect(chat.primaryTarget.value?.model).toBe('beta')
    expect(chat.selectedTargets.value.map((target) => target.model)).toEqual([
      'beta',
    ])
    expect(chat.temperature.value).toBe(0.3)
    scope.stop()
  })

  it('keeps a cancelled lane stopped when shared ticket issuance fails', async () => {
    let rejectTickets!: (reason: Error) => void
    apiFetch.mockImplementationOnce(
      () =>
        new Promise<ModelLabTicketResponse>((_resolve, reject) => {
          rejectTickets = reject
        }),
    )
    vi.stubGlobal('fetch', vi.fn())

    const scope = effectScope()
    const chat = scope.run(() => useModelLabChat(shallowRef(targets)))!
    chat.setDraft('cancel while authorizing')
    const sendPromise = chat.send()
    await vi.waitFor(() => expect(apiFetch).toHaveBeenCalledOnce())

    const laneID = chat.turns.value[0]!.primary.id
    chat.cancelLane(laneID)
    rejectTickets(new Error('ticket backend unavailable'))
    await sendPromise

    expect(chat.turns.value[0]?.primary.status).toBe('stopped')
    expect(chat.turns.value[0]?.primary.error).toBeNull()
    scope.stop()
  })
})
