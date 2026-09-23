import { describe, it, expect } from 'vitest'
import { z } from 'zod'
import { zodToFields, email, isoDate, money, tagMap, optionalString, positiveInt } from '@/forms'

describe('zodToFields', () => {
  it('maps every schema shape to a field type with overrides', () => {
    const S = z.object({
      name: z.string(), age: z.number().optional(), on: z.boolean().default(false), kind: z.enum(['a', 'b']),
      born_at: z.string().nullable(), notes: z.string(), password: z.string(), mail: email, when: isoDate, cost: money, n: positiveInt,
      tags: tagMap(3), raw: z.record(z.string(), z.string()), maybe: optionalString(5), either: z.union([z.string(), z.number()]),
      piped: z.string().pipe(z.string().min(1)),
    })
    const f = Object.fromEntries(zodToFields(S, { name: { label: 'Full name', cols: 12 } }).map((x) => [x.key, x]))
    expect(f.name).toMatchObject({ type: 'text', required: true, label: 'Full name', cols: 12 })
    expect(f.age).toMatchObject({ type: 'number', required: false })
    expect(f.on).toMatchObject({ type: 'checkbox', required: false })
    expect(f.kind).toMatchObject({ type: 'select', options: [{ title: 'a', value: 'a' }, { title: 'b', value: 'b' }] })
    expect(f.born_at?.type).toBe('date')
    expect(f.notes).toMatchObject({ type: 'textarea', cols: 12 })
    expect(f.password?.type).toBe('secret')
    expect(f.mail?.type).toBe('text')
    expect(f.when).toMatchObject({ type: 'date', required: false })
    expect(f.cost).toMatchObject({ type: 'number', required: false })
    expect(f.n?.type).toBe('number')
    expect(f.tags?.type).toBe('tags')
    expect(f.raw?.type).toBe('tags')
    expect(f.maybe).toMatchObject({ type: 'text', required: false })
    expect(f.either?.required).toBe(false)
    expect(f.piped?.type).toBe('text')
    expect(f.name?.label).toBe('Full name')
    expect(zodToFields(z.object({ category_id: z.string() }))[0]?.label).toBe('Category')
  })
})
