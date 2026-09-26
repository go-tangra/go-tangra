import { beforeAll, describe, expect, it } from 'vitest'
import { compile } from '@tailwindcss/node'
import sfc from '../src/components/UiToast.vue?raw'

// The toast host must be positioned by classes the platform CSS defines.
// FlyonUI (the component layer) has no `toast` component, so a host built on
// daisyUI's `toast` classes rendered in the page flow, below the viewport.
const src = decodeURIComponent(import.meta.url.replace(/^file:\/\//, '').replace(/tests\/[^/]*$/, 'src/'))

function hostClasses(): string[] {
  const m = sfc.match(/<template>\s*<div class="([^"]+)"/)
  expect(m, 'UiToast host element').not.toBeNull()
  return m![1]!.split(/\s+/).filter(Boolean)
}

const escape = (c: string) => c.replace(/[[\]:/.%()]/g, (ch) => '\\' + ch)

describe('UiToast host CSS', () => {
  let css = ''
  const classes = hostClasses()
  beforeAll(async () => {
    const c = await compile('@import "tailwindcss";\n@import "./theme.css";', { base: src, onDependency: () => {} })
    css = c.build(classes)
  }, 30_000)
  it('uses only classes the platform CSS defines', () => {
    for (const cls of classes) expect(css, cls).toContain('.' + escape(cls))
  })
  it('is fixed to the viewport', () => {
    expect(classes).toContain('fixed')
    expect(css).toMatch(/\.fixed\s*\{[^}]*position:\s*fixed/)
  })
})
