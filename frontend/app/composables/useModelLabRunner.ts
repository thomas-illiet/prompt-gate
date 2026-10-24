import type {
  ModelLabConversationMessage,
  ModelLabLane,
  ModelLabRunSettings,
  ModelLabStreamUpdate,
  ModelLabTarget,
  ModelLabTicket,
  ModelLabTicketRequest,
  ModelLabTicketResponse,
} from '~/types/model-lab'
import { toApiErrorMessage } from '~/utils/api-error'
import { modelLabTargetKey } from '~/utils/model-lab'
import { buildModelLabPayload } from '~/utils/model-lab-payload'
import {
  modelLabStreamByteLimit,
  readModelLabStream,
} from '~/utils/model-lab-stream'

const TICKET_ERRORS = {
  invalid_model_lab_targets: 'Choose one or two distinct models.',
  model_lab_catalog_unavailable:
    'The available models are temporarily unavailable.',
  model_lab_parallel_limit: 'Three Model Lab streams are already active.',
  model_lab_ticket_issue_limit:
    'Too many unused run tickets are still pending. Wait briefly and try again.',
  model_lab_target_not_allowed:
    'One of the selected models is no longer allowed.',
  model_lab_ticket_unavailable: 'Run authorization is temporarily unavailable.',
} as const

let laneSequence = 0

export function createModelLabLane(target: ModelLabTarget): ModelLabLane {
  laneSequence += 1
  return {
    clientDurationMs: null,
    clientTtftMs: null,
    error: null,
    id: `model-lab-lane-${laneSequence}`,
    output: '',
    status: 'authorizing',
    target,
    usage: { inputTokens: null, outputTokens: null },
  }
}

function isAbortError(error: unknown) {
  return (
    (typeof DOMException !== 'undefined' &&
      error instanceof DOMException &&
      error.name === 'AbortError') ||
    (error instanceof Error && error.name === 'AbortError')
  )
}

export interface ModelLabRunnerRequest {
  lanes: ModelLabLane[]
  messages: ModelLabConversationMessage[]
  onLaneUpdate?: (lane: ModelLabLane) => void
  settings: ModelLabRunSettings
}

// useModelLabRunner owns only ticket issuance, stream transport and cancellation.
// Conversation state and snapshots remain the responsibility of useModelLabChat.
export function useModelLabRunner() {
  const apiFetch = useApiFetch()
  const lanes = shallowRef<ModelLabLane[]>([])
  const error = shallowRef<string | null>(null)
  const controllers = new Map<string, AbortController>()
  const cancelledBeforeStart = new Set<string>()
  let authorizationController: AbortController | null = null
  let onLaneUpdate: ((lane: ModelLabLane) => void) | null = null
  let runVersion = 0

  const running = computed(() =>
    lanes.value.some(
      (lane) => lane.status === 'authorizing' || lane.status === 'streaming',
    ),
  )

  function replaceLane(
    version: number,
    id: string,
    update: (lane: ModelLabLane) => ModelLabLane,
  ) {
    if (version !== runVersion) return
    let changed: ModelLabLane | null = null
    lanes.value = lanes.value.map((lane) => {
      if (lane.id !== id) return lane
      changed = update(lane)
      return changed
    })
    if (changed) onLaneUpdate?.(changed)
  }

  function emitLanes() {
    for (const lane of lanes.value) onLaneUpdate?.(lane)
  }

  function cancelLane(id: string) {
    const controller = controllers.get(id)
    if (controller) controller.abort()
    else cancelledBeforeStart.add(id)

    replaceLane(runVersion, id, (lane) =>
      lane.status === 'authorizing' || lane.status === 'streaming'
        ? { ...lane, status: 'stopped' }
        : lane,
    )
  }

  function stopAll() {
    runVersion += 1
    authorizationController?.abort()
    authorizationController = null
    for (const controller of controllers.values()) controller.abort()
    controllers.clear()
    lanes.value = lanes.value.map((lane) =>
      lane.status === 'authorizing' || lane.status === 'streaming'
        ? { ...lane, status: 'stopped' }
        : lane,
    )
    emitLanes()
    onLaneUpdate = null
  }

  async function issueTickets(targets: ModelLabTarget[], signal: AbortSignal) {
    const request: ModelLabTicketRequest = {
      targets: targets.map(({ model, providerName }) => ({
        model,
        providerName,
      })),
    }
    return apiFetch<ModelLabTicketResponse>('/api/v1/me/model-lab/tickets', {
      body: request,
      method: 'POST',
      redirectOnAccessError: false,
      signal,
    })
  }

  async function runLane(
    version: number,
    lane: ModelLabLane,
    ticket: ModelLabTicket,
    messages: ModelLabConversationMessage[],
    settings: ModelLabRunSettings,
  ) {
    const controller = new AbortController()
    controllers.set(lane.id, controller)
    const startedAt = performance.now()
    let firstChunkAt: number | null = null
    let pendingOutput = ''
    let flushTimer: ReturnType<typeof setTimeout> | null = null

    const flush = () => {
      if (!pendingOutput) return
      const text = pendingOutput
      pendingOutput = ''
      replaceLane(version, lane.id, (current) => ({
        ...current,
        output: current.output + text,
      }))
    }
    const onUpdate = (update: ModelLabStreamUpdate) => {
      if (version !== runVersion) return
      if (update.output) {
        if (firstChunkAt === null) {
          firstChunkAt = performance.now()
          replaceLane(version, lane.id, (current) => ({
            ...current,
            clientTtftMs: Math.round(firstChunkAt! - startedAt),
          }))
        }
        pendingOutput += update.output
        if (flushTimer === null) {
          flushTimer = setTimeout(() => {
            flushTimer = null
            flush()
          }, 40)
        }
      }
      if (update.usage) {
        replaceLane(version, lane.id, (current) => ({
          ...current,
          usage: {
            inputTokens: update.usage?.inputTokens ?? current.usage.inputTokens,
            outputTokens:
              update.usage?.outputTokens ?? current.usage.outputTokens,
          },
        }))
      }
    }

    replaceLane(version, lane.id, (current) => ({
      ...current,
      status: 'streaming',
    }))

    try {
      await readModelLabStream({
        body: buildModelLabPayload(lane.target, messages, settings),
        endpoint: ticket.endpoint,
        maxStreamBytes: modelLabStreamByteLimit(settings.maxOutputTokens),
        onUpdate,
        protocol: ticket.protocol,
        signal: controller.signal,
        token: ticket.token,
      })
      flush()
      replaceLane(version, lane.id, (current) => ({
        ...current,
        clientDurationMs: Math.round(performance.now() - startedAt),
        status: 'completed',
      }))
    } catch (streamError) {
      flush()
      replaceLane(version, lane.id, (current) => ({
        ...current,
        clientDurationMs: Math.round(performance.now() - startedAt),
        error: isAbortError(streamError)
          ? null
          : streamError instanceof Error
            ? streamError.message
            : 'The model request failed.',
        status: isAbortError(streamError) ? 'stopped' : 'error',
      }))
    } finally {
      if (flushTimer !== null) clearTimeout(flushTimer)
      if (controllers.get(lane.id) === controller) controllers.delete(lane.id)
    }
  }

  async function run(request: ModelLabRunnerRequest) {
    stopAll()
    const version = runVersion
    cancelledBeforeStart.clear()
    error.value = null
    onLaneUpdate = request.onLaneUpdate ?? null
    lanes.value = request.lanes.map((lane) => ({
      ...lane,
      clientDurationMs: null,
      clientTtftMs: null,
      error: null,
      output: '',
      status: 'authorizing',
      usage: { inputTokens: null, outputTokens: null },
    }))
    emitLanes()

    try {
      const controller = new AbortController()
      authorizationController = controller
      const response = await issueTickets(
        lanes.value.map((lane) => lane.target),
        controller.signal,
      )
      if (authorizationController === controller) authorizationController = null
      if (version !== runVersion) return lanes.value

      const tickets = new Map(
        response.tickets.map((ticket) => [modelLabTargetKey(ticket), ticket]),
      )
      await Promise.allSettled(
        lanes.value.map(async (lane) => {
          if (cancelledBeforeStart.delete(lane.id)) return
          const ticket = tickets.get(modelLabTargetKey(lane.target))
          if (!ticket) {
            replaceLane(version, lane.id, (current) => ({
              ...current,
              error: 'The API did not authorize this model.',
              status: 'error',
            }))
            return
          }
          await runLane(
            version,
            lane,
            ticket,
            request.messages,
            request.settings,
          )
        }),
      )
    } catch (ticketError) {
      if (version !== runVersion || isAbortError(ticketError))
        return lanes.value
      error.value = toApiErrorMessage(
        ticketError,
        TICKET_ERRORS,
        'The run could not be authorized.',
      )
      lanes.value = lanes.value.map((lane) =>
        lane.status === 'stopped'
          ? lane
          : {
              ...lane,
              error: error.value,
              status: 'error',
            },
      )
      emitLanes()
    } finally {
      if (version === runVersion) {
        authorizationController = null
        onLaneUpdate = null
      }
    }

    return lanes.value
  }

  onScopeDispose(stopAll)

  return {
    cancelLane,
    error: shallowReadonly(error),
    lanes: shallowReadonly(lanes),
    run,
    running,
    stopAll,
  }
}
