import { describe, it, expect, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountBody, setViewport } from '../helpers'
import { UiAuditTable } from '@/index'

describe('UiAuditTable page mode', () => {
  it('requests page/page_size, renders the pager and resets to page 1 on filter', async () => {
    setViewport(1280)
    const api = vi.fn(async (_m: string, _p: string, _b: unknown, opts?: { query?: Record<string, unknown> }) => ({
      items: [{ id: 'e1', at: '2026-10-01T10:00:00Z', action: 'login', outcome: 'ok' }],
      total: 120,
      page: Number(opts?.query?.page ?? 1),
      page_size: 50,
    }))
    const w = mountBody(UiAuditTable, { props: { api, paging: 'page' } })
    await flushPromises()
    expect(api.mock.calls[0]![3]!.query).toMatchObject({ page: 1, page_size: 50 })
    expect(api.mock.calls[0]![3]!.query!.cursor).toBeUndefined()
    expect(w.text()).toContain('Showing 1–50 of 120')
    await w.find('[aria-label="Page 3"]').trigger('click')
    await flushPromises()
    expect(api.mock.calls.at(-1)![3]!.query).toMatchObject({ page: 3 })
    await w.find('#audit-type').setValue('login')
    await w.findAll('button').find((b) => b.text() === 'Filter')!.trigger('click')
    await flushPromises()
    expect(api.mock.calls.at(-1)![3]!.query).toMatchObject({ page: 1, event_type: 'login' })
  })

  it('cursor mode stays the default', async () => {
    setViewport(1280)
    const api = vi.fn(async () => ({ items: [], next_cursor: '' }))
    mountBody(UiAuditTable, { props: { api } })
    await flushPromises()
    expect((api.mock.calls[0] as unknown[])[3]).toMatchObject({ query: { limit: 50 } })
  })
})
