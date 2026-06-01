<script setup lang="ts">
import type { DashboardScope, UsageWindow } from '~/types/user-service'

definePageMeta({
  icon: 'mdi-monitor-dashboard',
  title: 'Dashboard',
  drawerIndex: 0,
  requiredRoles: ['user', 'manager', 'admin'],
})

const selectedWindow = shallowRef<UsageWindow>('7d')
const selectedScope = shallowRef<DashboardScope>('self')
const authStore = useAuthStore()
const dashboardRefresh = useDashboardRefresh()
const isAdmin = computed(() => authStore.user?.role === 'admin')
const dashboardScope = computed<DashboardScope>(() =>
  isAdmin.value ? selectedScope.value : 'self',
)

watch(
  isAdmin,
  (value) => {
    if (!value) {
      selectedScope.value = 'self'
    }
  },
  { immediate: true },
)
</script>

<template>
  <v-container fluid class="app-page user-dashboard-page">
    <div class="user-dashboard-page__header">
      <div>
        <h1 class="user-dashboard-page__title">My Dashboard</h1>
        <p class="user-dashboard-page__subtitle">
          Your service usage across the selected period.
        </p>
      </div>

      <div class="user-dashboard-page__controls">
        <DashboardScopeSelect v-if="isAdmin" v-model="selectedScope" />
        <DashboardTimeRangeSelect v-model="selectedWindow" />
        <v-tooltip text="Refresh dashboard" location="top">
          <template #activator="{ props: tooltipProps }">
            <v-btn
              v-bind="tooltipProps"
              class="user-dashboard-page__refresh"
              icon="mdi-refresh"
              color="primary"
              variant="tonal"
              rounded="lg"
              aria-label="Refresh dashboard"
              @click="dashboardRefresh.refresh"
            />
          </template>
        </v-tooltip>
      </div>
    </div>

    <v-row>
      <v-col cols="12" md="4">
        <DashboardTokensKpi :window="selectedWindow" :scope="dashboardScope" />
      </v-col>
      <v-col cols="12" md="4">
        <DashboardMessagesKpi
          :window="selectedWindow"
          :scope="dashboardScope"
        />
      </v-col>
      <v-col cols="12" md="4">
        <DashboardDurationKpi
          :window="selectedWindow"
          :scope="dashboardScope"
        />
      </v-col>
    </v-row>

    <DashboardAdoptionKpis
      v-if="dashboardScope === 'global'"
      :window="selectedWindow"
    />

    <v-row>
      <v-col cols="12">
        <DashboardActivityChart
          :window="selectedWindow"
          :scope="dashboardScope"
        />
      </v-col>
    </v-row>

    <v-row v-if="dashboardScope === 'global'">
      <v-col cols="12">
        <DashboardTopIdentitiesChart :window="selectedWindow" />
      </v-col>
    </v-row>

    <v-row>
      <v-col cols="12" lg="4">
        <DashboardTopModelsChart
          :window="selectedWindow"
          :scope="dashboardScope"
        />
      </v-col>
      <v-col cols="12" lg="4">
        <DashboardTopProviderNamesChart
          :window="selectedWindow"
          :scope="dashboardScope"
        />
      </v-col>
      <v-col cols="12" lg="4">
        <DashboardTopProviderTypesChart
          :window="selectedWindow"
          :scope="dashboardScope"
        />
      </v-col>
    </v-row>
  </v-container>
</template>

<style scoped>
.user-dashboard-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}

.user-dashboard-page__title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
}

.user-dashboard-page__subtitle {
  margin: 4px 0 0;
  color: rgb(var(--app-shell-text-secondary));
}

.user-dashboard-page__controls {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
}

.user-dashboard-page__refresh {
  flex: 0 0 auto;
}

@media (max-width: 720px) {
  .user-dashboard-page__header {
    align-items: stretch;
    flex-direction: column;
  }

  .user-dashboard-page__controls {
    align-items: stretch;
    flex-direction: column;
  }

  .user-dashboard-page__refresh {
    align-self: flex-end;
  }
}
</style>
