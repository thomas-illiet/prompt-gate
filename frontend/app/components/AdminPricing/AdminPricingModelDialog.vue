<script setup lang="ts">
import type { ModelPricePayload, ModelPriceRecord } from '~/types/pricing'
import type { Provider, ProviderModelCatalog } from '~/types/providers'

const props = defineProps<{
  existingPrices: ModelPriceRecord[]
  loading: boolean
  modelCatalog: ProviderModelCatalog[]
  optionsLoading: boolean
  price: ModelPriceRecord | ModelPricePayload | null
  providers: Provider[]
}>()

const emit = defineEmits<{
  save: [payload: ModelPricePayload]
}>()

const isOpen = defineModel<boolean>({ default: false })
const providerName = shallowRef('')
const model = shallowRef('')
const input = shallowRef(0)
const output = shallowRef(0)
const hasSubmitted = shallowRef(false)

const isEditingModelPrice = computed(() =>
  Boolean(props.price && 'id' in props.price),
)
const title = computed(() =>
  isEditingModelPrice.value ? 'Edit model price' : 'Create model price',
)
const submitLabel = computed(() =>
  isEditingModelPrice.value ? 'Save price' : 'Create price',
)
const providerItems = computed(() =>
  props.providers.map((provider) => ({
    title: provider.displayName || provider.name,
    value: provider.name,
    props: {
      subtitle: provider.name,
    },
  })),
)
const providerNames = computed(
  () => new Set(props.providers.map((provider) => provider.name)),
)
const selectedProviderCatalog = computed(() =>
  props.modelCatalog.find((provider) => provider.name === providerName.value),
)
const existingPriceKeys = computed(() => {
  const currentID =
    props.price && 'id' in props.price ? (props.price.id ?? '') : ''
  return new Set(
    props.existingPrices
      .filter((price) => !currentID || price.id !== currentID)
      .map((price) => modelPriceKey(price.providerName, price.model)),
  )
})
const modelItems = computed(() => {
  const currentModel =
    props.price?.providerName === providerName.value ? props.price.model : ''
  const available = (selectedProviderCatalog.value?.models ?? []).filter(
    (catalogModel) =>
      !existingPriceKeys.value.has(
        modelPriceKey(providerName.value, catalogModel),
      ) || catalogModel === currentModel,
  )
  if (currentModel && !available.includes(currentModel)) {
    available.unshift(currentModel)
  }
  return [...new Set(available)].sort((left, right) =>
    left.localeCompare(right),
  )
})
const modelNames = computed(() => new Set(modelItems.value))
const modelNoDataText = computed(() => {
  if (!providerName.value) {
    return 'Select a provider first.'
  }
  if (selectedProviderCatalog.value?.modelsError) {
    return selectedProviderCatalog.value.modelsError
  }
  return 'No unpriced models found.'
})
const providerNameError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }
  if (!providerName.value.trim()) {
    return 'Provider name is required.'
  }
  if (!providerNames.value.has(providerName.value)) {
    return 'Select an existing provider.'
  }
  return ''
})
const modelError = computed(() => {
  if (!hasSubmitted.value) {
    return ''
  }
  if (!model.value.trim()) {
    return 'Model is required.'
  }
  if (!modelNames.value.has(model.value)) {
    return 'Select an available model.'
  }
  return ''
})
const inputError = computed(() => priceError(input.value))
const outputError = computed(() => priceError(output.value))
const canSave = computed(
  () =>
    !providerNameError.value &&
    !modelError.value &&
    !inputError.value &&
    !outputError.value,
)
const formId = useId()

watch(
  [isOpen, () => props.price],
  ([open]) => {
    if (!open) {
      return
    }
    providerName.value = props.price?.providerName ?? ''
    model.value = props.price?.model ?? ''
    input.value = normalizePriceValue(props.price?.input ?? 0)
    output.value = normalizePriceValue(props.price?.output ?? 0)
    hasSubmitted.value = false
  },
  { immediate: true },
)

watch([isOpen, providerName, modelItems], ([open]) => {
  if (!open) {
    return
  }
  if (model.value && modelNames.value.has(model.value)) {
    return
  }
  model.value = modelItems.value[0] ?? ''
})

function updateInputPrice(value: number | string) {
  input.value = normalizePriceValue(value)
}

function updateOutputPrice(value: number | string) {
  output.value = normalizePriceValue(value)
}

function normalizePriceValue(value: number | string) {
  const price = Number(value)
  if (!Number.isFinite(price) || price < 0) {
    return 0
  }
  return price
}

function priceError(value: number) {
  if (!hasSubmitted.value) {
    return ''
  }
  if (!Number.isFinite(Number(value)) || Number(value) < 0) {
    return 'Use a positive value or zero.'
  }
  return ''
}

function modelPriceKey(priceProviderName: string, priceModel: string) {
  return `${priceProviderName.trim()}\u0000${priceModel.trim()}`
}

function save() {
  hasSubmitted.value = true
  if (!canSave.value) {
    return
  }
  emit('save', {
    providerName: providerName.value.trim(),
    model: model.value.trim(),
    input: Number(input.value),
    output: Number(output.value),
  })
}
</script>

<template>
  <AppDialogCard v-model="isOpen" icon="mdi-cash-multiple" :loading="props.loading" max-width="680" subtitle="Set the input and output cost used to estimate model usage." :title="title">
      <form :id="formId" @submit.prevent="save">
          <v-row>
            <v-col cols="12" md="6">
              <v-select
                v-model="providerName"
                :items="providerItems"
                label="Provider name"
                variant="outlined"
                density="comfortable"
                :disabled="isEditingModelPrice || props.loading"
                :loading="props.optionsLoading"
                no-data-text="No providers found"
                :error="Boolean(providerNameError)"
                :error-messages="providerNameError ? [providerNameError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-select
                v-model="model"
                :items="modelItems"
                label="Model"
                variant="outlined"
                density="comfortable"
                :disabled="
                  isEditingModelPrice ||
                  !providerName ||
                  props.loading ||
                  props.optionsLoading
                "
                :loading="props.optionsLoading"
                :no-data-text="modelNoDataText"
                :error="Boolean(modelError)"
                :error-messages="modelError ? [modelError] : []"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                :model-value="input"
                label="Input USD / 1M tokens"
                type="number"
                min="0"
                step="0.000001"
                variant="outlined"
                density="comfortable"
                :error="Boolean(inputError)"
                :error-messages="inputError ? [inputError] : []"
                @update:model-value="updateInputPrice"
              />
            </v-col>

            <v-col cols="12" md="6">
              <v-text-field
                :model-value="output"
                label="Output USD / 1M tokens"
                type="number"
                min="0"
                step="0.000001"
                variant="outlined"
                density="comfortable"
                :error="Boolean(outputError)"
                :error-messages="outputError ? [outputError] : []"
                @update:model-value="updateOutputPrice"
              />
            </v-col>
          </v-row>
      </form>
      <template #actions>
          <AppDialogCloseButton :disabled="props.loading" label="Cancel" @click="isOpen = false" />
          <AppDialogActionButton
            color="primary"
            :form="formId"
            :label="submitLabel"
            type="submit"
            :loading="props.loading"
          />
      </template>
  </AppDialogCard>
</template>

<style scoped>
.admin-pricing-model-dialog {
  border: 1px solid rgba(var(--app-shell-border), 0.45);
}

.admin-pricing-model-dialog__form {
  display: contents;
}
</style>
