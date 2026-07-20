import { aliases } from 'vuetify/iconsets/mdi'
import { defineNuxtConfig } from 'nuxt/config'

const vuetifyAliasIcons = Object.values(aliases).map((icon) =>
  (icon as string).replace(/^mdi-/, 'mdi:'),
)

const runtimeMdiIcons = [
  'mdi:account-cancel-outline',
  'mdi:account-check-outline',
  'mdi:cloud-check-outline',
  'mdi:cloud-off-outline',
  'mdi:chart-box-outline',
  'mdi:delete-outline',
  'mdi:key-chain',
  'mdi:key-plus',
  'mdi:key-remove',
  'mdi:pencil-outline',
  'mdi:server-network',
  'mdi:server-network-off',
  'mdi:shield-account-outline',
  'mdi:shield-check-outline',
  'mdi:shield-off-outline',
] as const

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  ssr: false,
  devtools: { enabled: true },
  modules: [
    '@pinia/nuxt',
    '@vueuse/nuxt',
    'vuetify-nuxt-module',
    'nuxt-echarts',
    '@nuxt/icon',
    '@nuxt/eslint',
    '@nuxt/test-utils/module',
  ],
  css: ['~/assets/styles/index.css'],
  experimental: { typedPages: true },
  typescript: {
    shim: false,
    strict: true,
    tsConfig: {
      include: ['../test/**/*.ts'],
    },
  },
  vue: { propsDestructure: true },
  vueuse: { ssrHandlers: true },
  vuetify: {
    moduleOptions: {
      ssrClientHints: {
        viewportSize: true,
        prefersColorScheme: true,
        prefersColorSchemeOptions: {},
        reloadOnFirstRequest: true,
      },
    },
    vuetifyOptions: './vuetify.config.ts',
  },
  icon: {
    clientBundle: {
      icons: [...new Set([...vuetifyAliasIcons, ...runtimeMdiIcons])],
      scan: true,
    },
    customCollections: [
      {
        prefix: 'custom',
        dir: './app/assets/icons',
      },
    ],
  },
  echarts: {
    charts: ['LineChart', 'BarChart', 'PieChart', 'RadarChart'],
    renderer: 'svg',
    components: [
      'DataZoomComponent',
      'LegendComponent',
      'TooltipComponent',
      'ToolboxComponent',
      'GridComponent',
      'TitleComponent',
      'DatasetComponent',
      'VisualMapComponent',
    ],
  },
  postcss: {
    plugins: {
      cssnano: {
        preset: ['default', { calc: false }],
      },
    },
  },
  vite: {
    build: {
      sourcemap: false,
      cssMinify: 'lightningcss',
      chunkSizeWarningLimit: 1000,
      rollupOptions: {
        onwarn(warning, defaultHandler) {
          const warningId = warning.id?.replaceAll('\\', '/')

          if (
            warning.code === 'INVALID_ANNOTATION' &&
            warningId?.endsWith('/node_modules/@vueuse/core/dist/index.js')
          ) {
            return
          }

          if (
            warning.message.includes('Sourcemap is likely to be incorrect') &&
            warning.message.includes('nuxt:module-preload-polyfill')
          ) {
            return
          }

          defaultHandler(warning)
        },
      },
    },
  },
  runtimeConfig: {
    public: {
      apiBaseUrl: '',
    },
  },
  compatibilityDate: '2024-08-05',
})
