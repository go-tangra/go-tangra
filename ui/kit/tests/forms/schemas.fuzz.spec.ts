import { describe, it, expect } from 'vitest'
import { email, uuid, cidr, isoDate, money, nonEmpty, tagMap, slug, positiveInt } from '@/forms/schemas'

// Property-style fuzz: random inputs never throw, and every accepted value is
// normalised (no surrounding whitespace, finite numbers, ISO dates).
function rnd(seed: number) {
  let s = seed
  return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 2 ** 32)
}
const alphabet = 'abcXYZ019 .@-_/:\n\té中{}[]"\\<>'
function randString(r: () => number) {
  const n = Math.floor(r() * 40)
  let out = ''
  for (let i = 0; i < n; i++) out += alphabet[Math.floor(r() * alphabet.length)]
  return out
}

describe('schema fuzz', () => {
  const r = rnd(42)
  it('string schemas never throw and normalise', () => {
    for (let i = 0; i < 2000; i++) {
      const v = randString(r)
      for (const s of [email, uuid, cidr, slug, nonEmpty(20)]) {
        const res = s.safeParse(v)
        if (res.success) expect(res.data).toBe(res.data.trim())
      }
      const d = isoDate.safeParse(v)
      if (d.success && d.data !== undefined) expect(d.data).toMatch(/^\d{4}-\d{2}-\d{2}T/)
    }
  })
  it('numeric schemas never throw and stay finite', () => {
    const inputs: unknown[] = ['', ' 1 ', '1e309', '-0', 'NaN', 'Infinity', 1e308, -1, 0.5, null, undefined, {}, [], '0x10']
    for (let i = 0; i < 200; i++) inputs.push(String((r() - 0.5) * 1e6))
    for (const v of inputs) {
      const m = money.safeParse(v)
      if (m.success) expect(Number.isFinite(m.data) && m.data >= 0).toBe(true)
      const p = positiveInt.safeParse(v)
      if (p.success) expect(Number.isInteger(p.data) && p.data >= 0).toBe(true)
    }
  })
  it('tagMap never throws', () => {
    for (let i = 0; i < 300; i++) {
      const o: Record<string, unknown> = {}
      const n = Math.floor(r() * 5)
      for (let k = 0; k < n; k++) o[randString(r)] = r() > 0.5 ? randString(r) : 1
      expect(() => tagMap(3).safeParse(o)).not.toThrow()
    }
  })
})
