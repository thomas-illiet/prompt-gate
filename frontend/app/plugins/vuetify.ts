import type { IconProps } from 'vuetify'
import { Icon } from '#components'
import { aliases } from 'vuetify/iconsets/mdi'

import { promptGateLightThemeColors } from '~/theme/palette'

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.hook('vuetify:configuration', ({ vuetifyOptions }) => {
    const themeOptions =
      vuetifyOptions.theme === false ? {} : (vuetifyOptions.theme ?? {})

    vuetifyOptions.icons = {
      defaultSet: 'nuxtIcon',
      sets: {
        nuxtIcon: {
          component: ({ icon, tag, ...rest }: IconProps) =>
            h(tag, rest, [
              h(Icon, { name: (aliases[icon as string] as string) ?? icon }),
            ]),
        },
      },
      aliases,
    }
    vuetifyOptions.theme = {
      ...themeOptions,
      defaultTheme: 'light',
      themes: {
        light: {
          ...(themeOptions.themes?.light ?? {}),
          colors: {
            ...(themeOptions.themes?.light?.colors ?? {}),
            ...promptGateLightThemeColors,
          },
        },
      },
    }
  })
})
