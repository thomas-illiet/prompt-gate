<script setup lang="ts">
import AdminSubscriptionPlanDialog from '~/components/AdminSubscriptions/AdminSubscriptionPlanDialog.vue'
import AdminSubscriptionPlansTable from '~/components/AdminSubscriptions/AdminSubscriptionPlansTable.vue'
import type {
  SubscriptionPlan,
  SubscriptionPlanPayload,
} from '~/types/subscriptions'

definePageMeta({
  requiredRoles: ['admin'],
  title: 'Subscription plans',
  icon: 'mdi-card-account-details-star-outline',
  drawerIndex: 4,
  drawerSection: 'Access',
})

const adminSubscriptions = useAdminSubscriptions()
const planDialogOpen = shallowRef(false)
const deleteDialog = useTargetDialog<SubscriptionPlan>()
const defaultDialog = useTargetDialog<SubscriptionPlan>()

const totalLabel = computed(() => {
  const total = adminSubscriptions.total.value
  const defaultName = adminSubscriptions.defaultPlan.value?.name
  if (!defaultName) {
    return total === 1 ? '1 plan' : `${total} plans`
  }
  return defaultName
})

function openCreateDialog() {
  adminSubscriptions.selectedPlan.value = null
  planDialogOpen.value = true
}

async function openEditDialog(plan: SubscriptionPlan) {
  await adminSubscriptions.loadPlan(plan.id)
  planDialogOpen.value = true
}

async function savePlan(payload: SubscriptionPlanPayload) {
  if (adminSubscriptions.selectedPlan.value) {
    await adminSubscriptions.updatePlan(
      adminSubscriptions.selectedPlan.value.id,
      payload,
    )
  } else {
    await adminSubscriptions.createPlan(payload)
  }
  planDialogOpen.value = false
}

async function confirmDelete() {
  if (!deleteDialog.target.value) {
    return
  }
  await adminSubscriptions.deletePlan(deleteDialog.target.value.id)
  deleteDialog.close()
}

async function confirmDefault() {
  if (!defaultDialog.target.value) {
    return
  }
  await adminSubscriptions.setDefaultPlan(defaultDialog.target.value.id)
  defaultDialog.close()
}
</script>

<template>
  <v-container fluid class="app-page">
    <v-row>
      <v-col cols="12">
        <AppPageHero
          icon="mdi-card-account-details-star-outline"
          kicker="Usage control"
          title="Subscription plans"
          copy="Define token windows for proxy access and choose the default plan inherited by accounts."
          stat-label="Default plan"
          :stat-value="totalLabel"
        />
      </v-col>

      <v-col cols="12">
        <v-alert
          v-if="adminSubscriptions.listError.value"
          type="warning"
          variant="tonal"
          rounded="lg"
          class="mb-4"
        >
          {{ adminSubscriptions.listError.value }}
        </v-alert>

        <AdminSubscriptionPlansTable
          :items="adminSubscriptions.plans.value"
          :loading="adminSubscriptions.loading.value"
          :page="adminSubscriptions.page.value"
          :page-size="adminSubscriptions.pageSize.value"
          :sort-by="adminSubscriptions.sortBy.value"
          :sort-dir="adminSubscriptions.sortDir.value"
          :total="adminSubscriptions.total.value"
          @create="openCreateDialog"
          @delete="deleteDialog.open"
          @edit="openEditDialog"
          @refresh="adminSubscriptions.reload"
          @set-default="defaultDialog.open"
          @update:page="adminSubscriptions.setPage"
          @update:page-size="adminSubscriptions.setPageSize"
          @update:sort="adminSubscriptions.setSort"
        />
      </v-col>
    </v-row>

    <AdminSubscriptionPlanDialog
      v-model="planDialogOpen"
      :loading="adminSubscriptions.saving.value"
      :plan="adminSubscriptions.selectedPlan.value"
      @save="savePlan"
    />

    <AppConfirmDialog
      v-model="deleteDialog.isOpen.value"
      confirm-color="error"
      confirm-label="Delete plan"
      icon="mdi-delete-alert-outline"
      :loading="adminSubscriptions.saving.value"
      :message="
        deleteDialog.target.value
          ? `Delete subscription plan ${deleteDialog.target.value.name}.`
          : 'Delete this subscription plan.'
      "
      title="Delete subscription plan"
      @cancel="deleteDialog.close"
      @confirm="confirmDelete"
    />

    <AppConfirmDialog
      v-model="defaultDialog.isOpen.value"
      confirm-color="primary"
      confirm-label="Set default"
      icon="mdi-star-check-outline"
      :loading="adminSubscriptions.saving.value"
      :message="
        defaultDialog.target.value
          ? `Use ${defaultDialog.target.value.name} as the default inherited plan.`
          : 'Set this subscription plan as default.'
      "
      title="Set default plan"
      @cancel="defaultDialog.close"
      @confirm="confirmDefault"
    />
  </v-container>
</template>
