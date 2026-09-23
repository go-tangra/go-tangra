// useZodForm: the platform's single form mechanism. One Zod schema per form is
// the source of truth for validation (blur + submit) and for the payload the
// API receives (the schema output). It focuses the first invalid field on
// submit, blocks the request while invalid and maps server refusals onto the
// same inline/banner presentation.
import { computed, reactive, ref, watch, type Ref } from 'vue'
import type { z } from 'zod'
import { ApiError } from '@/api/client'
import { describeIssue, describeReason } from './messages'

export interface UseZodFormOptions<S extends z.ZodType, R = unknown> {
  initial?: Partial<z.input<S>> | undefined
  onSubmit: (payload: z.output<S>) => Promise<R> | R
  onSuccess?: (result: R) => void
  /** Root element to search for `[data-field]` inputs when focusing (default: document). */
  root?: () => ParentNode | null
}

export interface FieldBinding {
  id: string
  modelValue: unknown
  'onUpdate:modelValue': (v: unknown) => void
  onBlur: () => void
  error: string | undefined
  'data-field': string
}

export interface ZodForm<S extends z.ZodType> {
  values: Record<string, unknown>
  errors: Ref<Record<string, string>>
  touched: Ref<Record<string, boolean>>
  serverError: Ref<string>
  submitting: Ref<boolean>
  dirty: Ref<boolean>
  valid: Ref<boolean>
  blur: (path: string) => void
  validate: () => z.output<S> | undefined
  submit: () => Promise<boolean>
  reset: (initial?: Partial<z.input<S>>) => void
  field: (path: string) => FieldBinding
  setServerError: (err: unknown) => void
  setFieldError: (path: string, text: string) => void
}

function pathKey(p: PropertyKey[]): string {
  return p.map(String).join('.')
}

export function useZodForm<S extends z.ZodType, R = unknown>(schema: S, opts: UseZodFormOptions<S, R>): ZodForm<S> {
  const snapshot = () => ({ ...((opts.initial ?? {}) as Record<string, unknown>) })
  const values = reactive<Record<string, unknown>>(snapshot())
  const errors = ref<Record<string, string>>({})
  const touched = ref<Record<string, boolean>>({})
  const serverError = ref('')
  const submitting = ref(false)
  let baseline = JSON.stringify(snapshot())
  const dirty = computed(() => JSON.stringify(values) !== baseline)

  function run(): { ok: true; data: z.output<S> } | { ok: false; errs: Record<string, string> } {
    const res = schema.safeParse(values)
    if (res.success) return { ok: true, data: res.data }
    const errs: Record<string, string> = {}
    for (const issue of res.error.issues) {
      const k = pathKey(issue.path) || '_'
      if (!errs[k]) errs[k] = describeIssue(issue)
      // An element issue (tags.2, redirect_uris.0) also surfaces on the field
      // bound to the collection, which is the control the person sees.
      const root = issue.path.length > 1 ? String(issue.path[0]) : ''
      if (root && !errs[root]) errs[root] = describeIssue(issue)
    }
    return { ok: false, errs }
  }

  const valid = computed(() => run().ok)

  /** Re-validate touched fields whenever values change (clears errors as they become valid). */
  watch(values, () => {
    const r = run()
    const next: Record<string, string> = {}
    for (const k of Object.keys(touched.value)) {
      if (!r.ok && r.errs[k]) next[k] = r.errs[k]
    }
    // keep server-set field errors for untouched fields
    for (const [k, v] of Object.entries(errors.value)) if (!touched.value[k] && !(k in next)) next[k] = v
    errors.value = next
  }, { deep: true })

  function blur(path: string) {
    touched.value = { ...touched.value, [path]: true }
    const r = run()
    const next = { ...errors.value }
    if (!r.ok && r.errs[path]) next[path] = r.errs[path]
    else delete next[path]
    errors.value = next
  }

  function focusFirst(errs: Record<string, string>) {
    // An explicit root that is not mounted yet means "nothing to focus" (not a fallback to document).
    const root = opts.root ? opts.root() : typeof document !== 'undefined' ? document : null
    if (!root) return
    const FOCUSABLE = 'input,select,textarea,button,[tabindex]'
    for (const k of Object.keys(errs)) {
      const sel = `[data-field="${k.replace(/["\\]/g, '')}"]`
      const els = Array.from(root.querySelectorAll<HTMLElement>(sel))
      const el = els.find((e) => e.matches(FOCUSABLE)) ?? els[0]?.querySelector<HTMLElement>(FOCUSABLE) ?? els[0]
      if (el) {
        el.focus()
        return
      }
    }
  }

  function validate(): z.output<S> | undefined {
    const r = run()
    if (r.ok) {
      errors.value = {}
      return r.data
    }
    const all: Record<string, boolean> = {}
    for (const k of Object.keys(r.errs)) all[k] = true
    touched.value = { ...touched.value, ...all }
    errors.value = r.errs
    focusFirst(r.errs)
    return undefined
  }

  function setServerError(err: unknown) {
    if (err === undefined || err === null) {
      serverError.value = ''
      return
    }
    if (err instanceof ApiError) {
      const fields = (err.detail as { fields?: Record<string, unknown> } | undefined)?.fields
      if (err.reason === 'validation_failed' && fields && typeof fields === 'object') {
        const next = { ...errors.value }
        for (const [k, v] of Object.entries(fields)) next[k] = typeof v === 'string' && v ? v : describeReason('validation_failed')
        errors.value = next
        focusFirst(next)
        serverError.value = ''
        return
      }
      serverError.value = describeReason(err.reason)
      return
    }
    serverError.value = describeReason('generic')
  }

  async function submit(): Promise<boolean> {
    serverError.value = ''
    const data = validate()
    if (data === undefined) return false
    submitting.value = true
    try {
      const result = await opts.onSubmit(data)
      opts.onSuccess?.(result as R)
      baseline = JSON.stringify(values)
      return true
    } catch (err) {
      setServerError(err)
      return false
    } finally {
      submitting.value = false
    }
  }

  function reset(initial?: Partial<z.input<S>>) {
    if (initial !== undefined) opts.initial = initial
    for (const k of Object.keys(values)) delete values[k]
    Object.assign(values, snapshot())
    baseline = JSON.stringify(values)
    errors.value = {}
    touched.value = {}
    serverError.value = ''
  }

  function field(path: string): FieldBinding {
    return {
      id: path,
      'data-field': path,
      modelValue: values[path],
      'onUpdate:modelValue': (v: unknown) => {
        values[path] = v
      },
      onBlur: () => blur(path),
      error: errors.value[path],
    }
  }

  function setFieldError(path: string, text: string) {
    errors.value = { ...errors.value, [path]: text }
  }

  return { values, errors, touched, serverError, submitting, dirty, valid, blur, validate, submit, reset, field, setServerError, setFieldError }
}
