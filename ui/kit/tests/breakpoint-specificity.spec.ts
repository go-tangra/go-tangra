import { describe, expect, it } from 'vitest'
// @ts-expect-error plain ESM build helper without type declarations
import { boost, boostBreakpoints, rankOf } from '../build/breakpoint-specificity.mjs'

describe('breakpoint specificity (remote stylesheets)', () => {
  it('ranks min-width queries by breakpoint and ignores others', () => {
    expect(rankOf('(width>=40rem)')).toBe(1)
    expect(rankOf('(width >= 48rem)')).toBe(2)
    expect(rankOf('(min-width: 1024px)')).toBe(3)
    expect(rankOf('(width>=96rem)')).toBe(5)
    expect(rankOf('(width>=30rem)')).toBe(1)
    expect(rankOf('(hover:hover)')).toBe(0)
    expect(rankOf('(width<48rem)')).toBe(0)
    expect(rankOf('(prefers-color-scheme:dark)')).toBe(0)
  })
  it('boosts before pseudo-elements', () => {
    expect(boost('.md\\:p-4', 2)).toBe('.md\\:p-4:not(._):not(._)')
    expect(boost('.md\\:placeholder-x::placeholder', 1)).toBe('.md\\:placeholder-x:not(._)::placeholder')
  })
  it('rewrites only rules under min-width media, including nested ones', () => {
    const out = boostBreakpoints('.grid-cols-2{a:b}@media (width>=48rem){.md\\:grid-cols-12{a:c}@media (hover:hover){.md\\:hover\\:x:hover{a:d}}}@media (hover:hover){.hover\\:y:hover{a:e}}@keyframes spin{to{a:f}}')
    expect(out).toContain('.grid-cols-2{a:b}')
    expect(out).toContain('.md\\:grid-cols-12:not(._):not(._){a:c}')
    expect(out).toContain('.md\\:hover\\:x:hover:not(._):not(._){a:d}')
    expect(out).toContain('.hover\\:y:hover{a:e}')
    expect(out).toContain('to{a:f}')
  })
})
