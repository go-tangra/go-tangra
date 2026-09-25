import { beforeAll, describe, expect, it } from 'vitest'
import { compile } from '@tailwindcss/node'

// The compiled platform theme: the chosen data-theme must always outrank the
// OS colour scheme, so no prefers-color-scheme rule may target a <html> that
// already carries a data-theme (a light choice on a dark OS rendered dark).
// (not `new URL(…, import.meta.url)`: Vite rewrites that into an asset URL)
const src = decodeURIComponent(import.meta.url.replace(/^file:\/\//, '').replace(/tests\/[^/]*$/, 'src/'))

async function build(): Promise<string> {
  const c = await compile('@import "tailwindcss";\n@import "./theme.css";', { base: src, onDependency: () => {} })
  return c.build([])
}

function mediaBlocks(css: string, query: RegExp): string[] {
  const out: string[] = []
  for (let i = css.search(query); i >= 0; ) {
    const open = css.indexOf('{', i)
    let depth = 0
    let j = open
    for (; j < css.length; j++) {
      if (css[j] === '{') depth++
      else if (css[j] === '}' && --depth === 0) break
    }
    out.push(css.slice(open + 1, j))
    const next = css.slice(j).search(query)
    i = next < 0 ? -1 : j + next
  }
  return out
}

describe('theme.css', () => {
  let css = ''
  // compiling Tailwind + FlyonUI takes a few seconds on a busy machine
  beforeAll(async () => { css = await build() }, 30_000)
  it('lets the chosen data-theme win over prefers-color-scheme', () => {
    const blocks = mediaBlocks(css, /@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)/)
    expect(blocks.length).toBeGreaterThan(0)
    for (const b of blocks) {
      const selectors = [...b.matchAll(/([^{}]+)\{/g)].map((m) => m[1]!.trim())
      for (const s of selectors) expect(s, 'OS-dark rule must not apply once a theme is chosen').toMatch(/:not\(\[data-theme\]\)/)
    }
  })
  it('keeps both platform themes selectable by data-theme', () => {
    expect(css).toMatch(/\[data-theme="?freya-light"?\]\s*[,{]/)
    expect(css).toMatch(/\[data-theme="?freya-dark"?\]\s*\{[^}]*color-scheme:\s*dark/)
  })
})
