import { createApp } from 'vue'
import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { aliases, mdi } from 'vuetify/iconsets/mdi'
import App from './App.vue'
import './style.css'

const vuetify = createVuetify({
  components,
  directives,
  icons: {
    defaultSet: 'mdi',
    aliases,
    sets: { mdi }
  },
  theme: {
    defaultTheme: 'lyricSyncDark',
    themes: {
      lyricSyncDark: {
        dark: true,
        colors: {
          background: '#101114',
          surface: '#1a1c20',
          primary: '#77b7ad',
          secondary: '#d6aa4c',
          accent: '#a890e8',
          success: '#6fc58a',
          warning: '#d7a83a',
          error: '#ff8276',
          info: '#7fb5ff'
        }
      }
    }
  },
  defaults: {
    VBtn: {
      rounded: 'md'
    },
    VCard: {
      rounded: 'md'
    },
    VTextField: {
      color: 'primary'
    },
    VTextarea: {
      color: 'primary'
    },
    VSwitch: {
      color: 'primary',
      inset: true
    }
  }
})

createApp(App).use(vuetify).mount('#app')
