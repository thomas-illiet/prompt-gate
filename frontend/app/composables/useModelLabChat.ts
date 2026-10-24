import type { MaybeRefOrGetter } from 'vue'

import { createModelLabLane, useModelLabRunner } from './useModelLabRunner'

import type {
  ModelLabLane,
  ModelLabLimits,
  ModelLabRunSettings,
  ModelLabTarget,
  ModelLabTurn,
} from '~/types/model-lab'
import {
  MODEL_LAB_MAX_PROMPT_BYTES,
  MODEL_LAB_MAX_SELECTED_TARGETS,
  MODEL_LAB_MAX_TARGETS,
  modelLabPromptByteLength,
  modelLabTargetKey,
} from '~/utils/model-lab'
import { buildModelLabContextWindow } from '~/utils/model-lab-context'

let turnSequence = 0

function nextTurnID() {
  turnSequence += 1
  return `model-lab-turn-${turnSequence}`
}

function copySettings(settings: ModelLabRunSettings): ModelLabRunSettings {
  return { ...settings }
}

export function useModelLabChat(
  availableTargets: MaybeRefOrGetter<ModelLabTarget[]>,
  configuredLimits?: MaybeRefOrGetter<ModelLabLimits | null | undefined>,
) {
  const runner = useModelLabRunner()
  const limits = computed<ModelLabLimits>(() => {
    const configured = configuredLimits ? toValue(configuredLimits) : null
    return {
      maxOutputTokens: Math.max(
        1,
        Math.trunc(configured?.maxOutputTokens ?? 4096),
      ),
      maxPromptBytes: Math.max(
        1,
        Math.trunc(configured?.maxPromptBytes ?? MODEL_LAB_MAX_PROMPT_BYTES),
      ),
      maxTargets: Math.min(
        MODEL_LAB_MAX_TARGETS,
        Math.max(1, Math.trunc(configured?.maxTargets ?? 3)),
      ),
    }
  })
  const draft = shallowRef('')
  const turns = shallowRef<ModelLabTurn[]>([])
  const selectedTargets = shallowRef<ModelLabTarget[]>([])
  const temperature = shallowRef(0.7)
  const maxOutputTokens = shallowRef(
    Math.min(1024, limits.value.maxOutputTokens),
  )
  const error = shallowRef<string | null>(null)

  const targets = computed(() => toValue(availableTargets))
  const settings = computed<ModelLabRunSettings>(() => ({
    maxOutputTokens: maxOutputTokens.value,
    temperature: temperature.value,
  }))
  const maxPromptLength = computed(() => limits.value.maxPromptBytes)
  const maxSelectedTargets = computed(() =>
    Math.min(MODEL_LAB_MAX_SELECTED_TARGETS, limits.value.maxTargets),
  )
  const primaryTarget = computed(() => selectedTargets.value[0] ?? null)
  const currentWindow = computed(() =>
    buildModelLabContextWindow({
      maxBytes: limits.value.maxPromptBytes,
      prompt: draft.value,
      settings: settings.value,
      targets: targets.value,
      turns: turns.value,
    }),
  )
  const draftError = computed(() => {
    if (!draft.value.trim()) return ''
    if (!primaryTarget.value) return 'Choose a model before sending a message.'
    if (draft.value.length > maxPromptLength.value) {
      return `Keep the message at or below ${maxPromptLength.value.toLocaleString()} characters.`
    }
    if (modelLabPromptByteLength(draft.value) > limits.value.maxPromptBytes) {
      return `Keep the UTF-8 message at or below ${limits.value.maxPromptBytes.toLocaleString()} bytes.`
    }
    if (!currentWindow.value.fits) {
      return 'This message is too large for the Model Lab request limit.'
    }
    return ''
  })

  watch(
    [targets, maxSelectedTargets],
    ([nextTargets, nextMaximum]) => {
      const availableByKey = new Map(
        nextTargets.map((target) => [modelLabTargetKey(target), target]),
      )
      const retained = selectedTargets.value
        .map((target) => availableByKey.get(modelLabTargetKey(target)))
        .filter((target): target is ModelLabTarget => Boolean(target))
        .slice(0, nextMaximum)
      selectedTargets.value =
        retained.length > 0 ? retained : nextTargets.slice(0, 1)
    },
    { immediate: true },
  )

  watch(
    () => limits.value.maxOutputTokens,
    () => setMaxOutputTokens(maxOutputTokens.value),
  )

  function setDraft(value: string) {
    draft.value = value
  }

  function setSelectedTargets(nextTargets: ModelLabTarget[]) {
    const availableByKey = new Map(
      targets.value.map((target) => [modelLabTargetKey(target), target]),
    )
    const unique = new Map<string, ModelLabTarget>()
    for (const target of nextTargets) {
      const key = modelLabTargetKey(target)
      const available = availableByKey.get(key)
      if (available) unique.set(key, available)
      if (unique.size === maxSelectedTargets.value) break
    }
    selectedTargets.value = [...unique.values()]
  }

  function setTemperature(value: number) {
    temperature.value = Math.min(1, Math.max(0, Number(value) || 0))
  }

  function setMaxOutputTokens(value: number) {
    maxOutputTokens.value = Math.min(
      limits.value.maxOutputTokens,
      Math.max(1, Math.round(Number(value) || 1)),
    )
  }

  function replaceTurn(
    id: string,
    update: (turn: ModelLabTurn) => ModelLabTurn,
  ) {
    turns.value = turns.value.map((turn) =>
      turn.id === id ? update(turn) : turn,
    )
  }

  function replaceTurnLane(turnID: string, lane: ModelLabLane) {
    replaceTurn(turnID, (turn) => {
      if (turn.primary.id === lane.id) return { ...turn, primary: lane }
      return {
        ...turn,
        comparisons: turn.comparisons.map((comparison) =>
          comparison.id === lane.id ? lane : comparison,
        ),
      }
    })
  }

  async function runTurnLanes(
    turnID: string,
    lanes: ModelLabLane[],
    turn: ModelLabTurn,
  ) {
    await runner.run({
      lanes,
      messages: turn.context.map((message) => ({ ...message })),
      onLaneUpdate: (lane) => replaceTurnLane(turnID, lane),
      settings: copySettings(turn.settings),
    })
    error.value = runner.error.value
  }

  async function send() {
    if (
      runner.running.value ||
      !draft.value.trim() ||
      draftError.value ||
      !primaryTarget.value
    ) {
      return
    }

    error.value = null
    const window = currentWindow.value
    const lanes = selectedTargets.value.map(createModelLabLane)
    const primaryLane = lanes[0]
    if (!primaryLane) return
    const turn: ModelLabTurn = {
      comparisons: lanes.slice(1),
      context: window.messages.map((message) => ({ ...message })),
      id: nextTurnID(),
      omittedTurns: window.omittedTurns,
      primary: primaryLane,
      prompt: draft.value.trim(),
      settings: copySettings(settings.value),
    }
    turns.value = [...turns.value, turn]
    draft.value = ''
    await runTurnLanes(turn.id, lanes, turn)
  }

  async function retryLane(turnID: string, laneID: string) {
    if (runner.running.value) return
    const turn = turns.value.find((item) => item.id === turnID)
    if (!turn) return
    const previous =
      turn.primary.id === laneID
        ? turn.primary
        : turn.comparisons.find((lane) => lane.id === laneID)
    if (!previous) return

    error.value = null
    const replacement = createModelLabLane(previous.target)
    replaceTurn(turnID, (current) =>
      current.primary.id === laneID
        ? { ...current, primary: replacement }
        : {
            ...current,
            comparisons: current.comparisons.map((lane) =>
              lane.id === laneID ? replacement : lane,
            ),
          },
    )
    const updatedTurn = turns.value.find((item) => item.id === turnID)
    if (updatedTurn) await runTurnLanes(turnID, [replacement], updatedTurn)
  }

  function cancelLane(id: string) {
    runner.cancelLane(id)
  }

  function stopAll() {
    runner.stopAll()
  }

  function reset() {
    runner.stopAll()
    draft.value = ''
    turns.value = []
    error.value = null
  }

  onScopeDispose(() => {
    runner.stopAll()
    draft.value = ''
    turns.value = []
  })

  return {
    cancelLane,
    draft: shallowReadonly(draft),
    draftError,
    error: shallowReadonly(error),
    maxAllowedOutputTokens: computed(() => limits.value.maxOutputTokens),
    maxOutputTokens: shallowReadonly(maxOutputTokens),
    maxPromptBytes: computed(() => limits.value.maxPromptBytes),
    maxPromptLength,
    maxSelectedTargets,
    primaryTarget: shallowReadonly(primaryTarget),
    reset,
    retryLane,
    running: runner.running,
    send,
    selectedTargets: shallowReadonly(selectedTargets),
    setDraft,
    setMaxOutputTokens,
    setSelectedTargets,
    setTemperature,
    stopAll,
    temperature: shallowReadonly(temperature),
    turns: shallowReadonly(turns),
  }
}
