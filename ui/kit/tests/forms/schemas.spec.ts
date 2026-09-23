import { describe, it, expect } from 'vitest'
import { email, uuid, cidr, isoDate, money, nonEmpty, tagMap, slug, optionalString, positiveInt, dateRange, jsonObject } from '@/forms/schemas'
import { z } from 'zod'

const ok = (s: z.ZodType, v: unknown) => expect(s.safeParse(v).success, JSON.stringify(v)).toBe(true)
const bad = (s: z.ZodType, v: unknown) => expect(s.safeParse(v).success, JSON.stringify(v)).toBe(false)

describe('shared schemas', () => {
  it('email', () => {
    ok(email, 'a@b.co')
    ok(email, '  A@B.CO ')
    expect(email.parse('  A@B.CO ')).toBe('a@b.co')
    bad(email, 'nope')
    bad(email, '')
  })
  it('uuid', () => {
    ok(uuid, '11111111-1111-7111-8111-111111111111')
    bad(uuid, 'x')
  })
  it('cidr', () => {
    ok(cidr, '10.0.0.0/8')
    ok(cidr, '2001:db8::/32')
    bad(cidr, '999.1.1.1/8')
    bad(cidr, '10.0.0.0/33')
    bad(cidr, '10.0.0.0')
  })
  it('isoDate normalises to ISO 8601 UTC and accepts blank as undefined', () => {
    expect(isoDate.parse('2025-01-02')).toBe('2025-01-02T00:00:00.000Z')
    expect(isoDate.parse('2025-01-02T10:00:00Z')).toBe('2025-01-02T10:00:00.000Z')
    expect(isoDate.parse('')).toBeUndefined()
    expect(isoDate.parse(undefined)).toBeUndefined()
    bad(isoDate, '2025-13-40')
    bad(isoDate, 'yesterday')
  })
  it('money coerces and bounds', () => {
    expect(money.parse('12.5')).toBe(12.5)
    expect(money.parse('')).toBe(0)
    bad(money, -1)
    bad(money, 'abc')
    bad(money, Infinity)
  })
  it('nonEmpty trims and bounds', () => {
    const s = nonEmpty(5)
    expect(s.parse(' ab ')).toBe('ab')
    bad(s, '   ')
    bad(s, 'toolong')
  })
  it('optionalString maps blank to undefined', () => {
    expect(optionalString(10).parse('  ')).toBeUndefined()
    expect(optionalString(10).parse(' x ')).toBe('x')
    bad(optionalString(2), 'abc')
  })
  it('tagMap limits keys and rejects empty keys', () => {
    const s = tagMap(2)
    ok(s, { a: '1', b: '2' })
    bad(s, { a: '1', b: '2', c: '3' })
    bad(s, { '': '1' })
    expect(s.parse(undefined)).toEqual({})
  })
  it('slug', () => {
    ok(slug, 'my-slug-1')
    bad(slug, 'My Slug')
  })
  it('positiveInt', () => {
    expect(positiveInt.parse('3')).toBe(3)
    bad(positiveInt, -1)
    bad(positiveInt, 1.5)
  })
  it('dateRange refinement', () => {
    const S = dateRange(z.object({ valid_from: isoDate, valid_to: isoDate }), 'valid_from', 'valid_to')
    ok(S, { valid_from: '2025-01-01', valid_to: '2025-02-01' })
    ok(S, { valid_from: '', valid_to: '2025-02-01' })
    const r = S.safeParse({ valid_from: '2025-03-01', valid_to: '2025-02-01' })
    expect(r.success).toBe(false)
    if (!r.success) expect(r.error.issues[0]?.path).toEqual(['valid_to'])
  })
})

describe('jsonObject', () => {
  it('parses objects, treats blank as {}, refuses arrays, scalars and broken JSON', () => {
    expect(jsonObject.parse('{"a":1}')).toEqual({ a: 1 })
    expect(jsonObject.parse('')).toEqual({})
    expect(jsonObject.parse(null)).toEqual({})
    for (const bad of ['[1]', '"x"', '1', '{a:1}', 'null']) expect(jsonObject.safeParse(bad).success, bad).toBe(false)
    const r = jsonObject.safeParse('{')
    expect(r.success ? '' : r.error.issues[0]!.message).toBe('Enter valid JSON.')
  })
})
