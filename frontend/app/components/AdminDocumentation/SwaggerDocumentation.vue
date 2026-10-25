<script setup lang="ts">
import 'swagger-ui/dist/swagger-ui.css'

import type { Root } from 'react-dom/client'

import { toApiErrorMessage } from '~/utils/api-error'
import { resolveRuntimeApiBaseUrl } from '~/utils/auth'
import { createSwaggerLifecyclePreset } from '~/utils/swagger-lifecycle'
import {
  rebaseSwaggerRequest,
  type SwaggerRequest,
} from '~/utils/swagger-request'

type OpenAPIDocument = Record<string, unknown>

const apiFetch = useApiFetch()
const runtimeConfig = useRuntimeConfig()
const swaggerRoot = useTemplateRef<HTMLDivElement>('swaggerRoot')
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
        apiFetch<OpenAPIDocument>('/api/v1/openapi.json'),
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
      requestInterceptor(request: SwaggerRequest) {
        return rebaseSwaggerRequest(
          request,
          resolveRuntimeApiBaseUrl(runtimeConfig.public.apiBaseUrl),
          window.location.origin,
        )
      },
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
    subtitle="Try it out requests reuse your current user session."
  >
    <v-progress-linear v-if="loading" indeterminate />

    <div class="swagger-documentation-content">
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

.swagger-documentation-alert p {
  margin: 0 0 16px;
}

.swagger-documentation {
  min-height: 560px;
  color-scheme: light;
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

@media (max-width: 720px) {
  .swagger-documentation-content {
    padding-inline: 4px;
  }

  .swagger-documentation :deep(.swagger-ui .wrapper) {
    padding-inline: 4px;
  }
}
</style>
