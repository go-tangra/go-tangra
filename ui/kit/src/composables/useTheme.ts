import { ref, watch } from 'vue'

export type Theme = 'freya-light' | 'freya-dark'
export const THEME_KEY = 'freya.theme'
const THEMES: Theme[] = ['freya-light', 'freya-dark']

const current = ref<Theme>(readStored() ?? systemTheme())

function readStored(): Theme | undefined {
  try {
    const v = typeof localStorage !== 'undefined' ? localStorage.getItem(THEME_KEY) : null
    return THEMES.includes(v as Theme) ? (v as Theme) : undefined
  } catch {
    return undefined
  }
}

function systemTheme(): Theme {
  return typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'freya-dark' : 'freya-light'
}

function apply(t: Theme) {
  if (typeof document !== 'undefined') document.documentElement.setAttribute('data-theme', t)
}

/**
 * Platform theme: one shared ref, applied as data-theme on <html> (the
 * FlyonUI theme selector) and persisted per browser. The shell and the
 * console call it once at boot; remotes only read `theme`/`isDark`.
 */
export function useTheme() {
  apply(current.value)
  watch(current, (t) => {
    apply(t)
    try {
      localStorage.setItem(THEME_KEY, t)
    } catch {
      /* storage may be unavailable */
    }
  })
  const set = (t: Theme) => (current.value = t)
  const toggle = () => set(current.value === 'freya-dark' ? 'freya-light' : 'freya-dark')
  return { theme: current, isDark: () => current.value === 'freya-dark', set, toggle, themes: THEMES }
}
