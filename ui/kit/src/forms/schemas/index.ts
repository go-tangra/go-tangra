// Shared Zod primitives. Every module form composes these so validation
// wording and normalisation are identical platform-wide. Outputs are always
// normalised (trimmed, blank → undefined, dates → ISO UTC).
import { z } from 'zod'

/** Trimmed, lower-cased e-mail. */
export const email = z.string().trim().toLowerCase().pipe(z.email()).meta({ kind: 'email' })

/** Canonical UUID (any version). */
export const uuid = z.string().trim().pipe(z.uuid())

/** IPv4 or IPv6 network in CIDR notation. */
export const cidr = z.string().trim().pipe(z.union([z.cidrv4(), z.cidrv6()]))

export const ipv4 = z.string().trim().pipe(z.ipv4())

/** A date ("YYYY-MM-DD") or ISO date-time; blank → undefined; output ISO 8601 UTC. */
export const isoDate = z
  .string()
  .nullish()
  .transform((v, ctx) => {
    const s = (v ?? '').toString().trim()
    if (!s) return undefined
    const t = /^\d{4}-\d{2}-\d{2}$/.test(s) ? Date.parse(s + 'T00:00:00Z') : Date.parse(s)
    if (Number.isNaN(t)) {
      ctx.addIssue({ code: 'invalid_format', format: 'date', input: s, message: 'invalid date' })
      return z.NEVER
    }
    return new Date(t).toISOString()
  })
  .meta({ kind: 'date', optional: true })

/** Non-negative finite amount; blank → 0. */
export const money = z.preprocess((v) => (v === '' || v === null || v === undefined ? 0 : v), z.coerce.number().finite().min(0)).meta({ kind: 'number', optional: true })

/** Non-negative integer; blank → 0. */
export const positiveInt = z.preprocess((v) => (v === '' || v === null || v === undefined ? 0 : v), z.coerce.number().int().min(0)).meta({ kind: 'number', optional: true })

/** Required trimmed string with a maximum length. */
export const nonEmpty = (max = 200) => z.string().trim().min(1).max(max)

/** Optional trimmed string; blank → undefined. */
export const optionalString = (max = 1000) =>
  z.string().nullish().transform((v) => {
    const s = (v ?? '').toString().trim()
    return s === '' ? undefined : s
  }).pipe(z.string().max(max).optional()).meta({ kind: 'text', optional: true })

/** A key→value tag map with a maximum number of entries and no blank keys. */
export const tagMap = (maxKeys = 64) =>
  z.preprocess((v) => (v === undefined || v === null ? {} : v), z.record(z.string().min(1), z.string()).refine((m) => Object.keys(m).length <= maxKeys, { message: `At most ${maxKeys} tags.` })).meta({ kind: 'tags', optional: true })

/** Lower-case slug. */
export const slug = z.string().trim().regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/)

/** Adds a "to is not before from" refinement on two isoDate fields. */
export function dateRange<T extends z.ZodObject<z.ZodRawShape>>(schema: T, from: string, to: string, message = 'Must not be before the start date.') {
  return schema.refine(
    (o) => {
      const a = (o as Record<string, unknown>)[from]
      const b = (o as Record<string, unknown>)[to]
      return !(typeof a === 'string' && typeof b === 'string') || b >= a
    },
    { message, path: [to] },
  )
}

/** A JSON object typed into a textarea: blank → {}; arrays/scalars/invalid JSON are refused. */
export const jsonObject = z
  .string()
  .nullish()
  .transform((v, ctx) => {
    const s = (v ?? '').toString().trim()
    if (!s) return {} as Record<string, unknown>
    let parsed: unknown
    try {
      parsed = JSON.parse(s)
    } catch {
      ctx.addIssue({ code: 'custom', message: 'Enter valid JSON.' })
      return z.NEVER
    }
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      ctx.addIssue({ code: 'custom', message: 'Enter a JSON object ({…}).' })
      return z.NEVER
    }
    return parsed as Record<string, unknown>
  })
  .meta({ kind: 'textarea', optional: true })
