import { describe, it, expect } from 'vitest'
import { expectA11y, mountBody, setViewport } from '../helpers'
import { UiDataTable, type Column } from '@/index'

type Row = Record<string, unknown> & { id: string; name: string; n: number }
const rows: Row[] = [{ id: '1', name: 'b', n: 2 }, { id: '2', name: 'a', n: 1 }, { id: '3', name: 'c', n: 3 }]
const columns: Column<Record<string, unknown>>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'n', label: 'N', sortable: true, defaultDir: 'desc', align: 'end' },
  { key: 'x', label: 'X', format: (r) => 'x' + String(r.id) },
]
const server = { items: rows, columns, total: 1234, page: 2, pageSize: 25, sort: { key: 'name', dir: 'asc' as const } }

describe('UiDataTable server mode', () => {
  it('never sorts locally; headers emit update:sort with the column default then toggle', async () => {
    setViewport(1280)
    const w = mountBody(UiDataTable, { props: server })
    const order = () => w.findAll('tbody tr').map((r) => r.text().slice(0, 1))
    expect(order()).toEqual(['b', 'a', 'c']) // as given by the server
    expect(w.find('th[aria-sort]').attributes('aria-sort')).toBe('ascending')
    const [name, n] = w.findAll('th button')
    await name!.trigger('click') // active column → reverse
    await n!.trigger('click') // new column → its defaultDir
    expect(w.emitted('update:sort')).toEqual([[{ key: 'name', dir: 'desc' }], [{ key: 'n', dir: 'desc' }]])
    expect(order()).toEqual(['b', 'a', 'c']) // still untouched
    expect(w.findAll('th button')).toHaveLength(2) // non-sortable column has no control
    await w.setProps({ sort: { key: 'n', dir: 'desc' } })
    expect(w.findAll('th')[1]!.attributes('aria-sort')).toBe('descending')
    await w.setProps({ sort: { key: 'x', dir: 'asc' } })
    await w.findAll('th button')[0]!.trigger('click') // inactive column without defaultDir → asc
    expect(w.emitted('update:sort')!.at(-1)).toEqual([{ key: 'name', dir: 'asc' }])
    await w.setProps({ sort: null })
    expect(w.find('th[aria-sort]').exists()).toBe(false)
    await expectA11y(w.element)
  })

  it('renders the pager, forwards its events and has no window or load-more', async () => {
    setViewport(1280)
    const many = Array.from({ length: 250 }, (_, i) => ({ id: String(i), name: 'n' + i, n: i }))
    const w = mountBody(UiDataTable, { props: { ...server, items: many, hasMore: true, virtualAt: 10 } })
    expect(w.findAll('tbody tr')).toHaveLength(250)
    expect(w.text()).not.toContain('Show more')
    expect(w.text()).not.toContain('Load more')
    expect(w.text()).toContain('Showing 26–50 of 1,234')
    await w.find('[aria-label="Next page"]').trigger('click')
    await w.find('select').setValue('100')
    expect(w.emitted('update:page')).toEqual([[3]])
    expect(w.emitted('update:pageSize')).toEqual([[100]])
  })

  it('keeps the previous rows visible while loading', async () => {
    setViewport(1280)
    const w = mountBody(UiDataTable, { props: { ...server, loading: true } })
    expect(w.findAll('tbody tr')).toHaveLength(3)
    expect(w.find('[aria-busy="true"]').exists()).toBe(true)
  })

  it('shows the empty state and no pager for zero results', () => {
    setViewport(1280)
    const w = mountBody(UiDataTable, { props: { ...server, items: [], total: 0, page: 1, emptyTitle: 'No hosts' } })
    expect(w.text()).toContain('No hosts')
    expect(w.text()).not.toContain('Showing')
  })

  it('stacked layout offers a sort select and a direction toggle', async () => {
    setViewport(375)
    const w = mountBody(UiDataTable, { props: server })
    expect(w.find('table').exists()).toBe(false)
    const sel = w.find('select[aria-label="Sort by"]')
    expect(sel.findAll('option').map((o) => o.text())).toEqual(['Name', 'N'])
    await sel.setValue('n')
    await w.find('button[aria-label="Sort descending"]').trigger('click')
    expect(w.emitted('update:sort')).toEqual([[{ key: 'n', dir: 'desc' }], [{ key: 'name', dir: 'desc' }]])
    await w.setProps({ sort: { key: 'name', dir: 'desc' } })
    expect(w.find('button[aria-label="Sort ascending"]').exists()).toBe(true)
    expect(w.text()).toContain('Showing 26–50 of 1,234')
    await expectA11y(w.element)
  })

  it('client mode (no total) is unchanged: local sort, no pager', async () => {
    setViewport(1280)
    const w = mountBody(UiDataTable, { props: { items: rows, columns } })
    await w.find('th button').trigger('click')
    expect(w.findAll('tbody tr').map((r) => r.text().slice(0, 1))).toEqual(['a', 'b', 'c'])
    expect(w.emitted('update:sort')).toBeUndefined()
    expect(w.text()).not.toContain('Showing')
  })
})
