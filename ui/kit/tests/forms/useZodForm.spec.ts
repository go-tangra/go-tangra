import { describe, it, expect, vi } from 'vitest'
import { nextTick } from 'vue'
import { z } from 'zod'
import { useZodForm } from '@/forms/useZodForm'
import { ApiError } from '@/api/client'
import { messages } from '@/forms/messages'
import { email, nonEmpty, money } from '@/forms/schemas'

const S = z.object({ name: nonEmpty(10), email, cost: money, note: z.string().optional() })

function mountInputs(paths: string[]) {
  document.body.innerHTML = paths.map((p) => `<input data-field="${p}" />`).join('')
}

describe('useZodForm', () => {
  it('validates on blur and clears once valid', async () => {
    const f = useZodForm(S, { onSubmit: vi.fn() })
    f.values.email = 'nope'
    f.blur('email')
    await nextTick()
    expect(f.errors.value.email).toBeTruthy()
    f.values.email = 'a@b.co'
    await nextTick()
    expect(f.errors.value.email).toBeUndefined()
  })
  it('submit blocks while invalid, focuses first invalid field, does not call onSubmit', async () => {
    mountInputs(['name', 'email', 'cost'])
    const onSubmit = vi.fn()
    const f = useZodForm(S, { onSubmit })
    const ok = await f.submit()
    expect(ok).toBe(false)
    expect(onSubmit).not.toHaveBeenCalled()
    expect(f.errors.value.name).toBe(messages.required)
    expect(document.activeElement?.getAttribute('data-field')).toBe('name')
  })
  it('submits the schema output (normalised payload) and reports success', async () => {
    const onSubmit = vi.fn(async (p: z.output<typeof S>) => ({ id: '1', ...p }))
    const onSuccess = vi.fn()
    const f = useZodForm(S, { initial: { name: '  Laptop ', email: ' A@B.CO', cost: '12.5' as unknown as number }, onSubmit, onSuccess })
    expect(await f.submit()).toBe(true)
    expect(onSubmit).toHaveBeenCalledWith({ name: 'Laptop', email: 'a@b.co', cost: 12.5 })
    expect(onSuccess).toHaveBeenCalledWith({ id: '1', name: 'Laptop', email: 'a@b.co', cost: 12.5 })
    expect(f.submitting.value).toBe(false)
  })
  it('maps server validation detail onto fields and conflicts onto the banner', async () => {
    const f = useZodForm(S, {
      initial: { name: 'x', email: 'a@b.co', cost: 1 },
      onSubmit: async () => {
        throw new ApiError(422, 'validation_failed', { fields: { email: 'taken' } })
      },
    })
    expect(await f.submit()).toBe(false)
    expect(f.errors.value.email).toBe('taken')
    const g = useZodForm(S, { initial: { name: 'x', email: 'a@b.co', cost: 1 }, onSubmit: async () => { throw new ApiError(409, 'conflict') } })
    expect(await g.submit()).toBe(false)
    expect(g.serverError.value).toBe(messages.reason.conflict)
    const h = useZodForm(S, { initial: { name: 'x', email: 'a@b.co', cost: 1 }, onSubmit: async () => { throw new Error('boom') } })
    expect(await h.submit()).toBe(false)
    expect(h.serverError.value).toBe(messages.reason.generic)
  })
  it('reset restores initial values and clears errors; field() binds v-model handlers', async () => {
    const f = useZodForm(S, { initial: { name: 'a' }, onSubmit: vi.fn() })
    const bind = f.field('name')
    bind['onUpdate:modelValue']('')
    bind.onBlur()
    await nextTick()
    expect(f.errors.value.name).toBeTruthy()
    f.reset()
    expect(f.values.name).toBe('a')
    expect(Object.keys(f.errors.value)).toHaveLength(0)
    f.reset({ name: 'zzz' })
    expect(f.values.name).toBe('zzz')
    expect(f.field('name').id).toBe('name')
    expect(f.field('name').modelValue).toBe('zzz')
    expect(f.dirty.value).toBe(false)
    f.values.name = 'changed'
    expect(f.dirty.value).toBe(true)
  })
  it('setServerError shows a banner and setFieldError targets a field', () => {
    const f = useZodForm(S, { onSubmit: vi.fn() })
    f.setServerError(new ApiError(403, 'forbidden'))
    expect(f.serverError.value).toBe(messages.reason.forbidden)
    f.setFieldError('name', 'custom')
    expect(f.errors.value.name).toBe('custom')
    f.setServerError(undefined)
    expect(f.serverError.value).toBe('')
  })
})

describe('useZodForm (branches)', () => {
  it('blur on a valid field clears its error; focus lookup falls back to a wrapper element; validate() returns data', async () => {
    document.body.innerHTML = '<div data-field="name"><input id="inner" /></div><div data-field="email"></div>'
    const f = useZodForm(S, { initial: { name: 'ok', email: 'bad' }, onSubmit: vi.fn() })
    f.blur('name')
    expect(f.errors.value.name).toBeUndefined()
    f.setFieldError('name', 'server says no')
    f.blur('name')
    expect(f.errors.value.name).toBeUndefined()
    f.values.name = ''
    expect(await f.submit()).toBe(false)
    expect(document.activeElement?.id).toBe('inner')
    f.values.name = 'ok'
    f.values.email = 'a@b.co'
    f.values.cost = 1
    expect(f.validate()).toEqual({ name: 'ok', email: 'a@b.co', cost: 1 })
    const g = useZodForm(S, { onSubmit: vi.fn(), root: () => null })
    expect(await g.submit()).toBe(false)
    const h = useZodForm(S, { initial: { name: 'x', email: 'a@b.co', cost: 1 }, onSubmit: async () => { throw new ApiError(422, 'validation_failed', { fields: { email: '' } }) } })
    expect(await h.submit()).toBe(false)
    expect(h.errors.value.email).toBe(messages.reason.validation_failed)
    const k = useZodForm(S, { initial: { name: 'x', email: 'a@b.co', cost: 1 }, onSubmit: async () => { throw new ApiError(422, 'validation_failed') } })
    expect(await k.submit()).toBe(false)
    expect(k.serverError.value).toBe(messages.reason.validation_failed)
  })
})

describe('useZodForm (edge branches)', () => {
  it('empty query strings, root-level refinements, server field errors survive edits of other fields', async () => {
    const R = z.object({ a: z.string(), b: z.string() }).refine((o) => o.a !== o.b, { message: 'must differ' })
    const f = useZodForm(R, { initial: { a: 'x', b: 'x' }, onSubmit: vi.fn(), root: () => null })
    expect(f.valid.value).toBe(false)
    expect(await f.submit()).toBe(false)
    expect(f.errors.value._).toBe('must differ')
    f.values.b = 'y'
    expect(f.valid.value).toBe(true)
    const g = useZodForm(S, { initial: { name: 'ok', email: 'a@b.co', cost: 1 }, onSubmit: vi.fn() })
    g.setFieldError('email', 'taken')
    g.values.name = 'other'
    await nextTick()
    expect(g.errors.value.email).toBe('taken')
    g.blur('name')
    g.values.name = 'again'
    await nextTick()
    expect(g.errors.value.email).toBe('taken')
  })
})

describe('useZodForm (element issues)', () => {
  it('surfaces an array element issue on the collection field too', () => {
    const S = z.object({ uris: z.string().transform((s) => s.split(',')).pipe(z.array(z.string().url('Not a URL.'))) })
    const form = useZodForm(S, { initial: { uris: 'https://a.test,nope' }, onSubmit: () => {} })
    expect(form.validate()).toBeUndefined()
    expect(form.errors.value['uris.1']).toBe('Not a URL.')
    expect(form.errors.value.uris).toBe('Not a URL.')
  })
})

describe('zod runtime configuration', () => {
  it('runs jitless so no `new Function` probe reaches the CSP', async () => {
    await import('@/forms')
    expect(z.config().jitless).toBe(true)
  })
})
