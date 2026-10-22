import { createElement } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import { createSwaggerLifecyclePreset } from '../../app/utils/swagger-lifecycle'
// @ts-expect-error Swagger UI does not publish types for its browser bundle entry.
import SwaggerUI from '../../node_modules/swagger-ui/dist/swagger-ui-es-bundle-core.js'

describe('Swagger React lifecycle', () => {
  it('captures and unmounts the real React root used by Swagger UI', async () => {
    const host = document.createElement('div')
    const roots: Array<ReturnType<typeof createRoot>> = []
    const lifecyclePreset = createSwaggerLifecyclePreset({
      createElement,
      createRoot,
      onRootCreated: (root) => roots.push(root),
    })
    document.body.append(host)

    const swaggerSystem = SwaggerUI({
      domNode: host,
      persistAuthorization: false,
      presets: [SwaggerUI.presets.apis, lifecyclePreset],
      queryConfigEnabled: false,
      spec: {
        openapi: '3.1.0',
        info: { title: 'Lifecycle test', version: '1.0.0' },
        paths: {},
      },
      validatorUrl: null,
      withCredentials: true,
    } as unknown as Parameters<typeof SwaggerUI>[0])

    await vi.waitFor(() => {
      expect(host.querySelector('.swagger-ui')).not.toBeNull()
    })

    expect(swaggerSystem).not.toHaveProperty('destroy')
    expect(swaggerSystem).not.toHaveProperty('unmount')
    expect(roots).toHaveLength(1)

    roots[0]?.unmount()

    expect(host.childElementCount).toBe(0)
    host.remove()
  })
})
