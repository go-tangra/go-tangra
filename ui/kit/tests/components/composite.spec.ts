import { describe, it, expect, vi } from 'vitest'
import { nextTick, defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { z } from 'zod'
import { createRouter, createMemoryHistory } from 'vue-router'
import { expectA11y, mountBody, setViewport, tick } from '../helpers'
import { UiRecordDialog, UiRecordDrawer, UiRecordForm, UiDocumentList, UiAuditTable, UiPermissionDrawer, UiAppShell, UiNavDrawer, UiUserMenu, UiRemoteBoundary, useTheme } from '@/index'
import { nonEmpty, money, isoDate, tagMap } from '@/forms'
import { ApiError, type Api } from '@/api'

const Schema = z.object({ name: nonEmpty(20), cost: money, status: z.enum(['a', 'b']).optional(), notes: z.string().optional(), when: isoDate, tags: tagMap(4), ok: z.boolean().optional(), secret_token: z.string().optional() })

function fakeApi(handlers: Record<string, (method: string, path: string, body?: unknown) => unknown>): Api {
  const api = (async (method: string, path: string, body?: unknown) => {
    const h = handlers[method + ' ' + path] ?? handlers[method + ' *']
    if (!h) throw new ApiError(404, 'not_found')
    return h(method, path, body)
  }) as unknown as Api
  api.upload = (async (path: string, file: File, fields?: Record<string, string>) => (handlers['UPLOAD ' + path] ?? (() => ({ id: 'up', file, fields })))('POST', path, fields)) as Api['upload']
  api.fileUrl = (p: string) => '/base/' + p
  return api
}

describe('UiRecordDialog / UiRecordDrawer / UiRecordForm', () => {
  it('derives fields from the schema, validates, submits, closes and emits saved; server refusal inline', async () => {
    const submit = vi.fn(async (v: Record<string, unknown>) => ({ id: '1', ...v }))
    const w = mountBody(UiRecordDialog, { props: { modelValue: true, title: 'New', schema: Schema, submit } })
    await tick()
    expect(document.body.querySelector('[role=dialog]')).toBeTruthy()
    expect(document.body.querySelectorAll('[data-field]').length).toBeGreaterThanOrEqual(7)
    expect(document.body.querySelector('select[data-field=status]')).toBeTruthy()
    expect(document.body.querySelector('textarea[data-field=notes]')).toBeTruthy()
    expect(document.body.querySelector('input[type=date][data-field=when]')).toBeTruthy()
    expect(document.body.querySelector('input[type=password][data-field=secret_token]')).toBeTruthy()
    expect(document.body.querySelector('input[type=checkbox][data-field=ok]')).toBeTruthy()
    const save = Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Save')!
    save.click()
    await tick()
    expect(submit).not.toHaveBeenCalled()
    expect(document.body.querySelector('[role=alert]')?.textContent).toContain('required')
    const name = document.body.querySelector<HTMLInputElement>('input[data-field=name]')!
    name.value = 'Laptop'
    name.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    save.click()
    await tick()
    await tick()
    expect(submit).toHaveBeenCalledWith(expect.objectContaining({ name: 'Laptop', cost: 0, tags: {} }))
    expect(w.emitted('saved')?.[0]?.[0]).toMatchObject({ id: '1' })
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([false])
    w.unmount()
    const failing = vi.fn(async () => { throw new ApiError(409, 'conflict') })
    const f = mountBody(UiRecordDialog, { props: { modelValue: true, title: 'Dup', schema: Schema, initial: { name: 'x' }, submit: failing, fields: [{ key: 'name', label: 'Name' }] } })
    await tick()
    Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Save')!.click()
    await tick()
    await tick()
    expect(document.body.textContent).toContain('not possible')
    Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Cancel')!.click()
    expect(f.emitted('update:modelValue')?.at(-1)).toEqual([false])
    await expectA11y(document.body.querySelector('[role=dialog]')!)
    f.unmount()
  })
  it('drawer variant renders in an aside, honours readonly, and the form resets on initial change', async () => {
    const submit = vi.fn(async (v: Record<string, unknown>) => v)
    const w = mountBody(UiRecordDrawer, { props: { modelValue: true, title: 'Edit', schema: Schema, initial: { name: 'a' }, submit, fields: [{ key: 'name', label: 'Name' }] } })
    await tick()
    expect(document.body.querySelector('aside[role=dialog]')).toBeTruthy()
    expect((document.body.querySelector('input[data-field=name]') as HTMLInputElement).value).toBe('a')
    await w.setProps({ initial: { name: 'b' } })
    await nextTick()
    expect((document.body.querySelector('input[data-field=name]') as HTMLInputElement).value).toBe('b')
    Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Save')!.click()
    await tick()
    await tick()
    expect(submit).toHaveBeenCalledWith(expect.objectContaining({ name: 'b' }))
    expect(w.emitted('saved')).toHaveLength(1)
    // A detail panel stays open after saving unless close-on-save is set.
    expect(w.emitted('update:modelValue')).toBeUndefined()
    w.unmount()
    const c = mountBody(UiRecordDrawer, { props: { modelValue: true, title: 'New', schema: Schema, initial: { name: 'n' }, submit, closeOnSave: true, fields: [{ key: 'name', label: 'Name' }] } })
    await tick()
    Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Save')!.click()
    await tick()
    await tick()
    expect(c.emitted('saved')).toHaveLength(1)
    expect(c.emitted('update:modelValue')).toEqual([[false]])
    c.unmount()
    const r = mountBody(UiRecordDrawer, { props: { modelValue: true, title: 'View', schema: Schema, submit, readonly: true } })
    await tick()
    expect(Array.from(document.body.querySelectorAll('button')).some((b) => b.textContent?.trim() === 'Save')).toBe(false)
    r.unmount()
    const form = mount(UiRecordForm, { props: { schema: Schema, submit, fields: [{ key: 'name', label: 'Name', cols: 12 }, { key: 'cost', label: 'Cost', type: 'number', cols: 4 }, { key: 'notes', label: 'N', type: 'textarea', cols: 3 }, { key: 'when', label: 'W', type: 'date', cols: 8 }] } })
    expect(form.find('.md\\:col-span-12').exists()).toBe(true)
    expect(form.find('.md\\:col-span-4').exists()).toBe(true)
    expect(form.find('.md\\:col-span-3').exists()).toBe(true)
    expect(form.find('.md\\:col-span-8').exists()).toBe(true)
  })
})

describe('UiDocumentList', () => {
  it('lists, uploads, downloads via fileUrl, deletes after confirm', async () => {
    const docs = [{ id: 'd1', file_name: 'inv.pdf', file_size: 2048, mime_type: 'application/pdf', description: 'inv', created_at: '2026-01-01T00:00:00Z' }, { id: 'd2', file_name: 'big.bin', file_size: 3 * 1048576, created_at: '2026-01-01T00:00:00Z' }, { id: 'd3', file_name: 's.txt', file_size: 12, created_at: '2026-01-01T00:00:00Z' }]
    const calls: string[] = []
    const api = fakeApi({
      'GET assets/1/documents': () => ({ items: docs }),
      'DELETE assets/1/documents/d1': () => { calls.push('del'); docs.shift(); return undefined },
      'UPLOAD assets/1/documents': () => { calls.push('up'); return { id: 'n' } },
    })
    setViewport(1280)
    const w = mountBody(UiDocumentList, { props: { api, base: 'assets/1/documents' } })
    await tick()
    await nextTick()
    expect(w.text()).toContain('inv.pdf')
    expect(w.text()).toContain('2.0 KB')
    expect(w.text()).toContain('3.0 MB')
    expect(w.text()).toContain('12 B')
    expect(w.find('a.link').attributes('href')).toBe('/base/assets/1/documents/d1/download')
    // upload
    const input = w.find('input[type=file]').element as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [new File(['x'], 'a.txt')], configurable: true })
    await w.find('input[type=file]').trigger('change')
    await w.find('button.btn-primary').trigger('click')
    await tick()
    expect(calls).toContain('up')
    // delete with confirm host
    const { UiConfirm } = await import('@/index')
    const host = mountBody(UiConfirm)
    await w.find('button[aria-label=Delete]').trigger('click')
    await nextTick()
    await tick()
    Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Delete' && b.className.includes('btn-error'))!.click()
    await tick()
    await tick()
    expect(calls).toContain('del')
    host.unmount()
    // errors surface
    const bad = fakeApi({ 'GET *': () => { throw new ApiError(500, 'internal') } })
    const e = mountBody(UiDocumentList, { props: { api: bad, base: 'x', canManage: false } })
    await tick()
    await nextTick()
    expect(e.text()).toContain('Something went wrong')
    expect(e.find('input[type=file]').exists()).toBe(false)
    e.unmount()
    w.unmount()
  })
})

describe('UiAuditTable / UiPermissionDrawer', () => {
  it('audit loads with filters, pages, expands detail', async () => {
    const seen: unknown[] = []
    const page = () => ({ items: Array.from({ length: 50 }, (_, i) => ({ id: 'e' + i, at: '2026-01-01T00:00:00Z', action: 'x', actor_id: 'u', actor_kind: 'user', outcome: i ? 'ok' : 'refused', detail: { k: i } })), next_cursor: 'c' })
    const api = fakeApi({ 'GET audit': (_m, _p, q) => { seen.push(q); return page() } })
    const w = mountBody(UiAuditTable, { props: { api } })
    await tick()
    await nextTick()
    expect(w.findAll('tbody tr')).toHaveLength(50)
    expect(w.text()).toContain('refused')
    await w.find('tbody tr').trigger('click')
    expect(w.find('pre').text()).toContain('"k": 0')
    await w.find('tbody tr').trigger('click')
    expect(w.find('pre').exists()).toBe(false)
    await w.find('button.btn').trigger('click') // Load more at the bottom? first .btn is Filter
    await tick()
    const more = w.findAll('button').find((b) => b.text() === 'Load more')!
    await more.trigger('click')
    await tick()
    await nextTick()
    expect(w.findAll('tbody tr')).toHaveLength(100)
    const bad = fakeApi({ 'GET *': () => { throw new ApiError(403, 'forbidden') } })
    const e = mountBody(UiAuditTable, { props: { api: bad } })
    await tick()
    await nextTick()
    expect(e.text()).toContain('not allowed')
    e.unmount()
    w.unmount()
  })
  it('permission drawer grants, revokes, changes level', async () => {
    const w = mountBody(UiPermissionDrawer, { props: { modelValue: true, title: 'Perms', canManage: true, grants: [{ id: 'g1', subject_kind: 'user', subject_id: 'u1', subject_name: 'Ann', level: 'read' }], subjects: [{ title: 'Bob', value: 'u2' }], levels: [{ title: 'Read', value: 'read' }, { title: 'Write', value: 'write' }] } })
    await tick()
    const cb = document.body.querySelector<HTMLInputElement>('input[role=combobox]')!
    cb.focus()
    cb.value = 'Bob'
    cb.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    ;(document.body.querySelector('[role=option] button') as HTMLElement).dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    await nextTick()
    Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Grant')!.click()
    expect(w.emitted('grant')?.[0]).toEqual([{ subject: 'u2', level: 'read' }])
    const sel = document.body.querySelector<HTMLSelectElement>('td select')!
    sel.value = 'write'
    sel.dispatchEvent(new Event('change'))
    expect(w.emitted('change-level')?.[0]?.[1]).toBe('write')
    ;(document.body.querySelector('button[aria-label=Revoke]') as HTMLElement).click()
    expect(w.emitted('revoke')).toHaveLength(1)
    await w.setProps({ error: 'nope', canManage: false })
    await nextTick()
    expect(document.body.textContent).toContain('nope')
    expect(document.body.querySelector('td select')).toBeNull()
    w.unmount()
  })
})

describe('UiAppShell / UiNavDrawer / UiUserMenu / UiRemoteBoundary', () => {
  it('drawer overlays below lg and is persistent above; user menu toggles theme; boundary catches errors', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div/>' } }, { path: '/asset', component: { template: '<div/>' } }] })
    setViewport(360)
    const w = mountBody({
      components: { UiAppShell, UiNavDrawer, UiUserMenu },
      template: `<UiAppShell title="Tangra"><template #nav="{ close }"><UiNavDrawer :groups="[{ title: 'Assets', items: [{ title: 'Assets', path: '/asset', icon: 'mdi-laptop' }] }]" @navigate="close" /></template><template #app-bar><UiUserMenu name="Ann" tenant="Platform" @select="sel = $event" /></template><p>content</p></UiAppShell><span id="sel">{{ sel }}</span>`,
      data: () => ({ sel: '' }),
    }, { global: { plugins: [router] } })
    await router.isReady()
    const nav = w.find('#ui-nav')
    expect(nav.classes()).toContain('-translate-x-full')
    await w.find('button[aria-label="Open navigation"]').trigger('click')
    expect(w.find('#ui-nav').classes()).toContain('translate-x-0')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(w.find('#ui-nav').classes()).toContain('-translate-x-full')
    await w.find('button[aria-label="Open navigation"]').trigger('click')
    // The "Assets" group does not own the current route, so expand it first.
    await w.find('#ui-nav .accordion-toggle').trigger('click')
    await w.find('#ui-nav a').trigger('click')
    await nextTick()
    expect(w.find('#ui-nav').classes()).toContain('-translate-x-full')
    expect(w.find('a[href="#main"]').exists()).toBe(true)
    // user menu
    localStorage.clear()
    document.documentElement.setAttribute('data-theme', 'freya-light')
    useTheme().set('freya-light')
    await w.find('button[aria-label="Account menu"]').trigger('click')
    await nextTick()
    const items = w.findAll('[role=menuitem]')
    expect(items.map((i) => i.text())).toEqual(['Dark theme', 'Sign out'])
    await items[0]!.trigger('click')
    await nextTick()
    expect(document.documentElement.getAttribute('data-theme')).toBe('freya-dark')
    expect(localStorage.getItem('freya.theme')).toBe('freya-dark')
    await w.find('button[aria-label="Account menu"]').trigger('click')
    await nextTick()
    await w.findAll('[role=menuitem]')[1]!.trigger('click')
    expect(w.find('#sel').text()).toBe('signout')
    useTheme().toggle()
    expect(useTheme().isDark()).toBe(false)
    await expectA11y(w.element)
    w.unmount()
    setViewport(1280)
    const p = mountBody(UiAppShell, { props: { title: 'T' }, slots: { nav: '<span>n</span>' } })
    expect(p.find('#ui-nav').classes()).toContain('translate-x-0')
    expect(p.find('header').classes()).toContain('sticky')
    expect(p.find('button[aria-label="Open navigation"]').exists()).toBe(false)
    p.unmount()
    // remote boundary
    const Thrower = defineComponent({ setup() { throw new Error('shared module @go-tangra/ui version mismatch') } })
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const b = mountBody(UiRemoteBoundary, { props: { module: 'asset' }, slots: { default: () => h(Thrower) } })
    await nextTick()
    expect(b.text()).toContain('different platform version')
    await b.find('button').trigger('click')
    expect(b.emitted('retry')).toHaveLength(1)
    warn.mockRestore()
    b.unmount()
    const e = mount(UiRemoteBoundary, { props: { module: 'ipam', error: new Error('net') } })
    expect(e.text()).toContain('could not be loaded')
    const l = mount(UiRemoteBoundary, { props: { module: 'ipam', loading: true } })
    expect(l.find('[aria-busy=true]').exists()).toBe(true)
  })
})

describe('UiNavDrawer (collapsible groups)', () => {
  it('opens the group owning the current route, toggles manually, renders untitled groups flat', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div/>' } }] })
    await router.push('/warden/folders')
    await router.isReady()
    const groups = [
      { items: [{ title: 'Home', path: '/', icon: 'mdi-home-outline', exact: true, testId: 'nav-home' }] },
      { key: 'warden', title: 'Warden', icon: 'mdi-key-variant', testId: 'nav-group-warden', items: [{ title: 'Secrets', path: '/warden' }, { title: 'Folders', path: '/warden/folders', testId: 'nav-warden' }] },
      { key: 'auth', title: 'Authentication', testId: 'nav-group-auth', items: [{ title: 'Users', path: '/console/admin/users', testId: 'nav-auth' }] },
    ]
    const w2 = mountBody(UiNavDrawer, { props: { groups }, global: { plugins: [router] } })
    await nextTick()
    expect(w2.find('[data-test=nav-home]').exists()).toBe(true)
    // A group is open when its toggle reports aria-expanded (FlyonUI accordion rows).
    const warden = () => w2.find('[data-test=nav-group-warden]')
    const auth = () => w2.find('[data-test=nav-group-auth]')
    const open = (b: ReturnType<typeof warden>) => b.attributes('aria-expanded') === 'true'
    expect(open(warden())).toBe(true)
    expect(open(auth())).toBe(false)
    expect(w2.find('[data-test=nav-warden]').classes()).toContain('menu-active')
    await auth().trigger('click')
    expect(open(auth())).toBe(true)
    await warden().trigger('click')
    expect(open(warden())).toBe(false)
    await router.push('/console/admin/users')
    await nextTick()
    expect(open(auth())).toBe(true)
    expect(open(warden())).toBe(false)
    await expectA11y(w2.element)
    w2.unmount()
  })
})
