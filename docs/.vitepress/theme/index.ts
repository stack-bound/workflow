import DefaultTheme from 'vitepress/theme'
import { h } from 'vue'
import TerminalHero from './components/TerminalHero.vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  Layout() {
    // Drop the faux-terminal dashboard panel in below the home hero, above the
    // feature grid. On non-home pages this slot renders nothing.
    return h(DefaultTheme.Layout, null, {
      'home-hero-after': () => h(TerminalHero),
    })
  },
  enhanceApp({ app }) {
    // Also expose it for direct use inside any markdown page.
    app.component('TerminalHero', TerminalHero)
  },
}
