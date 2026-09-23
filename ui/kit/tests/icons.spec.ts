import { describe, it, expect } from 'vitest'
import mdi from '@iconify-json/mdi/icons.json'
import { ICONS, iconClass } from '@/icons'

describe('icon safelist', () => {
  it('is sorted, unique and mdi-prefixed', () => {
    expect(ICONS).toEqual([...ICONS].sort())
    expect(new Set(ICONS).size).toBe(ICONS.length)
    for (const n of ICONS) expect(n).toMatch(/^mdi-[a-z0-9-]+$/)
  })
  it('every name exists in the MDI icon set (a missing icon renders as an empty box at runtime)', () => {
    const set = mdi as { icons: Record<string, unknown>; aliases?: Record<string, unknown> }
    const missing = ICONS.map((n) => n.slice(4)).filter((n) => !set.icons[n] && !set.aliases?.[n])
    expect(missing).toEqual([])
  })
  it('maps a name (with or without the mdi- prefix) to the Iconify Tailwind class', () => {
    expect(iconClass('mdi-server')).toBe('icon-[mdi--server]')
    expect(iconClass('server')).toBe('icon-[mdi--server]')
  })
})

describe('icon class safelist', () => {
  it('mirrors the name list one-to-one so Tailwind emits every icon', async () => {
    const { ICON_CLASS_SAFELIST } = await import('@/icons')
    // The fallback glyph (iconClass(undefined)) is safelisted on top of the named icons.
    expect([...ICON_CLASS_SAFELIST].sort()).toEqual([...ICONS.map((n) => iconClass(n)), iconClass(undefined)].sort())
  })
})
