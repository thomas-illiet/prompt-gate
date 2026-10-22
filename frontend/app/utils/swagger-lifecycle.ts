import type { ComponentType } from 'react'
import type { Root } from 'react-dom/client'

interface SwaggerPresetContext {
  getSystem: () => {
    getComponent: (name: string, container?: string) => ComponentType
  }
}

interface SwaggerLifecycleOptions {
  createElement: (typeof import('react'))['createElement']
  createRoot: (typeof import('react-dom/client'))['createRoot']
  onRootCreated: (root: Root) => void
}

// createSwaggerLifecyclePreset exposes Swagger UI's otherwise-private React root.
export function createSwaggerLifecyclePreset({
  createElement,
  createRoot,
  onRootCreated,
}: SwaggerLifecycleOptions) {
  return ({ getSystem }: SwaggerPresetContext) => ({
    rootInjects: {
      render(domNode: HTMLElement) {
        const App = getSystem().getComponent('App', 'root')
        const reactRoot = createRoot(domNode)

        onRootCreated(reactRoot)
        reactRoot.render(createElement(App))
      },
    },
  })
}
