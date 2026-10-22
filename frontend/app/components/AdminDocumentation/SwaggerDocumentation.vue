<script setup lang="ts">
import 'swagger-ui/dist/swagger-ui.css'

import type { Root } from 'react-dom/client'

import { toApiErrorMessage } from '~/utils/api-error'
import { createSwaggerLifecyclePreset } from '~/utils/swagger-lifecycle'

type OpenAPIDocument = Record<string, unknown>

const apiFetch = useApiFetch()
const theme = useVTheme()
const swaggerRoot = useTemplateRef<HTMLDivElement>('swaggerRoot')
const isDark = computed(() => theme.current.value.dark)
const initializationError = shallowRef<string | null>(null)
const loading = shallowRef(false)

let initializationRun = 0
let swaggerReactRoot: Root | null = null

// unmountSwagger runs React's supported teardown before clearing its host.
function unmountSwagger() {
  const reactRoot = swaggerReactRoot
  swaggerReactRoot = null

  try {
    reactRoot?.unmount()
  } finally {
    swaggerRoot.value?.replaceChildren()
  }
}

// initializeSwagger loads the protected OpenAPI document before mounting Swagger UI.
async function initializeSwagger() {
  const currentRun = ++initializationRun
  loading.value = true
  initializationError.value = null
  unmountSwagger()

  try {
    const [spec, swaggerModule, reactModule, reactDOMModule] =
      await Promise.all([
        apiFetch<OpenAPIDocument>('/api/v1/admin/openapi.json'),
        import('swagger-ui'),
        import('react'),
        import('react-dom/client'),
      ])

    if (currentRun !== initializationRun || !swaggerRoot.value) {
      return
    }

    const lifecyclePreset = createSwaggerLifecyclePreset({
      createElement: reactModule.createElement,
      createRoot: reactDOMModule.createRoot,
      onRootCreated(reactRoot) {
        swaggerReactRoot = reactRoot
      },
    })

    // Swagger UI accepts null to disable validation; its community types only allow strings.
    const swaggerOptions = {
      deepLinking: true,
      displayRequestDuration: true,
      filter: true,
      domNode: swaggerRoot.value,
      persistAuthorization: false,
      presets: [swaggerModule.default.presets.apis, lifecyclePreset],
      queryConfigEnabled: false,
      spec,
      supportedSubmitMethods: ['get', 'post', 'put', 'patch', 'delete'],
      validatorUrl: null,
      withCredentials: true,
    }

    swaggerModule.default(
      swaggerOptions as unknown as Parameters<typeof swaggerModule.default>[0],
    )
  } catch (error) {
    if (currentRun !== initializationRun) {
      return
    }

    unmountSwagger()
    initializationError.value = toApiErrorMessage(
      error,
      {},
      'The API documentation could not be loaded.',
    )
  } finally {
    if (currentRun === initializationRun) {
      loading.value = false
    }
  }
}

onMounted(() => {
  void initializeSwagger()
})

onBeforeUnmount(() => {
  initializationRun += 1
  unmountSwagger()
})
</script>

<template>
  <AppSectionCard
    icon="mdi-api"
    title="Interactive API explorer"
    subtitle="Try it out requests reuse your current administrator session."
  >
    <v-progress-linear v-if="loading" indeterminate />

    <div class="swagger-documentation-content">
      <v-alert
        type="warning"
        variant="tonal"
        title="Requests can change data"
        class="swagger-documentation-warning"
      >
        Try it out sends real requests to PromptGate. POST, PUT, PATCH, and
        DELETE operations can change or remove data in the current environment
        immediately.
      </v-alert>

      <v-alert
        v-if="initializationError"
        type="error"
        variant="tonal"
        title="API documentation unavailable"
        class="swagger-documentation-alert"
      >
        <p>{{ initializationError }}</p>
        <v-btn
          variant="tonal"
          prepend-icon="mdi-refresh"
          :disabled="loading"
          :loading="loading"
          @click="initializeSwagger"
        >
          Retry
        </v-btn>
      </v-alert>

      <div
        v-show="!initializationError"
        ref="swaggerRoot"
        class="swagger-documentation"
        :class="{ 'swagger-documentation--dark': isDark }"
      />
    </div>
  </AppSectionCard>
</template>

<style scoped>
.swagger-documentation-content {
  min-width: 0;
  padding: 0 16px 20px;
}

.swagger-documentation-alert {
  margin: 20px 8px 4px;
}

.swagger-documentation-warning {
  margin: 20px 8px 4px;
}

.swagger-documentation-alert p {
  margin: 0 0 16px;
}

.swagger-documentation {
  min-height: 560px;
  color-scheme: light;
}

.swagger-documentation--dark {
  color-scheme: dark;
}

.swagger-documentation :deep(.swagger-ui) {
  color: rgb(var(--app-shell-text-primary));
  background: transparent;
  font-family: inherit;
}

.swagger-documentation :deep(.swagger-ui .wrapper) {
  max-width: none;
  padding-inline: 8px;
}

.swagger-documentation :deep(.swagger-ui .info) {
  margin: 32px 0;
}

.swagger-documentation :deep(.swagger-ui .scheme-container) {
  margin: 0 0 24px;
  border: 1px solid rgba(var(--app-shell-border), 0.52);
  border-radius: var(--app-card-radius);
  background: rgba(var(--app-shell-surface-muted), 0.72);
  box-shadow: none;
}

.swagger-documentation :deep(.swagger-ui .models) {
  border-color: rgba(var(--app-shell-border), 0.7);
  border-radius: var(--app-card-radius);
}

.swagger-documentation :deep(.swagger-ui .topbar) {
  display: none;
}

.swagger-documentation :deep(.swagger-ui .information-container),
.swagger-documentation :deep(.swagger-ui .auth-wrapper) {
  display: none;
}

.swagger-documentation--dark :deep(.swagger-ui),
.swagger-documentation--dark :deep(.swagger-ui .opblock-tag),
.swagger-documentation--dark :deep(.swagger-ui .opblock-summary-description),
.swagger-documentation--dark :deep(.swagger-ui .opblock-summary-path),
.swagger-documentation--dark :deep(.swagger-ui .opblock-section-header h4),
.swagger-documentation--dark :deep(.swagger-ui .opblock-section-header label),
.swagger-documentation--dark :deep(.swagger-ui .opblock-description-wrapper p),
.swagger-documentation--dark :deep(.swagger-ui .parameter__name),
.swagger-documentation--dark :deep(.swagger-ui .parameter__type),
.swagger-documentation--dark :deep(.swagger-ui .response-col_status),
.swagger-documentation--dark :deep(.swagger-ui .response-col_links),
.swagger-documentation--dark :deep(.swagger-ui .responses-inner h4),
.swagger-documentation--dark :deep(.swagger-ui .responses-inner h5),
.swagger-documentation--dark :deep(.swagger-ui table thead tr th),
.swagger-documentation--dark :deep(.swagger-ui .model-title),
.swagger-documentation--dark :deep(.swagger-ui .model),
.swagger-documentation--dark :deep(.swagger-ui section.models h4) {
  color: rgb(var(--app-shell-text-primary));
}

.swagger-documentation--dark :deep(.swagger-ui .opblock),
.swagger-documentation--dark :deep(.swagger-ui .opblock-section-header),
.swagger-documentation--dark :deep(.swagger-ui .model-container),
.swagger-documentation--dark :deep(.swagger-ui .model-box) {
  background: rgba(var(--app-shell-surface-muted), 0.72);
}

.swagger-documentation--dark :deep(.swagger-ui input),
.swagger-documentation--dark :deep(.swagger-ui select),
.swagger-documentation--dark :deep(.swagger-ui textarea) {
  border-color: rgba(var(--app-shell-border), 0.72);
  color: rgb(var(--app-shell-text-primary));
  background: rgb(var(--app-shell-surface));
}

.swagger-documentation--dark :deep(.swagger-ui svg) {
  fill: rgb(var(--app-shell-text-secondary));
}

@media (max-width: 720px) {
  .swagger-documentation-content {
    padding-inline: 4px;
  }

  .swagger-documentation :deep(.swagger-ui .wrapper) {
    padding-inline: 4px;
  }
}
</style>
