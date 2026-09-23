import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

/** Tailwind breakpoints (px). */
export const BREAKPOINTS = { sm: 640, md: 768, lg: 1024, xl: 1280 } as const
export type Breakpoint = keyof typeof BREAKPOINTS

/**
 * Reactive "viewport is at least <bp> wide". Uses matchMedia so it stays in
 * sync with resizes and needs no ResizeObserver. SSR/jsdom safe.
 */
export function useBreakpoint(bp: Breakpoint = 'md'): Ref<boolean> {
  const q = `(min-width: ${BREAKPOINTS[bp]}px)`
  const media = typeof window !== 'undefined' && window.matchMedia ? window.matchMedia(q) : null
  const matches = ref(media?.matches ?? true)
  const onChange = (e: MediaQueryListEvent) => (matches.value = e.matches)
  onMounted(() => media?.addEventListener?.('change', onChange))
  onBeforeUnmount(() => media?.removeEventListener?.('change', onChange))
  return matches
}

/** True when the user prefers reduced motion. */
export function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}
