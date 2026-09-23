import { expect, afterEach } from 'vitest'
import * as axeMatchers from 'vitest-axe/matchers'
import { config } from '@vue/test-utils'

expect.extend(axeMatchers)
afterEach(() => {
  document.body.innerHTML = ''
  document.body.className = ''
})

// Silence Vue's "ResizeObserver" absence in jsdom for layout composables.
class RO {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as unknown as { ResizeObserver: typeof RO }).ResizeObserver = RO
// matchMedia stub: desktop by default; tests override via setViewport().
;(globalThis as unknown as { __vw: number }).__vw = 1280
window.matchMedia = (q: string) => {
  const m = q.match(/min-width:\s*(\d+)px/)
  const vw = (globalThis as unknown as { __vw: number }).__vw
  const matches = m ? vw >= Number(m[1]) : q.includes('prefers-reduced-motion') ? false : false
  return { matches, media: q, onchange: null, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => false } as MediaQueryList
}
config.global.stubs = {}
