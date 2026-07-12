<script setup lang="ts">
import AdminPricingConfigurationAlert from '~/components/AdminPricing/AdminPricingConfigurationAlert.vue'
import AdminPricingFallbackCard from '~/components/AdminPricing/AdminPricingFallbackCard.vue'
import AdminPricingModelDialog from '~/components/AdminPricing/AdminPricingModelDialog.vue'
import AdminPricingModelsTable from '~/components/AdminPricing/AdminPricingModelsTable.vue'
import type { ModelPricePayload, ModelPriceRecord } from '~/types/pricing'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Model pricing',
  icon: 'mdi-currency-usd',
  drawerIndex: 6,
  drawerSection: 'Access',
})

const adminPricing = useAdminPricing()
const modelDialogOpen = shallowRef(false)
const selectedPrice = shallowRef<ModelPriceRecord | ModelPricePayload | null>(
  null,
)
const deleteDialog = useTargetDialog<ModelPriceRecord>()

const statusLabel = computed(() => {
  if (adminPricing.providerErrorsCount.value > 0) {
    return `${adminPricing.providerErrorsCount.value} provider errors`
  }
  if (adminPricing.missingPricesCount.value > 0) {
    return `${adminPricing.missingPricesCount.value} missing`
  }
  if (adminPricing.isConfigured.value) {
    return 'Complete'
  }
  return `${adminPricing.configuredModelsCount.value} model prices`
})

function openCreateDialog() {
  selectedPrice.value = null
  modelDialogOpen.value = true
}

function openEditDialog(price: ModelPriceRecord) {
  selectedPrice.value = price
  modelDialogOpen.value = true
}

function openMissingDialog(missing: { providerName: string; model: string }) {
  selectedPrice.value = adminPricing.priceFromMissing(missing)
  modelDialogOpen.value = true
}

async function saveModelPrice(payload: ModelPricePayload) {
  if (
    selectedPrice.value &&
    'id' in selectedPrice.value &&
    selectedPrice.value.id
  ) {
    await adminPricing.updateModelPrice(selectedPrice.value.id, payload)
  } else {
    await adminPricing.createModelPrice(payload)
  }
  modelDialogOpen.value = false
}

async function confirmDelete() {
  if (!deleteDialog.target.value?.id) {
    return
  }
  await adminPricing.deleteModelPrice(deleteDialog.target.value.id)
  deleteDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-currency-usd"
          kicker="Cost intelligence"
          title="Model pricing"
          copy="Configure fallback and model-specific token prices used for dashboard cost estimates."
          stat-label="Pricing status"
          :stat-value="statusLabel"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminPricing.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminPricing.listError.value }}
        </v-alert>

        <AdminPricingConfigurationAlert
          :check="adminPricing.check.value"
          :loading="adminPricing.checking.value"
          @add-missing="openMissingDialog"
          @refresh="adminPricing.loadCheck"
        />
      </v-col>

      <v-col cols="12">
        <AdminPricingFallbackCard
          :fallback="adminPricing.fallback.value"
          :loading="adminPricing.saving.value"
          @save="adminPricing.saveFallback"
        />
      </v-col>

      <v-col cols="12">
        <AdminPricingModelsTable
          :items="adminPricing.models.value"
          :loading="adminPricing.loading.value"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @edit="openEditDialog"
          @refresh="adminPricing.reload"
        />
      </v-col>
    </v-row>

    <AdminPricingModelDialog
      v-model="modelDialogOpen"
      :existing-prices="adminPricing.models.value"
      :loading="adminPricing.saving.value"
      :model-catalog="adminPricing.modelCatalog.value"
      :options-loading="adminPricing.optionsLoading.value"
      :price="selectedPrice"
      :providers="adminPricing.providerOptions.value"
      @save="saveModelPrice"
    />

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete price"
      icon="mdi-delete-alert-outline"
      :loading="adminPricing.saving.value"
      :message="
        deleteDialog.target.value
          ? `Delete pricing for ${deleteDialog.target.value.providerName} / ${deleteDialog.target.value.model}.`
          : 'Delete this pricing configuration.'
      "
      title="Delete model price"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />
  </v-container>
</template>
