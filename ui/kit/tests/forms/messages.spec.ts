import { describe, it, expect, vi } from 'vitest'
import { z } from 'zod'
import { messages, describeReason, describeIssue, API_REASONS } from '@/forms/messages'

describe('messages', () => {
  it('maps every API reason to text and unknown reasons to the generic fallback', () => {
    for (const r of API_REASONS) expect(messages.reason[r]).toBeTruthy()
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    expect(describeReason('something_internal_with_stack')).toBe(messages.reason.generic)
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('unknown api reason'), 'something_internal_with_stack')
    warn.mockRestore()
  })
  it('never echoes a server detail string into the user text', () => {
    const txt = describeReason('SELECT * FROM users WHERE secret=1')
    expect(txt).not.toContain('SELECT')
  })
  it('maps every Zod issue code', () => {
    const S = z.object({
      a: z.string().min(2).max(3),
      b: z.email(),
      c: z.number().min(1).max(5).multipleOf(1),
      d: z.enum(['x', 'y']),
      e: z.string(),
      f: z.uuid(),
      g: z.string().regex(/^A/),
    }).strict()
    const res = S.safeParse({ a: 'x', b: 'nope', c: 9, d: 'z', f: 'bad', g: 'B', extra: 1 })
    expect(res.success).toBe(false)
    if (res.success) return
    const seen = new Set<string>()
    for (const issue of res.error.issues) {
      seen.add(issue.code)
      const t = describeIssue(issue)
      expect(typeof t).toBe('string')
      expect(t.length).toBeGreaterThan(0)
    }
    expect(seen.has('invalid_type')).toBe(true) // e missing → required
    expect(describeIssue({ code: 'invalid_type', expected: 'string', input: undefined, path: ['e'], message: '' } as never)).toBe(messages.required)
    expect(describeIssue({ code: 'custom', path: [], message: 'valid_to precedes valid_from' } as never)).toBe('valid_to precedes valid_from')
    expect(describeIssue({ code: 'unknown_code_xyz', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'unknown_code_xyz', path: [], message: 'Authored wording.' } as never)).toBe('Authored wording.')
  })
})

describe('messages (all branches)', () => {
  it('covers every issue variant', () => {
    const arr = z.object({ a: z.array(z.string()).min(2).max(3), s: z.set(z.string()).min(1), n: z.number().min(2), b: z.bigint().max(1n), u: z.union([z.string(), z.number()]), k: z.record(z.enum(['x']), z.string()), e: z.array(z.number()).element })
    for (const v of [{ a: ['x'], s: new Set(), n: 1, b: 5n, u: true, k: { bad: 'v' } }, { a: ['x', 'y', 'z', 'w'], s: new Set(['a']), n: 3, b: 0n, u: 'ok', k: { x: 'v' } }]) {
      const r = arr.safeParse(v)
      if (!r.success) for (const i of r.error.issues) expect(describeIssue(i)).toBeTruthy()
    }
    expect(describeIssue({ code: 'too_small', minimum: 1, origin: 'string', path: [], message: '' } as never)).toBe(messages.required)
    expect(describeIssue({ code: 'too_small', minimum: 3, origin: 'string', path: [], message: '' } as never)).toBe(messages.tooShort(3))
    expect(describeIssue({ code: 'too_small', minimum: 2, origin: 'array', path: [], message: '' } as never)).toBe(messages.tooFew(2))
    expect(describeIssue({ code: 'too_small', minimum: 2, origin: 'number', path: [], message: '' } as never)).toBe(messages.tooSmall(2))
    expect(describeIssue({ code: 'too_big', maximum: 3, origin: 'string', path: [], message: '' } as never)).toBe(messages.tooLong(3))
    expect(describeIssue({ code: 'too_big', maximum: 3, origin: 'set', path: [], message: '' } as never)).toBe(messages.tooMany(3))
    expect(describeIssue({ code: 'too_big', maximum: 3, origin: 'number', path: [], message: '' } as never)).toBe(messages.tooBig(3))
    expect(describeIssue({ code: 'invalid_format', format: 'email', path: [], message: '' } as never)).toBe(messages.format.email)
    expect(describeIssue({ code: 'invalid_format', format: 'weird', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'invalid_format', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'not_multiple_of', divisor: 5, path: [], message: '' } as never)).toBe(messages.notMultiple(5))
    expect(describeIssue({ code: 'invalid_value', path: [], message: '' } as never)).toBe(messages.invalidValue)
    expect(describeIssue({ code: 'unrecognized_keys', path: [], message: '' } as never)).toBe(messages.unrecognized)
    expect(describeIssue({ code: 'custom', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'invalid_union', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'invalid_key', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'invalid_element', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'invalid_type', input: 'x', path: [], message: '' } as never)).toBe(messages.invalid)
    expect(describeIssue({ code: 'invalid_type', input: null, path: [], message: '' } as never)).toBe(messages.required)
  })
})

describe('describeIssue (authored messages)', () => {
  it('keeps a message the schema author wrote and replaces Zod defaults', () => {
    const authored = z.string().regex(/^a$/, 'Use a.').safeParse('b')
    expect(describeIssue(authored.success ? ({} as never) : authored.error.issues[0]!)).toBe('Use a.')
    const dflt = z.string().regex(/^a$/).safeParse('b')
    expect(describeIssue(dflt.success ? ({} as never) : dflt.error.issues[0]!)).toBe(messages.format.regex ?? messages.invalid)
    const min = z.string().min(3, 'At least three.').safeParse('b')
    expect(describeIssue(min.success ? ({} as never) : min.error.issues[0]!)).toBe('At least three.')
  })
})

describe('registerReasons', () => {
  it('adds module wording without echoing unknown reasons', async () => {
    const { registerReasons } = await import('@/forms/messages')
    registerReasons({ vault_unavailable: 'The vault is unavailable.' })
    expect(describeReason('vault_unavailable')).toBe('The vault is unavailable.')
    expect(describeReason('still_unknown')).toBe(messages.reason.generic)
  })
})
