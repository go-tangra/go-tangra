import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { expectA11y, mountBody, setViewport } from '../helpers'
import { UiDataTable, UiKeyValueTable, UiStatTile, UiStatGrid, UiPagination, UiTree, UiBarList, type Column, type TreeNode } from '@/index'

type Row = Record<string, unknown> & { id: string; name: string; n: number }
const rows: Row[] = [{ id: '1', name: 'b', n: 2 }, { id: '2', name: 'a', n: 1 }, { id: '3', name: 'c', n: 3 }]
const columns: Column<Record<string, unknown>>[] = [{ key: 'name', label: 'Name', sortable: true }, { key: 'n', label: 'N', align: 'end', sortable: true, hideOnStack: true }, { key: 'x', label: 'X', format: (r) => 'x' + String(r.id) }]

describe('UiDataTable', () => {
  it('renders a table above md with contained horizontal scroll, sorts, row click, actions, load more', async () => {
    setViewport(1280)
    const w = mountBody(UiDataTable, { props: { items: rows, columns, clickable: true, hasMore: true, caption: 'Rows', selectable: true, selected: [] }, slots: { actions: '<button>act</button>' } })
    expect(w.find('table').exists()).toBe(true)
    expect(w.find('.overflow-x-auto').exists()).toBe(true)
    expect(w.find('caption').text()).toBe('Rows')
    expect(w.findAll('tbody tr')).toHaveLength(3)
    await w.find('th button').trigger('click')
    expect(w.findAll('tbody tr').map((r) => r.text().slice(0, 1))).toEqual(['a', 'b', 'c'])
    expect(w.find('th[aria-sort]').attributes('aria-sort')).toBe('ascending')
    await w.find('th button').trigger('click')
    expect(w.find('th[aria-sort]').attributes('aria-sort')).toBe('descending')
    await w.findAll('th button')[1]!.trigger('click')
    expect(w.findAll('tbody tr').map((r) => r.text()).join(' ')).toContain('x2')
    await w.find('tbody tr').trigger('click')
    expect(w.emitted('row-click')).toHaveLength(1)
    await w.find('tbody tr td:last-child button').trigger('click')
    expect(w.emitted('row-click')).toHaveLength(1) // actions cell does not bubble
    await w.find('button.btn').trigger('click')
    expect(w.emitted('load-more')).toHaveLength(1)
    await w.find('tbody input[type=checkbox]').trigger('click')
    expect(w.emitted('update:selected')?.[0]?.[0]).toHaveLength(1)
    await w.find('thead input[type=checkbox]').trigger('change')
    expect(w.emitted('update:selected')?.[1]?.[0]).toHaveLength(3)
    await w.setProps({ selected: ['1', '2', '3'] })
    await w.find('thead input[type=checkbox]').trigger('change')
    expect(w.emitted('update:selected')?.[2]?.[0]).toHaveLength(0)
    await expectA11y(w.element)
    w.unmount()
  })
  it('stacks below md hiding hideOnStack columns; empty/loading states; virtualises', async () => {
    setViewport(360)
    const w = mountBody(UiDataTable, { props: { items: rows, columns, clickable: true, selectable: true, selected: ['1'] } })
    expect(w.find('table').exists()).toBe(false)
    expect(w.findAll('li')).toHaveLength(3)
    expect(w.findAll('dt').map((d) => d.text())).not.toContain('N')
    await w.find('li').trigger('click')
    expect(w.emitted('row-click')).toHaveLength(1)
    await w.find('li input[type=checkbox]').trigger('click')
    expect(w.emitted('update:selected')?.[0]).toEqual([[]])
    await expectA11y(w.element)
    w.unmount()
    setViewport(1280)
    const e = mount(UiDataTable, { props: { items: [], columns, emptyTitle: 'None', emptyText: 'nothing' } })
    expect(e.text()).toContain('None')
    const l = mount(UiDataTable, { props: { items: [], columns, loading: true } })
    expect(l.find('[aria-busy=true]').exists()).toBe(true)
    const many = Array.from({ length: 450 }, (_, i) => ({ id: String(i), name: 'n' + i, n: i }))
    const v = mount(UiDataTable, { props: { items: many, columns, virtualAt: 200 } })
    expect(v.findAll('tbody tr')).toHaveLength(200)
    await v.find('button.btn').trigger('click')
    expect(v.findAll('tbody tr')).toHaveLength(400)
    const s = mount(UiDataTable, { props: { items: rows, columns, responsive: 'scroll' } })
    setViewport(360)
    expect(s.find('table').exists()).toBe(true)
    setViewport(1280)
  })
})

describe('UiKeyValueTable / stats / pagination', () => {
  it('render values and slots', async () => {
    const w = mountBody({
      components: { UiKeyValueTable, UiStatTile, UiStatGrid, UiPagination },
      template: `<div><UiKeyValueTable :items="[{label:'A',value:'x'},{label:'B'},{label:'C',value:['p','q']},{label:'D',value:{k:1}},{label:'E',value:[]}]" :columns="2" />
        <UiStatGrid :cols="3"><UiStatTile title="Assets" :value="3" icon="mdi-laptop" color="primary" subtitle="s" /><UiStatTile title="Plain" value="x" /></UiStatGrid>
        <UiPagination :has-prev="false" :has-next="true" label="1" @next="n = 1" /><span id="n">{{ n }}</span></div>`,
      data: () => ({ n: 0 }),
    })
    expect(w.text()).toContain('x')
    expect(w.findAll('dd')[1]!.text()).toBe('—')
    expect(w.findAll('dd')[2]!.text()).toBe('p, q')
    expect(w.findAll('dd')[3]!.text()).toBe('{"k":1}')
    expect(w.findAll('dd')[4]!.text()).toBe('—')
    expect(w.find('button[aria-label="Previous page"]').attributes('disabled')).toBeDefined()
    await w.find('button[aria-label="Next page"]').trigger('click')
    expect(w.find('#n').text()).toBe('1')
    await expectA11y(w.element)
    w.unmount()
  })
})

describe('UiTree', () => {
  const items: TreeNode[] = [
    { id: 'r', label: 'Root', badge: '2', children: [{ id: 'c1', label: 'Child 1' }, { id: 'c2', label: 'Child 2', children: [{ id: 'g', label: 'Grand' }] }] },
    { id: 's', label: 'Solo' },
  ]
  it('expands/collapses, keyboard navigates, selects', async () => {
    const w = mountBody(UiTree, { props: { items } })
    const nodes = () => w.findAll('[role=treeitem]')
    expect(nodes()).toHaveLength(5)
    expect(w.find('[role=treeitem]').attributes('aria-expanded')).toBe('true')
    await w.find('button[aria-label=Collapse]').trigger('click')
    expect(nodes()).toHaveLength(2)
    expect(w.emitted('update:expanded')?.[0]?.[0]).not.toContain('r')
    const first = nodes()[0]!
    ;(first.element as HTMLElement).focus()
    await first.trigger('keydown', { key: 'ArrowRight' })
    expect(nodes()).toHaveLength(5) // expanded again (children keep their own state)
    await nodes()[0]!.trigger('keydown', { key: 'ArrowDown' })
    expect(document.activeElement?.textContent).toContain('Child 1')
    await nodes()[1]!.trigger('keydown', { key: 'ArrowLeft' })
    expect(document.activeElement?.textContent).toContain('Root')
    await nodes()[0]!.trigger('keydown', { key: 'End' })
    expect(document.activeElement?.textContent).toContain('Solo')
    await nodes()[3]!.trigger('keydown', { key: 'Home' })
    await nodes()[0]!.trigger('keydown', { key: 'ArrowUp' })
    await nodes()[0]!.trigger('keydown', { key: 'Enter' })
    expect(w.emitted('select')?.[0]?.[0]).toMatchObject({ id: 'r' })
    expect(w.emitted('update:selected')?.[0]).toEqual(['r'])
    await nodes()[0]!.trigger('keydown', { key: 'ArrowRight' }) // already open → move into child
    expect(document.activeElement?.textContent).toContain('Child 1')
    await nodes()[1]!.trigger('keydown', { key: ' ' })
    await nodes()[0]!.trigger('keydown', { key: 'ArrowLeft' }) // collapse root
    expect(nodes()).toHaveLength(2)
    await w.findAll('li button')[1]!.trigger('click') // label click selects
    await w.setProps({ selected: 'r' })
    expect(w.find('[aria-selected=true]').exists()).toBe(true)
    await expectA11y(w.element)
    w.unmount()
    const e = mount(UiTree, { props: { items: [] } })
    expect(e.text()).toContain('Nothing here yet')
    const c = mount(UiTree, { props: { items, defaultExpanded: false, expanded: [] } })
    expect(c.findAll('[role=treeitem]')).toHaveLength(2)
  })
})

describe('UiBarList', () => {
  it('renders proportional bars with labels and an empty state', () => {
    const w = mount(UiBarList, { props: { items: [{ label: 'active', value: 9, color: 'success' }, { label: 'stale', value: 3 }] } })
    const bars = w.findAll('progress')
    expect(bars).toHaveLength(2)
    expect(bars[0]!.attributes('max')).toBe('9')
    expect(bars[0]!.classes()).toContain('progress-success')
    expect(bars[1]!.classes()).toContain('progress-primary')
    expect(w.text()).toContain('stale')
    const e = mount(UiBarList, { props: { items: [], emptyTitle: 'No hosts yet' } })
    expect(e.text()).toContain('No hosts yet')
  })
})
