import { expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { axe } from 'vitest-axe'
import type { Component } from 'vue'

export function setViewport(px: number) {
  ;(globalThis as unknown as { __vw: number }).__vw = px
}

/** Mounts to document.body (needed for Teleport, focus and axe). */
export function mountBody<T extends Component>(comp: T, opts: Record<string, unknown> = {}) {
  const el = document.createElement('div')
  document.body.appendChild(el)
  return mount(comp, { attachTo: el, ...opts } as unknown as Parameters<typeof mount<T>>[1])
}

/** Asserts no axe violations in both themes; also asserts no inline style attributes. */
export async function expectA11y(root: Element) {
  expect(root.querySelectorAll('[style]').length, 'no style attributes (CSP)').toBe(0)
  for (const theme of ['freya-light', 'freya-dark']) {
    document.documentElement.setAttribute('data-theme', theme)
    const results = await axe(root as HTMLElement, { rules: { 'color-contrast': { enabled: false }, region: { enabled: false } } })
    ;(expect(results) as unknown as { toHaveNoViolations: () => void }).toHaveNoViolations()
  }
}

export const tick = () => new Promise((r) => setTimeout(r, 0))
