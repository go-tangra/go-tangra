// Derives a field list for UiRecordDialog/UiRecordDrawer from a Zod object
// schema so simple records need no hand-written field definitions.
import { z } from 'zod'

export type FieldType = 'text' | 'number' | 'date' | 'select' | 'textarea' | 'checkbox' | 'secret' | 'tags'

export interface FieldOption {
  title: string
  value: string
}

export interface FieldDef {
  key: string
  label: string
  type?: FieldType
  options?: FieldOption[]
  required?: boolean
  hint?: string
  cols?: number
  placeholder?: string
}

function label(key: string): string {
  const s = key.replace(/_id$/, '').replace(/_/g, ' ')
  return s.charAt(0).toUpperCase() + s.slice(1)
}

interface Meta { kind?: FieldType | 'email'; optional?: boolean }

function unwrap(t: z.ZodType): { inner: z.ZodType; optional: boolean; meta: Meta } {
  let cur: z.ZodType = t
  let optional = false
  let meta: Meta = {}
  for (let i = 0; i < 8; i++) {
    const m = (cur as z.ZodType & { meta?: () => Meta | undefined }).meta?.()
    if (m && !meta.kind) meta = { ...(m as Meta) }
    const def = (cur as z.ZodType & { def: { type: string; innerType?: z.ZodType; in?: z.ZodType; out?: z.ZodType } }).def
    if (def.type === 'optional' || def.type === 'nullable' || def.type === 'default') {
      optional = true
      cur = def.innerType as z.ZodType
    } else if (def.type === 'pipe') {
      const out = def.out as z.ZodType
      const outType = (out as z.ZodType & { def: { type: string } }).def.type
      cur = (outType === 'transform' ? def.in : out) as z.ZodType
    } else if (def.type === 'union') {
      optional = true
      break
    } else break
  }
  return { inner: cur, optional: optional || !!meta.optional, meta }
}

export function zodToFields(schema: z.ZodObject<z.ZodRawShape>, overrides: Record<string, Partial<FieldDef>> = {}): FieldDef[] {
  const out: FieldDef[] = []
  for (const [key, raw] of Object.entries(schema.shape)) {
    const { inner, optional, meta } = unwrap(raw as z.ZodType)
    const def = (inner as z.ZodType & { def: { type: string; entries?: Record<string, string>; checks?: unknown[] } }).def
    let type: FieldType = 'text'
    let options: FieldOption[] | undefined
    switch (meta.kind && meta.kind !== 'email' ? 'meta' : def.type) {
      case 'meta':
        type = meta.kind as FieldType
        break
      case 'number':
        type = 'number'
        break
      case 'boolean':
        type = 'checkbox'
        break
      case 'enum':
        type = 'select'
        options = Object.values(def.entries ?? {}).map((v) => ({ title: String(v), value: String(v) }))
        break
      case 'record':
        type = 'tags'
        break
      case 'transform':
      case 'string':
      default:
        type = /date|_at$/.test(key) ? 'date' : /notes|description/.test(key) ? 'textarea' : /password|secret|token/.test(key) ? 'secret' : 'text'
    }
    const f: FieldDef = { key, label: label(key), type, required: !optional }
    if (options) f.options = options
    if (type === 'textarea') f.cols = 12
    out.push({ ...f, ...(overrides[key] ?? {}) })
  }
  return out
}
